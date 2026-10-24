package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const dlsiteFixture = `<div id="search_result_list"><ul>
<li data-list_item_product_id="RJ123456"><dt class="work_thumb"><img :src="is_show ? '//img.dlsite.jp/modpub/images2/work/doujin/RJ124000/RJ123456_img_main.jpg' : ''"></dt><dd class="work_name"><a href="https://www.dlsite.com/maniax/work/=/product_id/RJ123456.html?track=1" title="ゲーム &amp; Example">ゲーム &amp; Example</a></dd></li>
<li data-list_item_product_id="RJ123456"><dd class="work_name"><a href="https://www.dlsite.com/maniax/work/=/product_id/RJ123456.html">Duplicate</a></dd></li>
<li data-list_item_product_id="RJ123457"><dd class="work_name"><a href="https://attacker.example/work/=/product_id/RJ123457.html">Wrong host</a></dd></li>
</ul></div>`

func TestDLsiteKeywordUsesFormEncodingInPath(t *testing.T) {
	for _, query := range []string{"BLACK SOULS", "ゲーム /?# Example", "A+B C"} {
		u, err := url.Parse(dlsiteSearchURL(query))
		if err != nil {
			t.Fatal(err)
		}
		encoded := strings.TrimSuffix(strings.TrimPrefix(u.EscapedPath(), "/maniax/fsr/=/keyword/"), "/order/trend_d/per_page/30")
		decoded, err := url.QueryUnescape(encoded)
		if err != nil || decoded != query || strings.Contains(encoded, "%20") || u.RawQuery != "" {
			t.Fatalf("query %q encoded as %q, decoded %q: %v", query, encoded, decoded, err)
		}
	}
}

func TestDLsiteSearchParsesTitlesAndOfficialMainCover(t *testing.T) {
	cs, e := parseDLsiteResults(strings.NewReader(dlsiteFixture), "ゲーム & Example")
	if e != nil || len(cs) != 1 {
		t.Fatal(cs, e)
	}
	c := cs[0]
	if c.Name != "ゲーム & Example" || !c.Exact || c.Key != "dlsite:RJ123456" || c.URL != "https://www.dlsite.com/maniax/work/=/product_id/RJ123456.html" || c.Image != "https://img.dlsite.jp/modpub/images2/work/doujin/RJ124000/RJ123456_img_main.jpg" {
		t.Fatal(c)
	}
	if _, e := parseDLsiteResults(strings.NewReader("<html><h1>Maintenance</h1></html>"), "test"); e == nil {
		t.Fatal("maintenance page treated as search results")
	}
	cs, e = parseDLsiteResults(strings.NewReader(`<div id="search_result_list"></div>`), "test")
	if e != nil || len(cs) != 0 {
		t.Fatal(cs, e)
	}
}

func TestDLsiteLookupAcceptsNameAndEscapesQuery(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	query := "ゲーム /?# Example"
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == "www.bing.com" || r.URL.Hostname() == "www.google.com" || r.URL.Hostname() == "html.duckduckgo.com" || r.URL.Hostname() == "search.yahoo.co.jp" {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header)}, nil
		}

		if r.URL.Hostname() == "api.vndb.org" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"results":[]}`)), Header: make(http.Header)}, nil
		}
		if r.URL.Hostname() != "www.dlsite.com" || !strings.Contains(r.URL.EscapedPath(), url.QueryEscape(query)) || r.URL.RawQuery != "" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(dlsiteFixture)), Header: make(http.Header)}, nil
	})
	cs, e := dlsiteLookup(query)
	if e != nil || len(cs) != 1 || cs[0].Provider != "DLsite · 名称检索" {
		t.Fatal(cs, e)
	}
}

func TestDLsiteNameFailureUsesClearlyLabeledVNDBBackup(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == "www.bing.com" || r.URL.Hostname() == "www.google.com" || r.URL.Hostname() == "html.duckduckgo.com" || r.URL.Hostname() == "search.yahoo.co.jp" {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header)}, nil
		}

		status, body := 503, "maintenance"
		if r.URL.Hostname() == "api.vndb.org" {
			status = 200
			body = `{"results":[{"id":"v1","title":"Example","titles":[],"image":{"url":"https://t.vndb.org/cv/1.jpg"}}]}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	cs, e := dlsiteLookup("Example")
	if e != nil || len(cs) != 1 || cs[0].Provider != "VNDB · DLsite名称备用" || cs[0].URL != "https://vndb.org/v1" {
		t.Fatal(cs, e)
	}
}

func TestDLsiteNameResolvesRJFromVNDB(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	for _, title := range []string{"Example", "Similar Example"} {
		http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.Hostname() == "www.bing.com" || r.URL.Hostname() == "www.google.com" || r.URL.Hostname() == "html.duckduckgo.com" || r.URL.Hostname() == "search.yahoo.co.jp" {
				return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header)}, nil
			}

			body := ""
			if r.URL.Hostname() == "www.dlsite.com" && r.URL.Path != "/maniax/product/info/ajax" {
				return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header)}, nil
			}
			switch r.URL.Path {
			case "/kana/vn":
				body = `{"results":[{"id":"v1","title":"` + title + `","titles":[]}]}`
			case "/kana/release":
				body = `{"results":[{"extlinks":[{"url":"https://www.dlsite.com/maniax/work/=/product_id/RJ123456.html"}]}]}`
			case "/maniax/product/info/ajax":
				if r.URL.Query().Get("product_id") != "RJ123456" {
					t.Fatal(r.URL)
				}
				body = `{"RJ123456":{"work_name":"Official Title","work_image":"//img.dlsite.jp/RJ123456.jpg"}}`
			default:
				t.Fatalf("unexpected request: %s", r.URL)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		cs, err := dlsiteLookup("Example")
		if err != nil || len(cs) != 1 || cs[0].Key != "dlsite:RJ123456" || cs[0].Exact != (title == "Example") || !strings.Contains(cs[0].Provider, "VNDB关联RJ") {
			t.Fatal(cs, err)
		}
	}
}
