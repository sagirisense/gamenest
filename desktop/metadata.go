package main

import (
	"bytes"
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
	"strconv"
	"strings"
	"time"
)

type Candidate struct {
	Score    float64  `json:"score,omitempty"`
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Image    string   `json:"image"`
	URL      string   `json:"url"`
	Provider string   `json:"provider"`
	Exact    bool     `json:"exact"`
	Aliases  []string `json:"aliases"`
	Preview  string   `json:"preview,omitempty"`
	Author   string   `json:"author,omitempty"`
	Width    int      `json:"width,omitempty"`
	Height   int      `json:"height,omitempty"`
}

var digits = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)
var rjPattern = regexp.MustCompile(`(?i)RJ[0-9]{6,10}`)

func parseAppID(s string) string {
	s = strings.TrimSpace(s)
	if digits.MatchString(s) {
		return s
	}
	for _, prefix := range []string{"appid=", "appid:", "steam_appid=", "steam_appid:"} {
		if strings.HasPrefix(strings.ToLower(s), prefix) {
			v := strings.TrimSpace(s[len(prefix):])
			if digits.MatchString(v) {
				return v
			}
		}
	}
	u, e := url.Parse(s)
	if e == nil && u.Scheme == "https" && u.Hostname() == "store.steampowered.com" {
		p := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(p) > 1 && p[0] == "app" && digits.MatchString(p[1]) {
			return p[1]
		}
	}
	return ""
}
func detectAppID(g Game) string {
	if g.AppID != nil {
		v := fmt.Sprint(g.AppID)
		if n, ok := g.AppID.(float64); ok {
			v = strconv.FormatFloat(n, 'f', -1, 64)
		}
		if id := parseAppID(v); id != "" {
			return id
		}
	}
	p := filepath.Join(g.Folder, "steam_appid.txt")
	if st, e := os.Stat(p); e == nil && st.Size() < 128 {
		b, e := os.ReadFile(p)
		if e == nil {
			return parseAppID(strings.TrimPrefix(string(b), "\ufeff"))
		}
	}
	return ""
}
func fetchJSON(address string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return e
		}
		reader = bytes.NewReader(b)
	}
	method := "GET"
	if body != nil {
		method = "POST"
	}
	req, e := http.NewRequest(method, address, reader)
	if e != nil {
		return e
	}
	req.Header.Set("User-Agent", "GameNest/0.1")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	r, e := client.Do(req)
	if e != nil {
		return fmt.Errorf("连接失败或超时，请稍后重试：%w", e)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("远端服务返回 HTTP %d", r.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(r.Body, 6<<20)).Decode(target)
}
func norm(s string) string {
	return strings.ToLower(regexp.MustCompile(`[\s\p{P}\p{S}_]+`).ReplaceAllString(s, ""))
}
func steamLookup(query string) ([]Candidate, error) {
	var last error
	success := false
	for _, region := range []string{"HK", "CN"} {
		cs, e := steamLookupRegion(query, region)
		if e != nil {
			last = e
			continue
		}
		success = true
		if len(cs) > 0 {
			return cs, nil
		}
	}
	if success {
		return []Candidate{}, nil
	}
	return nil, last
}
func steamLookupRegion(query, region string) ([]Candidate, error) {
	regionName := "港区"
	if region == "CN" {
		regionName = "国区"
	}
	provider := "Steam · " + regionName
	if id := parseAppID(query); id != "" {
		var response map[string]struct {
			Success bool `json:"success"`
			Data    struct {
				Name  string `json:"name"`
				ID    int    `json:"steam_appid"`
				Image string `json:"header_image"`
			} `json:"data"`
		}
		e := fetchJSON("https://store.steampowered.com/api/appdetails?"+url.Values{"appids": {id}, "l": {"schinese"}, "cc": {region}, "filters": {"basic"}}.Encode(), nil, &response)
		if e != nil {
			return nil, e
		}
		r := response[id]
		if !r.Success || strconv.Itoa(r.Data.ID) != id {
			return []Candidate{}, nil
		}
		return []Candidate{{Key: "steam:" + id, Name: r.Data.Name, Image: r.Data.Image, URL: "https://store.steampowered.com/app/" + id + "/?cc=" + strings.ToLower(region), Provider: provider, Exact: true}}, nil
	}
	found := map[int]*Candidate{}
	order := []int{}
	success := false
	var last error
	for _, language := range []string{"schinese", "english"} {
		var response struct {
			Items []struct {
				ID    int    `json:"id"`
				Name  string `json:"name"`
				Image string `json:"tiny_image"`
			} `json:"items"`
		}
		e := fetchJSON("https://store.steampowered.com/api/storesearch/?"+url.Values{"term": {query}, "l": {language}, "cc": {region}}.Encode(), nil, &response)
		if e != nil {
			last = e
			continue
		}
		success = true
		for _, item := range response.Items {
			if c := found[item.ID]; c != nil {
				c.Aliases = append(c.Aliases, item.Name)
				c.Exact = c.Exact || norm(item.Name) == norm(query)
			} else {
				found[item.ID] = &Candidate{Key: fmt.Sprintf("steam:%d", item.ID), Name: item.Name, Image: item.Image, URL: fmt.Sprintf("https://store.steampowered.com/app/%d/?cc=%s", item.ID, strings.ToLower(region)), Provider: provider, Exact: norm(item.Name) == norm(query), Aliases: []string{item.Name}}
				order = append(order, item.ID)
			}
		}
	}
	if !success {
		return nil, last
	}
	out := []Candidate{}
	for _, id := range order {
		out = append(out, *found[id])
	}
	return out, nil
}

type vnItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Image *struct {
		URL string `json:"url"`
	} `json:"image"`
	Titles []struct {
		Title string `json:"title"`
		Lang  string `json:"lang"`
	} `json:"titles"`
}

func vnCandidate(x vnItem, query string, exact bool) Candidate {
	name := x.Title
	aliases := []string{x.Title}
	for _, t := range x.Titles {
		aliases = append(aliases, t.Title)
		if t.Lang == "zh-Hans" || t.Lang == "zh" {
			name = t.Title
		}
	}
	image := ""
	if x.Image != nil {
		image = x.Image.URL
	}
	c := Candidate{Key: "vndb:" + x.ID, Name: name, Image: image, URL: "https://vndb.org/" + x.ID, Provider: "VNDB", Aliases: aliases, Exact: exact}
	for _, s := range aliases {
		c.Exact = c.Exact || norm(s) == norm(query)
	}
	return c
}
func vndbLookup(query string) ([]Candidate, error) {
	var d struct {
		Results []vnItem `json:"results"`
	}
	e := fetchJSON("https://api.vndb.org/kana/vn", map[string]any{"filters": []any{"search", "=", query}, "fields": "title,titles.title,titles.lang,image.url", "results": 8}, &d)
	if e != nil {
		return nil, e
	}
	out := []Candidate{}
	for _, v := range d.Results {
		out = append(out, vnCandidate(v, query, false))
	}
	return out, nil
}
func dlsiteLookup(query string) ([]Candidate, error) {
	rj := strings.ToUpper(rjPattern.FindString(query))
	if rj == "" {
		return dlsiteNameLookup(query)
	}
	link := "https://www.dlsite.com/maniax/work/=/product_id/" + rj + ".html"
	var response map[string]struct {
		Name  string `json:"work_name"`
		Image string `json:"work_image"`
	}
	e := fetchJSON("https://www.dlsite.com/maniax/product/info/ajax?product_id="+rj, nil, &response)
	if e == nil {
		if p, ok := response[rj]; ok && p.Name != "" {
			image := p.Image
			if strings.HasPrefix(image, "//") {
				image = "https:" + image
			}
			return []Candidate{{Key: "dlsite:" + rj, Name: p.Name, Image: image, URL: link, Provider: "DLsite", Exact: true}}, nil
		}
	}
	var fallback struct {
		Results []struct {
			VNs []vnItem `json:"vns"`
		} `json:"results"`
	}
	fe := fetchJSON("https://api.vndb.org/kana/release", map[string]any{"filters": []any{"extlink", "=", link}, "fields": "title,vns.title,vns.titles.title,vns.titles.lang,vns.image.url", "results": 8}, &fallback)
	if fe != nil {
		return nil, errors.New("DLsite 暂不可用，VNDB 备用查询也未能连接。请稍后重试")
	}
	out := []Candidate{}
	seen := map[string]bool{}
	for _, release := range fallback.Results {
		for _, v := range release.VNs {
			if !seen[v.ID] {
				c := vnCandidate(v, rj, true)
				c.Provider = "VNDB · RJ编号匹配"
				out = append(out, c)
				seen[v.ID] = true
			}
		}
	}
	if len(out) == 0 && e != nil {
		return nil, fmt.Errorf("DLsite 暂不可用或正在维护；VNDB 未收录此 RJ 编号。请尝试游戏名称查询（%v）", e)
	}
	return out, nil
}
func (a *App) SearchMetadata(id, query, provider string) ([]Candidate, error) {
	query = strings.TrimSpace(query)
	if query == "" || len([]rune(query)) > 160 {
		return nil, errors.New("请输入 1–160 字符的名称或编号")
	}
	a.mu.Lock()
	_, e := a.game(id)
	a.mu.Unlock()
	if e != nil {
		return nil, e
	}
	var cs []Candidate
	switch provider {
	case "all":
		a.mu.Lock()
		g, _ := a.game(id)
		source := str(a.meta[id], "source_url")
		a.mu.Unlock()
		all, _ := collectEnrichmentCandidates(g, source, backgroundName(query))
		cs, _, _ = rankEnrichmentCandidates(all, backgroundName(query), g, source)
	case "steam":
		cs, e = steamLookup(query)
	case "vndb":
		cs, e = vndbLookup(query)
	case "dlsite":
		cs, e = dlsiteLookup(query)
	default:
		return nil, errors.New("请选择有效查询来源")
	}
	if e != nil {
		return nil, e
	}
	return a.storeMetadataCandidates(id, query, provider, cs)
}

func (a *App) storeMetadataCandidates(id, query, provider string, cs []Candidate) ([]Candidate, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := a.meta[id]
	next := map[string]any{}
	for k, v := range old {
		next[k] = v
	}
	a.meta[id] = next
	a.meta[id]["candidates"] = cs
	a.meta[id]["query"] = query
	a.meta[id]["search_provider"] = provider
	if provider == "all" {
		reason := candidateReviewReason(cs)
		a.meta[id]["review_pending"] = reason != ""
		a.meta[id]["review_reason"] = reason
	} else {
		a.meta[id]["review_pending"] = len(cs) > 0
		a.meta[id]["review_reason"] = "请核对查询结果"
	}
	a.meta[id]["error"] = ""
	if e := writeJSON(filepath.Join(a.dir, "metadata.json"), a.meta); e != nil {
		a.meta[id] = old
		return nil, e
	}
	return cs, nil
}
func allowedImage(address string) bool {
	u, e := url.Parse(address)
	if e != nil || u.Scheme != "https" {
		return false
	}
	h := u.Hostname()
	for _, host := range []string{"vndb.org", "steamstatic.com", "dlsite.com", "dlsite.jp", "steamgriddb.com"} {
		if h == host || strings.HasSuffix(h, "."+host) {
			return true
		}
	}
	return false
}
func (a *App) saveImage(id string, b []byte) (string, error) {
	if len(b) > 6<<20 {
		return "", errors.New("封面不能超过 6 MB")
	}
	kind := http.DetectContentType(b)
	ext := ""
	switch kind {
	case "image/png":
		ext = ".png"
	case "image/jpeg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	default:
		return "", errors.New("仅支持有效 PNG、JPG、WebP 图片")
	}
	sum := sha256.Sum256(b)
	rel := "covers/" + id + "-" + hex.EncodeToString(sum[:6]) + ext
	path := filepath.Join(a.dir, filepath.FromSlash(rel))
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return "", e
	}
	return rel, os.WriteFile(path, b, 0600)
}
func (a *App) UseCandidate(id, key string) error {
	a.mu.Lock()
	if _, e := a.game(id); e != nil {
		a.mu.Unlock()
		return e
	}
	b, _ := json.Marshal(a.meta[id]["candidates"])
	a.mu.Unlock()
	var cs []Candidate
	if e := json.Unmarshal(b, &cs); e != nil {
		return e
	}
	var selected *Candidate
	for i := range cs {
		if cs[i].Key == key {
			selected = &cs[i]
			break
		}
	}
	if selected == nil {
		return errors.New("候选已过期，请重新查询")
	}
	image := ""
	if selected.Image != "" {
		raw, e := fetchCandidateCover(*selected)
		if e != nil {
			return e
		}
		image, e = a.saveImage(id, raw)
		if e != nil {
			return e
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.meta[id] == nil {
		a.meta[id] = map[string]any{}
	}
	old := a.meta[id]
	m := map[string]any{}
	for k, v := range old {
		m[k] = v
	}
	m["name"] = selected.Name
	m["provider"] = selected.Provider
	m["source_url"] = selected.URL
	m["manual"] = true
	m["review_pending"] = false
	m["review_resolved"] = candidateSignature(cs)
	m["review_decision"] = "selected"
	m["error"] = ""
	if image != "" {
		m["image"] = image
		m["cover_provider"] = selected.Provider
		m["cover_source_url"] = selected.URL
		m["cover_author"] = ""
	}
	a.meta[id] = m
	if e := writeJSON(filepath.Join(a.dir, "metadata.json"), a.meta); e != nil {
		a.meta[id] = old
		return e
	}
	return nil
}
