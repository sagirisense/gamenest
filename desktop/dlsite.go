package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func dlsiteSearchURL(query string) string {
	// DLsite uses form encoding even inside its keyword path: spaces must be '+'.
	return "https://www.dlsite.com/maniax/fsr/=/keyword/" + url.QueryEscape(query) + "/order/trend_d/per_page/30"
}

func dlsiteNameLookup(query string) ([]Candidate, error) {
	cs, searchErr := searchEngineDLsite(query)
	if len(cs) > 0 {
		return cs, nil
	}
	backup, backupErr := vndbLookup(query)
	if backupErr == nil {
		if resolved := dlsiteFromVNDB(backup); len(resolved) > 0 {
			return resolved, nil
		}
	}
	cs, directErr := dlsiteNameSearch(query)
	if directErr == nil && len(cs) > 0 {
		return cs, nil
	}
	if backupErr == nil && len(backup) > 0 {
		for i := range backup {
			backup[i].Provider = "VNDB · DLsite名称备用"
		}
		return backup, nil
	}
	if directErr != nil || searchErr != nil {
		return nil, fmt.Errorf("搜索引擎及 DLsite 未取得可用结果（搜索：%v；DLsite：%v），VNDB 备用也未找到候选。可尝试日文原名或 RJ 编号", searchErr, directErr)
	}
	return cs, nil
}

// Resolve official product links through VNDB as a final fallback.
// Keep the original title confidence.
func dlsiteFromVNDB(vns []Candidate) []Candidate {
	out := []Candidate{}
	seen := map[string]bool{}
	for i, vn := range vns {
		if i >= 4 {
			break
		}
		match := vnLink.FindStringSubmatch(vn.URL)
		if len(match) != 2 {
			continue
		}
		var response struct {
			More    bool `json:"more"`
			Results []struct {
				Links []struct {
					URL string `json:"url"`
				} `json:"extlinks"`
			} `json:"results"`
		}
		if fetchJSON("https://api.vndb.org/kana/release", map[string]any{
			"filters": []any{"and", []any{"vn", "=", []any{"id", "=", match[1]}}, []any{"extlink", "=", "dlsite"}},
			"fields":  "extlinks.url", "results": 100,
		}, &response) != nil {
			continue
		}
		for _, release := range response.Results {
			for _, link := range release.Links {
				u, err := url.Parse(link.URL)
				if err != nil || u.Scheme != "https" || (u.Hostname() != "www.dlsite.com" && u.Hostname() != "dlsite.com") {
					continue
				}
				rj := strings.ToUpper(rjPattern.FindString(u.Path))
				if rj == "" || seen[rj] || len(seen) >= 12 {
					continue
				}
				seen[rj] = true
				cs, err := dlsiteLookup(rj)
				if err != nil {
					continue
				}
				for _, c := range cs {
					if c.Key != "dlsite:"+rj {
						continue
					}
					c.Exact = vn.Exact && !response.More
					c.Provider = "DLsite · VNDB关联RJ编号"
					c.Aliases = vn.Aliases
					out = append(out, c)
				}
			}
		}
	}
	return out
}

func dlsiteNameSearch(query string) ([]Candidate, error) {
	req, e := http.NewRequest("GET", dlsiteSearchURL(query), nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "GameNest/0.1")
	req.Header.Set("Accept-Language", "ja,en;q=0.8")
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		h := req.URL.Hostname()
		if len(via) > 4 || req.URL.Scheme != "https" || (h != "www.dlsite.com" && h != "dlsite.com") {
			return errors.New("DLsite 返回了非搜索页面")
		}
		return nil
	}}
	r, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", r.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, (4<<20)+1))
	if e != nil {
		return nil, e
	}
	if len(b) > 4<<20 {
		return nil, errors.New("DLsite 搜索页面过大")
	}
	return parseDLsiteResults(strings.NewReader(string(b)), query)
}

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func nodeClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(nodeAttr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}
func nodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(nodeText(c))
	}
	return b.String()
}

var dlsiteMainImage = regexp.MustCompile(`(?:https:)?//img\.dlsite\.jp/[^\s'"<>]+_img_main\.(?:jpg|png|webp)`)

func parseDLsiteResults(reader io.Reader, query string) ([]Candidate, error) {
	doc, e := html.Parse(reader)
	if e != nil {
		return nil, e
	}
	cs := []Candidate{}
	seen := map[string]bool{}
	recognized := false
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && nodeAttr(n, "id") == "search_result_list" {
			recognized = true
		}
		id := strings.ToUpper(nodeAttr(n, "data-list_item_product_id"))
		if n.Type == html.ElementNode && n.Data == "li" && rjPattern.FindString(id) == id && id != "" && !seen[id] && len(cs) < 12 {
			name, link, picture := "", "", ""
			var inspect func(*html.Node, bool)
			inspect = func(x *html.Node, inTitle bool) {
				inTitle = inTitle || nodeClass(x, "work_name")
				if inTitle && x.Type == html.ElementNode && x.Data == "a" && name == "" {
					u, err := url.Parse(nodeAttr(x, "href"))
					if err == nil && u.Scheme == "https" && (u.Hostname() == "www.dlsite.com" || u.Hostname() == "dlsite.com") && strings.Contains(u.Path, "/work/=/product_id/"+id+".html") {
						name = nodeAttr(x, "title")
						if name == "" {
							name = nodeText(x)
						}
						name = strings.Join(strings.Fields(name), " ")
						u.RawQuery = ""
						u.Fragment = ""
						link = u.String()
					}
				}
				for _, a := range x.Attr {
					if image := dlsiteMainImage.FindString(a.Val); image != "" && strings.Contains(image, "/"+id+"_img_main.") {
						if strings.HasPrefix(image, "//") {
							image = "https:" + image
						}
						picture = image
					}
				}
				for c := x.FirstChild; c != nil; c = c.NextSibling {
					inspect(c, inTitle)
				}
			}
			inspect(n, false)
			if name != "" && link != "" {
				seen[id] = true
				cs = append(cs, Candidate{Key: "dlsite:" + id, Name: name, Image: picture, URL: link, Provider: "DLsite · 名称检索", Exact: norm(name) == norm(query), Aliases: []string{name}})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
	if !recognized {
		return nil, errors.New("未取得 DLsite 搜索结果页面")
	}
	return cs, nil
}
