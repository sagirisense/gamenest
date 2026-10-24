package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testArtworkKey = "0123456789abcdef0123456789abcdef"

func TestArtworkCredentialsProtectedAndNeverReturned(t *testing.T) {
	a := testApp(t)
	if a.GetArtworkSettings().Configured {
		t.Fatal("unexpected key")
	}
	if _, e := a.SaveSteamGridDBKey(testArtworkKey); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(a.dir, "integrations.json"))
	if e != nil || bytes.Contains(b, []byte(testArtworkKey)) {
		t.Fatal("key not protected", e)
	}
	var stored IntegrationConfig
	if e := json.Unmarshal(b, &stored); e != nil {
		t.Fatal(e)
	}
	a.integrations = stored
	if key, e := a.steamGridDBKey(); e != nil || key != testArtworkKey {
		t.Fatal("protected key could not be read", e)
	}
	public, _ := json.Marshal(a.GetArtworkSettings())
	if bytes.Contains(public, []byte(testArtworkKey)) || bytes.Contains(public, []byte(stored.SteamGridDBKey)) {
		t.Fatal("key exposed by settings")
	}
	if _, e := a.SaveSteamGridDBKey("bad\r\nheader"); e == nil {
		t.Fatal("invalid key accepted")
	}
	if _, e := a.ClearSteamGridDBKey(); e != nil || a.GetArtworkSettings().Configured {
		t.Fatal("key not cleared", e)
	}
}

func artworkPNG(w, h int) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)))
	return b.Bytes()
}

func TestSteamGridDBSelectionPreservesNamesAndPrefersSteamOfficialArt(t *testing.T) {
	a := testApp(t)
	if _, e := a.SaveSteamGridDBKey(testArtworkKey); e != nil {
		t.Fatal(e)
	}
	a.meta["sample"] = map[string]any{"name": "自定义游戏名", "source_url": "https://store.steampowered.com/app/123/", "image": "covers/old.png"}
	cover, hero := artworkPNG(600, 900), artworkPNG(1920, 620)
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		body := []byte{}
		status := 200
		if r.URL.Hostname() == "www.steamgriddb.com" {
			if r.Header.Get("Authorization") != "Bearer "+testArtworkKey {
				t.Fatal("missing API authentication")
			}
			switch {
			case strings.Contains(r.URL.Path, "/search/autocomplete/"):
				body = []byte(`{"success":true,"data":[{"id":55,"name":"Example"}]}`)
			case strings.Contains(r.URL.Path, "/games/steam/123"):
				body = []byte(`{"success":true,"data":{"id":55,"name":"Example"}}`)
			case strings.Contains(r.URL.Path, "/grids/game/55"):
				body = []byte(`{"success":true,"data":[{"id":1,"url":"https://cdn2.steamgriddb.com/grid/test.png","thumb":"https://cdn2.steamgriddb.com/grid/thumb.png","width":600,"height":900,"style":"alternate","author":{"name":"Artist"}},{"id":2,"url":"https://cdn2.steamgriddb.com/grid/blur.png","width":600,"height":900,"style":["blurred"]},{"id":3,"url":"https://evil.example/test.png","width":600,"height":900}]}`)
			case strings.Contains(r.URL.Path, "/heroes/game/55"):
				body = []byte(`{"success":true,"data":[{"id":4,"url":"https://cdn2.steamgriddb.com/hero/test.png","width":1920,"height":620,"style":"no_logo","author":{"name":"Artist"}},{"id":5,"url":"https://cdn2.steamgriddb.com/hero/small.png","width":460,"height":215}]}`)
			default:
				t.Fatal("unexpected API route", r.URL.Path)
			}
		} else {
			if r.Header.Get("Authorization") != "" {
				t.Fatal("credential sent to image CDN")
			}
			if strings.Contains(r.URL.Path, "/grid/") {
				body = cover
			} else {
				body = hero
			}
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})
	if _, e := a.TestSteamGridDBKey(""); e != nil {
		t.Fatal(e)
	}
	games, e := a.SearchArtworkGames("sample", "123")
	if e != nil || len(games) != 1 {
		t.Fatal(games, e)
	}
	cs, e := a.GetSteamGridDBArtwork("sample", "55", "cover")
	if e != nil || len(cs) != 1 || cs[0].Author != "Artist" {
		t.Fatal(cs, e)
	}
	if e := a.UseArtwork("sample", "cover", cs[0].Key); e != nil {
		t.Fatal(e)
	}
	if str(a.meta["sample"], "name") != "自定义游戏名" || str(a.meta["sample"], "source_url") != "https://store.steampowered.com/app/123/" || str(a.meta["sample"], "cover_provider") != "SteamGridDB" {
		t.Fatal("cover selection modified identity")
	}
	cs, e = a.GetSteamGridDBArtwork("sample", "55", "background")
	if e != nil || len(cs) != 1 {
		t.Fatal(cs, e)
	}
	if e := a.UseArtwork("sample", "background", cs[0].Key); e != nil {
		t.Fatal(e)
	}
	d, _ := a.GetLibrary()
	if d.Games[0].HeroImage == "" || d.Games[0].Background.Provider != "SteamGridDB" {
		t.Fatal("community hero not exposed")
	}
	if e := a.UseArtwork("sample", "background", "sgdb:grid:1"); e == nil {
		t.Fatal("cover adopted as background")
	}
	r, e := a.FindBackground("sample", "")
	if e != nil || r.Provider != "Steam" || r.Status != "found" {
		t.Fatal("official Steam artwork not preferred", r, e)
	}
}

func TestSteamGridDBErrorsDoNotExposeKeyAndRedirectsAreBlocked(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	for _, status := range []int{401, 429, 302} {
		calls := 0
		http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("private response " + testArtworkKey)), Header: http.Header{"Location": []string{"https://evil.example/capture"}}}, nil
		})
		e := sgdbRequest(testArtworkKey, "/search/autocomplete/Portal", nil)
		if e == nil || strings.Contains(e.Error(), testArtworkKey) || calls != 1 {
			t.Fatal("unsafe API error handling", status, calls)
		}
	}
}
