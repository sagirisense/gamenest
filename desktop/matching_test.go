package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func emptySourceResponse(r *http.Request) *http.Response {
	body := `{"results":[],"items":[]}`
	switch r.URL.Hostname() {
	case "www.bing.com":
		body = "<rss><channel></channel></rss>"
	case "www.dlsite.com":
		body = `<div id="search_result_list"></div>`
	case "html.duckduckgo.com", "search.yahoo.co.jp":
		body = "<html></html>"
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func TestSearchTitleRemovesVersionsButKeepsSequels(t *testing.T) {
	for input, want := range map[string]string{"Game_v1.2.3": "Game", "Game Ver.1.04": "Game", "Game（v2.0）": "Game", "Game 1.02": "Game", "Game 2": "Game 2", "Game II": "Game II", "游戏 版本1.2": "游戏"} {
		if got := backgroundName(input); got != want {
			t.Errorf("%q => %q, want %q", input, got, want)
		}
	}
	if titleSimilarity("Game 2", "Game 3") != 0 {
		t.Fatal("sequel numbers ignored")
	}
}
func TestAllSourcesSearchedBeforeRanking(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	seen := map[string]bool{}
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		seen[r.URL.Hostname()] = true
		if r.URL.Hostname() == "store.steampowered.com" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"items":[{"id":1,"name":"Example","tiny_image":""}]}`)), Header: make(http.Header)}, nil
		}
		return emptySourceResponse(r), nil
	})
	cs, _ := collectEnrichmentCandidates(Game{Folder: t.TempDir()}, "", "Example")
	if len(cs) == 0 || !seen["api.vndb.org"] || !seen["www.dlsite.com"] || !seen["store.steampowered.com"] {
		t.Fatal(seen, cs)
	}
}
func TestClosestCandidateWinsAndTiesNeedReview(t *testing.T) {
	g := Game{Folder: t.TempDir()}
	all := []Candidate{{Key: "steam:1", Name: "Something Else"}, {Key: "vndb:v1", Name: "Example"}, {Key: "dlsite:RJ123456", Name: "Example II"}}
	ranked, best, ok := rankEnrichmentCandidates(all, "Example", g, "")
	if !ok || best.Key != "vndb:v1" || ranked[0].Score != 1 {
		t.Fatal(ranked, best, ok)
	}
	all = append(all, Candidate{Key: "vndb:v2", Name: "Example"})
	if _, _, ok := rankEnrichmentCandidates(all, "Example", g, ""); ok {
		t.Fatal("ambiguous works selected")
	}
}
