package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"

	"golang.org/x/net/html"
)

var rjProductPath = regexp.MustCompile(`(?i)^/(?:maniax|home|girls|pro)/work/=/product_id/(RJ[0-9]{6,10})\.html/?$`)

func officialRJLink(address string) string {
	u, err := url.Parse(strings.TrimSpace(address))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || (u.Hostname() != "www.dlsite.com" && u.Hostname() != "dlsite.com") {
		return ""
	}
	m := rjProductPath.FindStringSubmatch(u.Path)
	if len(m) != 2 {
		return ""
	}
	return strings.ToUpper(m[1])
}

// Read small nearby documents only; never follow symlinks or scan game assets.
func localRJIDs(g Game, source string) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(text string) {
		isAlphaNum := func(b byte) bool { return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
		for _, m := range rjPattern.FindAllStringIndex(text, -1) {
			if m[0] > 0 && isAlphaNum(text[m[0]-1]) || m[1] < len(text) && isAlphaNum(text[m[1]]) {
				continue
			}
			id := strings.ToUpper(text[m[0]:m[1]])
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	add(g.Name)
	add(g.OriginalName)
	add(g.Folder)
	add(g.Exe)
	for _, path := range g.Executables {
		add(path)
	}
	add(source)
	entries, bytesRead := 0, 0
	_ = filepath.WalkDir(g.Folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == g.Folder {
			return nil
		}
		entries++
		if entries > 256 || bytesRead >= 2<<20 {
			return fs.SkipAll
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		add(d.Name())
		if d.IsDir() {
			if filepath.Dir(path) != g.Folder || strings.HasSuffix(strings.ToLower(d.Name()), "_data") {
				return filepath.SkipDir
			}
			switch strings.ToLower(d.Name()) {
			case "www", "data", "node_modules", "renpy", "game", "images", "audio":
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".txt", ".url", ".webloc", ".htm", ".html", ".ini":
		default:
			return nil
		}
		st, e := d.Info()
		if e != nil || st.Size() > 256<<10 || !st.Mode().IsRegular() {
			return nil
		}
		f, e := os.Open(path)
		if e != nil {
			return nil
		}
		b, e := io.ReadAll(io.LimitReader(f, 256<<10))
		f.Close()
		if e != nil {
			return nil
		}
		bytesRead += len(b)
		if len(b) >= 2 && ((b[0] == 255 && b[1] == 254) || (b[0] == 254 && b[1] == 255)) {
			words := []uint16{}
			for i := 2; i+1 < len(b); i += 2 {
				v := uint16(b[i]) | uint16(b[i+1])<<8
				if b[0] == 254 {
					v = uint16(b[i])<<8 | uint16(b[i+1])
				}
				words = append(words, v)
			}
			add(string(utf16.Decode(words)))
		} else {
			add(string(b))
		}
		return nil
	})
	return out
}

func localDLSiteLookup(g Game, source string) ([]Candidate, error) {
	ids := localRJIDs(g, source)
	cs := []Candidate{}
	var last error
	for i, id := range ids {
		if i >= 8 {
			break
		}
		found, err := dlsiteLookup(id)
		if err != nil {
			last = err
			continue
		}
		for _, c := range found {
			c.Exact = c.Exact && len(ids) == 1
			c.Provider += " · 本地RJ编号"
			cs = append(cs, c)
		}
	}
	return cs, last
}

func searchPage(address string) ([]byte, error) {
	req, err := http.NewRequest("GET", address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; GameNest/0.1)")
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		host := req.URL.Hostname()
		if len(via) > 3 || req.URL.Scheme != "https" || (host != "www.bing.com" && host != "www.google.com" && host != "html.duckduckgo.com" && host != "duckduckgo.com" && host != "search.yahoo.co.jp") {
			return errors.New("搜索服务重定向不可用")
		}
		return nil
	}}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("搜索服务返回 HTTP %d", response.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if len(b) > 2<<20 {
		return nil, errors.New("搜索结果过大")
	}
	return b, err
}

func parseSearchRJ(b []byte, rss bool) ([]string, error) {
	links := []string{}
	if rss {
		var feed struct {
			Channel struct {
				Items []struct {
					Link string `xml:"link"`
				} `xml:"item"`
			} `xml:"channel"`
		}
		if err := xml.Unmarshal(b, &feed); err != nil {
			return nil, err
		}
		for _, item := range feed.Channel.Items {
			links = append(links, item.Link)
		}
	} else {
		doc, err := html.Parse(strings.NewReader(string(b)))
		if err != nil {
			return nil, err
		}
		var visit func(*html.Node)
		visit = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				href := nodeAttr(n, "href")
				if u, e := url.Parse(href); e == nil && (u.Hostname() == "duckduckgo.com" || u.Hostname() == "html.duckduckgo.com" || u.Hostname() == "") && u.Query().Get("uddg") != "" {
					href = u.Query().Get("uddg")
				}
				if strings.HasPrefix(href, "/url?") {
					if u, e := url.Parse(href); e == nil {
						href = u.Query().Get("q")
						if href == "" {
							href = u.Query().Get("url")
						}
					}
				}
				links = append(links, href)
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				visit(c)
			}
		}
		visit(doc)
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, link := range links {
		if id := officialRJLink(link); id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// Search-result titles, snippets and destination URLs can contain the product
// number even when DLsite itself has not indexed the supplied translated name.
// These are discovery hints only; names and artwork still come from DLsite.
func parseExternalSearchRJ(b []byte, rss bool) ([]string, error) {
	ids, err := parseSearchRJ(b, rss)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	add := func(text string) {
		alphaNum := func(b byte) bool { return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
		for _, m := range rjPattern.FindAllStringIndex(text, -1) {
			if m[0] > 0 && alphaNum(text[m[0]-1]) || m[1] < len(text) && alphaNum(text[m[1]]) {
				continue
			}
			id := strings.ToUpper(text[m[0]:m[1]])
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if rss {
		var feed struct {
			Channel struct {
				Items []struct {
					Title       string `xml:"title"`
					Link        string `xml:"link"`
					Description string `xml:"description"`
				} `xml:"item"`
			} `xml:"channel"`
		}
		if err := xml.Unmarshal(b, &feed); err != nil {
			return nil, err
		}
		for _, item := range feed.Channel.Items {
			add(item.Title + " " + item.Description + " " + item.Link)
		}
	} else {
		doc, err := html.Parse(strings.NewReader(string(b)))
		if err != nil {
			return nil, err
		}
		var visit func(*html.Node)
		visit = func(n *html.Node) {
			if n.Type == html.ElementNode {
				if n.Data == "script" || n.Data == "style" || n.Data == "form" {
					return
				}
				if nodeClass(n, "result") || nodeClass(n, "result__snippet") || n.Data == "article" || n.Data == "li" || n.Data == "h3" {
					text := nodeText(n)
					if len(text) <= 6000 {
						add(text)
					}
				}
				if n.Data == "a" {
					add(nodeText(n))
					href := nodeAttr(n, "href")
					if decoded, err := url.QueryUnescape(href); err == nil {
						href = decoded
					}
					add(href)
				}
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				visit(child)
			}
		}
		visit(doc)
	}
	return ids, nil
}

func searchEngineDLsite(query string) ([]Candidate, error) {
	term := query + " RJ"
	endpoints := []string{
		"https://html.duckduckgo.com/html/?" + url.Values{"q": {term}}.Encode(),
		"https://search.yahoo.co.jp/search?" + url.Values{"p": {term}}.Encode(),
		"https://www.bing.com/search?" + url.Values{"q": {term}, "format": {"rss"}}.Encode(),
	}
	var last error
	out := []Candidate{}
	seen := map[string]bool{}
	for i, address := range endpoints {
		b, err := searchPage(address)
		if err != nil {
			last = err
			continue
		}
		ids, err := parseExternalSearchRJ(b, i == 2)
		if err != nil {
			last = err
			continue
		}
		exact := false
		for j, id := range ids {
			if j >= 6 || len(seen) >= 12 {
				break
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			cs, e := dlsiteLookup(id)
			if e != nil {
				last = e
				continue
			}
			for _, c := range cs {
				if c.Key != "dlsite:"+id {
					continue
				}
				// A search hit is not identity proof: verify the official title too.
				c.Exact = norm(c.Name) == norm(query)
				exact = exact || c.Exact
				c.Provider = "DLsite · 外部搜索引擎关联RJ编号"
				out = append(out, c)
			}
		}
		if exact {
			return out, nil
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	return nil, last
}
