package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalRJReadsPaths(t *testing.T) {
	for _, tc := range []struct {
		name string
		game Game
	}{
		{"parent folder", Game{Folder: `D:\Games\[RJ123456]\作品\本体`}},
		{"executable", Game{Exe: `D:\Games\作品\rj123456\Game.exe`}},
		{"alternate executable", Game{Executables: []string{`D:\Games\作品\RJ123456.exe`}}},
		{"original name", Game{OriginalName: "作品 [RJ123456]"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ids := localRJIDs(tc.game, "")
			if len(ids) != 1 || ids[0] != "RJ123456" {
				t.Fatal(ids)
			}
		})
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "RJ123456"), 0755); err != nil {
		t.Fatal(err)
	}
	ids := localRJIDs(Game{Folder: root, Exe: filepath.Join(root, "RJ123456", "Game.exe"), Executables: []string{filepath.Join(root, "RJ654321", "Game.exe")}}, "")
	if len(ids) != 2 || ids[0] != "RJ123456" || ids[1] != "RJ654321" {
		t.Fatal(ids)
	}
}

func TestLocalRJReadsDocumentsAndRejectsPartialTokens(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{"readme.txt": "official RJ123456", "商店.url": "[InternetShortcut]\nURL=https://www.dlsite.com/maniax/work/=/product_id/RJ123456.html", "wrong.txt": "XRJ654321 RJ123456789012", "Game.exe": "RJ999999"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "Game_Data"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Game_Data", "readme.txt"), []byte("RJ999998"), 0600); err != nil {
		t.Fatal(err)
	}
	ids := localRJIDs(Game{Folder: root}, "")
	if len(ids) != 1 || ids[0] != "RJ123456" {
		t.Fatal(ids)
	}
	if err := os.WriteFile(filepath.Join(root, "link.url"), []byte{255, 254, 'R', 0, 'J', 0, '6', 0, '5', 0, '4', 0, '3', 0, '2', 0, '1', 0}, 0600); err != nil {
		t.Fatal(err)
	}
	ids = localRJIDs(Game{Folder: root}, "")
	if len(ids) != 2 {
		t.Fatal(ids)
	}
}

func TestSearchResultsOnlyAcceptOfficialProductLinks(t *testing.T) {
	rss := []byte(`<rss><channel><item><link>https://www.dlsite.com/maniax/work/=/product_id/RJ123456.html</link></item><item><link>https://www.dlsite.com.evil.example/maniax/work/=/product_id/RJ654321.html</link></item><item><link>https://evil.example/?url=https://www.dlsite.com/maniax/work/=/product_id/RJ654321.html</link></item></channel></rss>`)
	ids, err := parseSearchRJ(rss, true)
	if err != nil || len(ids) != 1 || ids[0] != "RJ123456" {
		t.Fatal(ids, err)
	}
	page := []byte(`<a href="/url?q=https%3A%2F%2Fwww.dlsite.com%2Fmaniax%2Fwork%2F%3D%2Fproduct_id%2FRJ123456.html">result</a><a href="https://evil.example/RJ654321">wrong</a>`)
	ids, err = parseSearchRJ(page, false)
	if err != nil || len(ids) != 1 || ids[0] != "RJ123456" {
		t.Fatal(ids, err)
	}
}

func TestSearchEngineVerifiesOfficialNameBeforeAutoMatch(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	for _, title := range []string{"Example", "Example II"} {
		http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
			body := ""
			switch r.URL.Hostname() {
			case "html.duckduckgo.com", "search.yahoo.co.jp":
				body = `<a href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fwww.dlsite.com%2Fmaniax%2Fwork%2F%3D%2Fproduct_id%2FRJ123456.html">result</a>`
			case "www.bing.com":
				body = `<rss><channel><item><link>https://www.dlsite.com/maniax/work/=/product_id/RJ123456.html</link></item></channel></rss>`
			case "www.dlsite.com":
				body = `{"RJ123456":{"work_name":"` + title + `","work_image":""}}`
			default:
				t.Fatalf("unexpected request %s", r.URL)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		cs, err := dlsiteNameLookup("Example")
		if err != nil || len(cs) != 1 || cs[0].Exact != (title == "Example") || !strings.Contains(cs[0].Provider, "搜索引擎") {
			t.Fatal(cs, err)
		}
	}
}

func TestAutomaticEnrichmentUsesLocalRJBeforeOnlineSearch(t *testing.T) {
	a := testApp(t)
	if err := os.WriteFile(filepath.Join(a.dir, "商店.url"), []byte("URL=https://www.dlsite.com/maniax/work/=/product_id/RJ123456.html"), 0600); err != nil {
		t.Fatal(err)
	}
	a.backgrounds = map[string]BackgroundRecord{"sample": {Image: "backgrounds/keep.jpg"}}
	if err := os.MkdirAll(filepath.Join(a.dir, "backgrounds"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.dir, "backgrounds", "keep.jpg"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() != "www.dlsite.com" || r.URL.Path != "/maniax/product/info/ajax" {
			return emptySourceResponse(r), nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"RJ123456":{"work_name":"Official Name","work_image":""}}`)), Header: make(http.Header)}, nil
	})
	result, err := a.EnrichGame("sample")
	if err != nil || !result.Name || str(a.meta["sample"], "name") != "Official Name" {
		t.Fatal(result, err)
	}
}

func TestMultipleLocalRJRequiresReview(t *testing.T) {
	a := testApp(t)
	if err := os.WriteFile(filepath.Join(a.dir, "readme.txt"), []byte("RJ123456 RJ654321"), 0600); err != nil {
		t.Fatal(err)
	}
	a.backgrounds = map[string]BackgroundRecord{"sample": {Image: "backgrounds/keep.jpg"}}
	if err := os.MkdirAll(filepath.Join(a.dir, "backgrounds"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.dir, "backgrounds", "keep.jpg"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() != "www.dlsite.com" || r.URL.Path != "/maniax/product/info/ajax" {
			return emptySourceResponse(r), nil
		}
		id := r.URL.Query().Get("product_id")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"` + id + `":{"work_name":"Original","work_image":""}}`)), Header: make(http.Header)}, nil
	})
	result, err := a.EnrichGame("sample")
	if err != nil || !result.Review || result.Name || str(a.meta["sample"], "name") != "" {
		t.Fatal(result, err, a.meta)
	}
}
