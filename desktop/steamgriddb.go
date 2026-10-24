package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type IntegrationConfig struct {
	SteamGridDBKey string `json:"steamgriddb_key_protected,omitempty"`
}
type ArtworkSettings struct {
	Configured bool   `json:"configured"`
	Message    string `json:"message"`
}
type ArtworkGame struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var artworkKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,256}$`)

func (a *App) steamGridDBKey() (string, error) {
	a.mu.Lock()
	sealed := a.integrations.SteamGridDBKey
	loadErr := a.integrationError
	a.mu.Unlock()
	if loadErr != nil {
		return "", errors.New("SteamGridDB 配置无法读取，请在设置中重新保存密钥")
	}
	if sealed == "" {
		return "", errors.New("请先在设置中填写 SteamGridDB API Key")
	}
	return unsealAPIKey(sealed)
}
func (a *App) GetArtworkSettings() ArtworkSettings {
	if _, e := a.steamGridDBKey(); e != nil {
		return ArtworkSettings{Message: e.Error()}
	}
	return ArtworkSettings{Configured: true, Message: "已保存 API Key"}
}
func (a *App) SaveSteamGridDBKey(value string) (ArtworkSettings, error) {
	value = strings.TrimSpace(value)
	if !artworkKeyPattern.MatchString(value) {
		return ArtworkSettings{}, errors.New("请填写有效的 API Key 本身，不包含 Bearer 前缀或空格")
	}
	sealed, e := sealAPIKey(value)
	if e != nil {
		return ArtworkSettings{}, e
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	c := IntegrationConfig{SteamGridDBKey: sealed}
	if e := writeJSON(filepath.Join(a.dir, "integrations.json"), c); e != nil {
		return ArtworkSettings{}, e
	}
	a.integrations = c
	a.integrationError = nil
	return ArtworkSettings{Configured: true, Message: "密钥已保存在本机"}, nil
}
func (a *App) ClearSteamGridDBKey() (ArtworkSettings, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if e := writeJSON(filepath.Join(a.dir, "integrations.json"), IntegrationConfig{}); e != nil {
		return ArtworkSettings{}, e
	}
	a.integrations = IntegrationConfig{}
	a.integrationError = nil
	a.artworkGames = nil
	a.artworkCandidates = nil
	return ArtworkSettings{Message: "密钥已清除"}, nil
}

// Credentials are sent only to the API origin, never to image CDNs or redirects.
func sgdbRequest(key, path string, target any) error {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return errors.New("无效图片查询")
	}
	req, e := http.NewRequest("GET", "https://www.steamgriddb.com/api/v2"+path, nil)
	if e != nil {
		return errors.New("无法创建图片查询")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "GameNest/0.1")
	client := &http.Client{Timeout: 18 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return errors.New("SteamGridDB API 返回了重定向，请稍后重试")
	}}
	r, e := client.Do(req)
	if e != nil {
		return errors.New("无法连接 SteamGridDB，请检查网络后重试")
	}
	defer r.Body.Close()
	switch r.StatusCode {
	case 200:
	case 401, 403:
		return errors.New("SteamGridDB 密钥无效或无访问权限，请在设置中检查")
	case 404:
		return errors.New("SteamGridDB 未收录此游戏或图片")
	case 429:
		return errors.New("SteamGridDB 查询过于频繁，请稍后重试")
	default:
		return fmt.Errorf("SteamGridDB 暂不可用（HTTP %d）", r.StatusCode)
	}
	var response struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if e := json.NewDecoder(io.LimitReader(r.Body, 6<<20)).Decode(&response); e != nil || !response.Success {
		return errors.New("SteamGridDB 未返回有效结果")
	}
	if target == nil {
		return nil
	}
	if e := json.Unmarshal(response.Data, target); e != nil {
		return errors.New("SteamGridDB 结果格式无法读取")
	}
	return nil
}
func (a *App) TestSteamGridDBKey(value string) (string, error) {
	key := strings.TrimSpace(value)
	if key == "" {
		var e error
		key, e = a.steamGridDBKey()
		if e != nil {
			return "", e
		}
	} else if !artworkKeyPattern.MatchString(key) {
		return "", errors.New("API Key 格式不正确")
	}
	if e := sgdbRequest(key, "/search/autocomplete/Portal", nil); e != nil {
		return "", e
	}
	return "连接成功，SteamGridDB API 可用", nil
}

func (a *App) SearchArtworkGames(id, query string) ([]ArtworkGame, error) {
	a.mu.Lock()
	_, e := a.game(id)
	a.mu.Unlock()
	if e != nil {
		return nil, e
	}
	query = strings.TrimSpace(query)
	if query == "" || len([]rune(query)) > 160 {
		return nil, errors.New("请输入游戏名称或 Steam AppID")
	}
	key, e := a.steamGridDBKey()
	if e != nil {
		return nil, e
	}
	games := []ArtworkGame{}
	if appid := parseAppID(query); appid != "" {
		var game ArtworkGame
		e = sgdbRequest(key, "/games/steam/"+appid, &game)
		if e == nil && game.ID > 0 {
			games = append(games, game)
		}
	} else {
		e = sgdbRequest(key, "/search/autocomplete/"+url.PathEscape(query), &games)
	}
	if e != nil {
		return nil, e
	}
	if len(games) > 12 {
		games = games[:12]
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.artworkGames == nil {
		a.artworkGames = map[string][]ArtworkGame{}
	}
	a.artworkGames[id] = games
	return games, nil
}

func (a *App) GetSteamGridDBArtwork(id, gameID, kind string) ([]Candidate, error) {
	if kind != "cover" && kind != "background" {
		return nil, errors.New("请选择封面或背景")
	}
	if !digits.MatchString(gameID) {
		return nil, errors.New("无效的作品编号")
	}
	a.mu.Lock()
	_, e := a.game(id)
	name := ""
	for _, g := range a.artworkGames[id] {
		if strconv.Itoa(g.ID) == gameID {
			name = g.Name
			break
		}
	}
	a.mu.Unlock()
	if e != nil {
		return nil, e
	}
	if name == "" {
		return nil, errors.New("请先搜索并选择对应作品")
	}
	key, e := a.steamGridDBKey()
	if e != nil {
		return nil, e
	}
	resource, page := "grids", "grid"
	if kind == "background" {
		resource, page = "heroes", "hero"
	}
	var assets []struct {
		ID     int    `json:"id"`
		URL    string `json:"url"`
		Thumb  string `json:"thumb"`
		Style  any    `json:"style"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Score  int    `json:"score"`
		Humor  bool   `json:"humor"`
		Author struct {
			Name string `json:"name"`
		} `json:"author"`
	}
	params := url.Values{"types": {"static"}, "mimes": {"image/png,image/jpeg"}, "humor": {"false"}, "nsfw": {"any"}}
	if e := sgdbRequest(key, "/"+resource+"/game/"+gameID+"?"+params.Encode(), &assets); e != nil {
		return nil, e
	}
	// Favor portrait covers and large artwork. A community upload is never
	// represented as official just because its game entry is verified.
	sort.SliceStable(assets, func(i, j int) bool {
		if kind == "cover" {
			ip, jp := assets[i].Height > assets[i].Width, assets[j].Height > assets[j].Width
			if ip != jp {
				return ip
			}
		}
		ai, aj := assets[i].Width*assets[i].Height, assets[j].Width*assets[j].Height
		if ai != aj {
			return ai > aj
		}
		return assets[i].Score > assets[j].Score
	})
	cs := []Candidate{}
	cache := map[string]Candidate{}
	for _, asset := range assets {
		if asset.ID <= 0 || asset.Humor || strings.Contains(strings.ToLower(fmt.Sprint(asset.Style)), "blur") || !sgdbImageURL(asset.URL) {
			continue
		}
		if kind == "background" && (asset.Width < 1280 || asset.Height < 400 || (float64(asset.Width)/float64(asset.Height) < 1.6 || float64(asset.Width)/float64(asset.Height) > 4.5)) {
			continue
		}
		if asset.Width <= 0 || asset.Height <= 0 {
			continue
		}
		preview := asset.Thumb
		if !sgdbImageURL(preview) {
			preview = asset.URL
		}
		c := Candidate{Key: fmt.Sprintf("sgdb:%s:%d", page, asset.ID), Name: name, Image: asset.URL, Preview: preview, URL: fmt.Sprintf("https://www.steamgriddb.com/%s/%d", page, asset.ID), Provider: "SteamGridDB · 社区图片", Width: asset.Width, Height: asset.Height, Author: asset.Author.Name}
		cs = append(cs, c)
		cache[c.Key] = c
		if len(cs) >= 24 {
			break
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.artworkCandidates == nil {
		a.artworkCandidates = map[string]map[string]Candidate{}
	}
	a.artworkCandidates[id+":"+kind] = cache
	return cs, nil
}
func sgdbImageURL(address string) bool {
	u, e := url.Parse(address)
	if e != nil || u.Scheme != "https" {
		return false
	}
	h := u.Hostname()
	return h == "steamgriddb.com" || strings.HasSuffix(h, ".steamgriddb.com")
}

func (a *App) UseArtwork(id, kind, key string) error {
	if kind != "cover" && kind != "background" {
		return errors.New("请选择封面或背景")
	}
	a.mu.Lock()
	_, e := a.game(id)
	c, ok := a.artworkCandidates[id+":"+kind][key]
	a.mu.Unlock()
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("候选已过期，请重新查找图片")
	}
	b, e := fetchBackgroundImage(c.Image)
	if e != nil {
		return e
	}
	if kind == "cover" {
		a.mu.Lock()
		defer a.mu.Unlock()
		image, e := a.saveImage(id, b)
		if e != nil {
			return e
		}
		m := map[string]any{}
		for k, v := range a.meta[id] {
			m[k] = v
		}
		m["image"] = image
		m["cover_provider"] = "SteamGridDB"
		m["cover_source_url"] = c.URL
		m["cover_author"] = c.Author
		m["manual"] = true
		old := a.meta[id]
		a.meta[id] = m
		if e := writeJSON(filepath.Join(a.dir, "metadata.json"), a.meta); e != nil {
			a.meta[id] = old
			return e
		}
		return nil
	}
	config, ext, e := validBackground(b)
	if e != nil {
		return e
	}
	sum := sha256.Sum256(b)
	rel := "backgrounds/" + id + "-" + hex.EncodeToString(sum[:8]) + "." + ext
	if e := os.MkdirAll(filepath.Join(a.dir, "backgrounds"), 0755); e != nil {
		return e
	}
	if e := os.WriteFile(filepath.Join(a.dir, filepath.FromSlash(rel)), b, 0600); e != nil {
		return e
	}
	return a.storeBackground(id, BackgroundRecord{Image: rel, Name: c.Name, Provider: "SteamGridDB", URL: c.Image, SourceURL: c.URL, Width: config.Width, Height: config.Height, CheckedAt: time.Now().Format(time.RFC3339), Status: "found", Message: "已采用 SteamGridDB 图片（社区来源）"})
}
