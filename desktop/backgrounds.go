package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type BackgroundRecord struct {
	Provider   string      `json:"provider,omitempty"`
	Image      string      `json:"image,omitempty"`
	AppID      string      `json:"appid,omitempty"`
	Name       string      `json:"name,omitempty"`
	URL        string      `json:"url,omitempty"`
	SourceURL  string      `json:"source_url,omitempty"`
	Status     string      `json:"status"`
	Message    string      `json:"message"`
	CheckedAt  string      `json:"checked_at,omitempty"`
	Width      int         `json:"width,omitempty"`
	Height     int         `json:"height,omitempty"`
	Candidates []Candidate `json:"candidates,omitempty"`
}

var versionSuffix = regexp.MustCompile(`(?i)\s+v?\d+\.\d+(?:\.\d+)*.*$`)
var explicitVersion = regexp.MustCompile(`(?i)[\s_\-（(\[【]*(?:ver(?:sion)?[.\s]*|v[.\s]*|build[.\s]*|版本[\s]*)(?:[0-9]+(?:[.][0-9a-z]+)*)(?:[）)\]】])?`)
var trailingVersion = regexp.MustCompile(`(?i)[\s_\-（(\[【]+[0-9]+[.][0-9]+(?:[.][0-9a-z]+)*(?:[）)\]】])?\s*$`)
var camelBoundary = regexp.MustCompile(`([a-z])([A-Z])`)
var vnLink = regexp.MustCompile(`^https://vndb\.org/(v[0-9]+)/?$`)

func backgroundName(name string) string {
	name = explicitVersion.ReplaceAllString(name, " ")
	name = trailingVersion.ReplaceAllString(name, "")
	name = versionSuffix.ReplaceAllString(name, "")
	name = camelBoundary.ReplaceAllString(name, "$1 $2")
	return strings.TrimSpace(strings.ReplaceAll(name, "_", " "))
}

// Only a unique exact title/alias is eligible for unattended matching.
func exactBackgroundCandidate(cs []Candidate) (Candidate, bool) {
	var result Candidate
	seen := map[string]bool{}
	for _, c := range cs {
		if c.Exact && !seen[c.Key] {
			result = c
			seen[c.Key] = true
		}
	}
	return result, len(seen) == 1
}

type steamCatalogItem struct {
	ID      int    `json:"id"`
	Type    int    `json:"type"`
	Success int    `json:"success"`
	Name    string `json:"name"`
	Assets  struct {
		Format string `json:"asset_url_format"`
		Hero   string `json:"library_hero"`
	} `json:"assets"`
}

func steamCatalog(appids []string) ([]steamCatalogItem, error) {
	var last error
	for _, region := range []string{"HK", "CN"} {
		items, e := steamCatalogRegion(appids, region)
		if e != nil {
			last = e
			continue
		}
		for _, item := range items {
			if item.Success == 1 {
				return items, nil
			}
		}
	}
	return nil, last
}
func steamCatalogRegion(appids []string, region string) ([]steamCatalogItem, error) {
	ids := []map[string]int{}
	for _, id := range appids {
		n, e := strconv.Atoi(id)
		if e != nil {
			return nil, e
		}
		ids = append(ids, map[string]int{"appid": n})
	}
	input, _ := json.Marshal(map[string]any{"ids": ids, "context": map[string]string{"language": "english", "country_code": region}, "data_request": map[string]bool{"include_assets": true}})
	var response struct {
		Response struct {
			Items []steamCatalogItem `json:"store_items"`
		} `json:"response"`
	}
	e := fetchJSON("https://api.steampowered.com/IStoreBrowseService/GetItems/v1/?input_json="+url.QueryEscape(string(input)), nil, &response)
	return response.Response.Items, e
}

func steamLinksForVN(address string) ([]Candidate, error) {
	m := vnLink.FindStringSubmatch(address)
	if len(m) != 2 {
		return nil, nil
	}
	var response struct {
		More    bool `json:"more"`
		Results []struct {
			Links []struct {
				URL string `json:"url"`
			} `json:"extlinks"`
		} `json:"results"`
	}
	e := fetchJSON("https://api.vndb.org/kana/release", map[string]any{
		"filters": []any{"vn", "=", []any{"id", "=", m[1]}}, "fields": "extlinks.url", "results": 100,
	}, &response)
	if e != nil {
		return nil, e
	}
	cs := []Candidate{}
	seen := map[string]bool{}
	for _, release := range response.Results {
		for _, link := range release.Links {
			if id := parseAppID(link.URL); id != "" && !seen[id] {
				seen[id] = true
				cs = append(cs, Candidate{Key: "steam:" + id, Name: "Steam AppID " + id, URL: link.URL, Provider: "VNDB 发行版关联", Exact: !response.More})
			}
		}
	}
	// A VNDB release can link the base game plus soundtracks, patches, or DLC.
	// Eliminate those only when Steam successfully identifies every linked item.
	if len(cs) > 1 {
		ids := []string{}
		for _, c := range cs {
			ids = append(ids, strings.TrimPrefix(c.Key, "steam:"))
		}
		if items, err := steamCatalog(ids); err == nil && len(items) == len(cs) {
			games := []Candidate{}
			allKnown := true
			for _, item := range items {
				if item.Success != 1 {
					allKnown = false
					break
				}
				if item.Type == 0 {
					id := strconv.Itoa(item.ID)
					games = append(games, Candidate{Key: "steam:" + id, Name: item.Name, URL: "https://store.steampowered.com/app/" + id + "/", Provider: "VNDB 发行版关联 · Steam 游戏", Exact: !response.More})
				}
			}
			if allKnown && len(games) > 0 {
				cs = games
			}
		}
	}

	return cs, nil
}

func (a *App) storeBackground(id string, record BackgroundRecord) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.backgrounds == nil {
		a.backgrounds = map[string]BackgroundRecord{}
	}
	old, hadOld := a.backgrounds[id]
	a.backgrounds[id] = record
	if e := writeJSON(filepath.Join(a.dir, "backgrounds.json"), a.backgrounds); e != nil {
		if hadOld {
			a.backgrounds[id] = old
		} else {
			delete(a.backgrounds, id)
		}
		return e
	}
	return nil
}

func (a *App) FindBackground(id, query string) (BackgroundRecord, error) {
	a.mu.Lock()
	g, e := a.game(id)
	sourceURL := str(a.meta[id], "source_url")
	name := str(a.meta[id], "name")
	old := a.backgrounds[id]
	local := a.heroPath(g)
	a.mu.Unlock()
	if e != nil {
		return BackgroundRecord{}, e
	}
	query = strings.TrimSpace(query)
	if len([]rune(query)) > 160 {
		return old, errors.New("关键词不能超过 160 字符")
	}
	if local != "" && query == "" && old.Provider != "SteamGridDB" {
		old.Status = "existing"
		old.Message = "已有正式宽幅背景，已跳过"
		return old, nil
	}
	record := old
	record.Candidates = nil
	record.CheckedAt = time.Now().Format(time.RFC3339)
	var cs []Candidate
	appid := parseAppID(query)
	if query == "" {
		appid = parseAppID(sourceURL)
		if appid == "" {
			appid = detectAppID(g)
		}
		// Spacewar is often a generic compatibility ID, not the actual game.
		if appid == "480" {
			appid = ""
		}
		if appid == "" && vnLink.MatchString(sourceURL) {
			cs, e = steamLinksForVN(sourceURL)
		}
	}
	if appid != "" {
		cs = []Candidate{{Key: "steam:" + appid, Name: name, URL: "https://store.steampowered.com/app/" + appid + "/", Provider: "Steam", Exact: true}}
	} else if len(cs) == 0 {
		queries := []string{query}
		if query == "" {
			queries = []string{name, backgroundName(g.Name)}
		}
		seen := map[string]bool{}
		for _, q := range queries {
			q = strings.TrimSpace(q)
			if q == "" || seen[q] {
				continue
			}
			seen[q] = true
			found, searchErr := steamLookup(q)
			if searchErr != nil {
				e = searchErr
				continue
			}
			e = nil
			cs = found
			if _, exact := exactBackgroundCandidate(cs); exact {
				break
			}
			// Preserve candidates from an earlier useful query.
			if len(cs) > 0 {
				record.Candidates = cs
			}
		}
		if len(cs) == 0 {
			cs = record.Candidates
		}
	}
	record.Candidates = cs
	if c, exact := exactBackgroundCandidate(cs); exact {
		if e := a.storeBackground(id, record); e != nil {
			return old, e
		}
		return a.UseBackground(id, strings.TrimPrefix(c.Key, "steam:"))
	}
	record.Status = "not_found"
	record.Message = "未找到能确认对应作品的正式宽幅图，保留现有封面"
	if len(cs) > 0 {
		record.Status = "review"
		record.Message = "找到相近作品，请核对后选择背景"
	}
	if e != nil && len(cs) == 0 {
		record.Status = "failed"
		record.Message = "在线查询暂不可用，可稍后重试：" + e.Error()
	}
	return record, a.storeBackground(id, record)
}

func validBackground(b []byte) (image.Config, string, error) {
	c, format, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || (format != "jpeg" && format != "png") {
		return c, "", errors.New("下载内容不是有效 JPG 或 PNG 图片")
	}
	ratio := float64(c.Width) / float64(c.Height)
	if c.Width < 1280 || c.Height < 400 || ratio < 1.6 || ratio > 4.5 {
		return c, "", errors.New("图片分辨率或比例不适合宽幅背景")
	}
	if format == "jpeg" {
		format = "jpg"
	}
	return c, format, nil
}

func fetchBackgroundImage(address string) ([]byte, error) {
	return fetchLibraryImage(address, 10<<20)
}

func fetchLibraryImage(address string, limit int64) ([]byte, error) {
	if !allowedImage(address) {
		return nil, errors.New("图片来源不受支持")
	}
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 4 || !allowedImage(req.URL.String()) {
			return errors.New("图片重定向来源不受支持")
		}
		return nil
	}}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 150 * time.Millisecond)
		}
		r, err := client.Get(address)
		retry := false
		if err != nil {
			last = err
			var networkError net.Error
			retry = errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &networkError)
		} else {
			if r.StatusCode == http.StatusOK {
				b, readErr := io.ReadAll(io.LimitReader(r.Body, limit+1))
				r.Body.Close()
				if readErr == nil {
					if int64(len(b)) > limit {
						return nil, errors.New("图片超过大小限制")
					}
					return b, nil
				}
				last = readErr
				var networkError net.Error
				retry = errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) || errors.As(readErr, &networkError)
			} else {
				r.Body.Close()
				last = fmt.Errorf("图片服务返回 HTTP %d", r.StatusCode)
				retry = r.StatusCode == 429 || r.StatusCode >= 500
			}
		}
		if !retry {
			break
		}
	}
	return nil, fmt.Errorf("图片下载失败，已保留现有资料，请稍后重试：%w", last)
}

// Store-search capsules can disappear or fail while the app-details header works.
func fetchCandidateCover(c Candidate) ([]byte, error) {
	b, err := fetchLibraryImage(c.Image, 6<<20)
	if err == nil {
		return b, nil
	}
	if strings.HasPrefix(c.Key, "steam:") {
		if cs, e := steamLookup(strings.TrimPrefix(c.Key, "steam:")); e == nil {
			for _, fallback := range cs {
				if fallback.Key == c.Key && fallback.Image != "" && fallback.Image != c.Image {
					if b, e := fetchLibraryImage(fallback.Image, 6<<20); e == nil {
						return b, nil
					}
				}
			}
		}
	}
	return nil, err
}

// Only Steam's dedicated library artwork is used; no blurred thumbnails,
// screenshots, fan art, or generated replacements are selected automatically.
func downloadSteamBackground(appid string) ([]byte, string, image.Config, string, error) {
	address := "https://cdn.akamai.steamstatic.com/steam/apps/" + appid + "/library_hero.jpg"
	b, e := fetchBackgroundImage(address)
	if e == nil {
		if c, ext, err := validBackground(b); err == nil {
			return b, address, c, ext, nil
		}
	}
	// Ask Steam for each asset's actual hashed path; header and hero hashes differ.
	if items, err := steamCatalog([]string{appid}); err == nil {
		for _, item := range items {
			if strconv.Itoa(item.ID) != appid || item.Success != 1 || item.Assets.Hero == "" {
				continue
			}
			format := item.Assets.Format
			if !strings.HasPrefix(format, "steam/apps/"+appid+"/") || !strings.Contains(format, "${FILENAME}") || strings.Contains(item.Assets.Hero, "..") {
				continue
			}
			address = "https://shared.akamai.steamstatic.com/store_item_assets/" + strings.ReplaceAll(format, "${FILENAME}", item.Assets.Hero)
			b, e = fetchBackgroundImage(address)
			if e == nil {
				if c, ext, err := validBackground(b); err == nil {
					return b, address, c, ext, nil
				}
			}
		}
	}
	return nil, "", image.Config{}, "", errors.New("此作品暂未提供可下载的正式宽幅原画，或图片服务暂不可用；保留现有图片")
}

func (a *App) UseBackground(id, appid string) (BackgroundRecord, error) {
	if !digits.MatchString(appid) {
		return BackgroundRecord{}, errors.New("无效 Steam AppID")
	}
	a.mu.Lock()
	_, e := a.game(id)
	record := a.backgrounds[id]
	a.mu.Unlock()
	if e != nil {
		return record, e
	}
	var selected *Candidate
	for i := range record.Candidates {
		if record.Candidates[i].Key == "steam:"+appid {
			selected = &record.Candidates[i]
			break
		}
	}
	if selected == nil {
		return record, errors.New("请先查找背景并核对候选作品")
	}
	b, address, config, ext, downloadErr := downloadSteamBackground(appid)
	record.CheckedAt = time.Now().Format(time.RFC3339)
	if downloadErr != nil {
		record.Status = "failed"
		record.Message = downloadErr.Error()
		return record, a.storeBackground(id, record)
	}
	sum := sha256.Sum256(b)
	rel := "backgrounds/" + id + "-" + hex.EncodeToString(sum[:8]) + "." + ext
	if e := os.MkdirAll(filepath.Join(a.dir, "backgrounds"), 0755); e != nil {
		return record, e
	}
	if e := os.WriteFile(filepath.Join(a.dir, filepath.FromSlash(rel)), b, 0600); e != nil {
		return record, e
	}
	record.Image = rel
	record.URL = address
	record.AppID = appid
	record.SourceURL = selected.URL
	record.Name = selected.Name
	record.Width = config.Width
	record.Height = config.Height
	record.Status = "found"
	record.Provider = "Steam"
	record.Message = "已采用 Steam 正式宽幅原画"
	record.Candidates = nil
	return record, a.storeBackground(id, record)
}
