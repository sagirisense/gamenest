package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSteamSearchRegionPriority(t *testing.T) {
	for _, tc := range []struct {
		name, query, available string
		wantCalls              string
	}{
		{"name_hk", "Example", "HK", "HK,HK"},
		{"name_cn_fallback", "Example", "CN", "HK,HK,CN,CN"},
		{"appid_hk", "123", "HK", "HK"},
		{"appid_cn_fallback", "123", "CN", "HK,CN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = old })
			calls := []string{}
			http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
				region := r.URL.Query().Get("cc")
				calls = append(calls, region)
				body := `{"items":[]}`
				if tc.query == "123" {
					body = `{"123":{"success":false}}`
				}
				if region == tc.available {
					body = `{"items":[{"id":123,"name":"Example"}]}`
					if tc.query == "123" {
						body = `{"123":{"success":true,"data":{"name":"Example","steam_appid":123}}}`
					}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			cs, e := steamLookup(tc.query)
			if e != nil || len(cs) != 1 || strings.Join(calls, ",") != tc.wantCalls {
				t.Fatal(cs, e, calls)
			}
			if !strings.Contains(cs[0].URL, "cc="+strings.ToLower(tc.available)) {
				t.Fatal(cs[0])
			}
		})
	}
}

func TestSteamArtworkCatalogRegionFallback(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	calls := []string{}
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		var input struct {
			Context struct {
				Country string `json:"country_code"`
			} `json:"context"`
		}
		if e := json.Unmarshal([]byte(r.URL.Query().Get("input_json")), &input); e != nil {
			t.Fatal(e)
		}
		calls = append(calls, input.Context.Country)
		body := `{"response":{"store_items":[{"id":123,"success":0}]}}`
		if input.Context.Country == "CN" {
			body = `{"response":{"store_items":[{"id":123,"success":1,"name":"Example"}]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	items, e := steamCatalog([]string{"123"})
	if e != nil || len(items) != 1 || items[0].Success != 1 || strings.Join(calls, ",") != "HK,CN" {
		t.Fatal(items, e, calls)
	}
}

func TestSteamSearchFallsBackAfterHKServiceFailure(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		status, body := 503, `unavailable`
		if r.URL.Query().Get("cc") == "CN" {
			status = 200
			body = `{"items":[{"id":123,"name":"Example"}]}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	cs, e := steamLookup("Example")
	if e != nil || len(cs) != 1 || cs[0].Provider != "Steam · 国区" {
		t.Fatal(cs, e)
	}
}
