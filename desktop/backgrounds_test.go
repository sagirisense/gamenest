package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackgroundMatchRejectsAmbiguousTitles(t *testing.T) {
	cs := []Candidate{{Key: "steam:1", Exact: true}, {Key: "steam:2", Exact: true}}
	if _, ok := exactBackgroundCandidate(cs); ok {
		t.Fatal("ambiguous exact titles auto-selected")
	}
	cs[1].Exact = false
	if c, ok := exactBackgroundCandidate(cs); !ok || c.Key != "steam:1" {
		t.Fatal("unique exact match not selected")
	}
	if backgroundName("KaijuPrincess v1.04") != "Kaiju Princess" {
		t.Fatal("folder alias normalization failed")
	}
}

type backgroundTransport func(*http.Request) (*http.Response, error)

func (f backgroundTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOnlineBackgroundPreservesCoverAndPersistsSource(t *testing.T) {
	a := testApp(t)
	a.meta["sample"] = map[string]any{"name": "自定义名称", "image": "covers/keep.png", "source_url": "https://vndb.org/v1"}
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1920, 620))); e != nil {
		t.Fatal(e)
	}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://cdn.akamai.steamstatic.com/steam/apps/123/library_hero.jpg" {
			t.Fatalf("unexpected request: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b.Bytes())), Header: make(http.Header)}, nil
	})
	record, e := a.FindBackground("sample", "123")
	if e != nil || record.Status != "found" || record.Width != 1920 || record.AppID != "123" {
		t.Fatal(record, e)
	}
	if _, e := os.Stat(filepath.Join(a.dir, filepath.FromSlash(record.Image))); e != nil {
		t.Fatal(e)
	}
	if str(a.meta["sample"], "image") != "covers/keep.png" || str(a.meta["sample"], "name") != "自定义名称" {
		t.Fatal("cover or custom name modified")
	}
	var stored map[string]BackgroundRecord
	if e := readJSON(filepath.Join(a.dir, "backgrounds.json"), &stored); e != nil || stored["sample"].SourceURL != "https://store.steampowered.com/app/123/" {
		t.Fatal(stored, e)
	}
	d, e := a.GetLibrary()
	if e != nil || d.Games[0].HeroImage == "" {
		t.Fatal("new background not immediately exposed", e)
	}
	if result, e := a.FindBackground("sample", ""); e != nil || result.Status != "existing" {
		t.Fatal("cached background not skipped", result, e)
	}
}

func TestBackgroundRejectsThumbnailsAndInvalidContent(t *testing.T) {
	if _, _, e := validBackground(makePNG(color.RGBA{255, 0, 0, 255})); e == nil {
		t.Fatal("thumbnail accepted")
	}
	if _, _, e := validBackground([]byte("<html>503</html>")); e == nil {
		t.Fatal("HTML accepted")
	}
	a := testApp(t)
	if _, e := a.UseBackground("sample", "123"); e == nil {
		t.Fatal("unreviewed candidate accepted")
	}
}

func TestBackgroundUsesHeroHashFromOfficialCatalog(t *testing.T) {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1920, 620)))
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	heroURL := "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/123/hero-hash/library_hero.jpg?t=1"
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		status, body := 200, []byte{}
		switch {
		case strings.Contains(r.URL.Path, "IStoreBrowseService"):
			body = []byte(`{"response":{"store_items":[{"id":123,"success":1,"type":0,"assets":{"asset_url_format":"steam/apps/123/${FILENAME}?t=1","library_hero":"hero-hash/library_hero.jpg"}}]}}`)
		case r.URL.String() == heroURL:
			body = b.Bytes()
		default:
			status = 404
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})
	_, address, config, _, e := downloadSteamBackground("123")
	if e != nil || address != heroURL || config.Width != 1920 {
		t.Fatal(address, config, e)
	}
}

func TestFailedBackgroundLookupKeepsPreviousArtwork(t *testing.T) {
	a := testApp(t)
	a.backgrounds = map[string]BackgroundRecord{"sample": {Image: "backgrounds/keep.jpg", AppID: "1", URL: "https://cdn.akamai.steamstatic.com/steam/apps/1/library_hero.jpg"}}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header)}, nil
	})
	r, e := a.FindBackground("sample", "123")
	if e != nil || r.Status != "failed" || r.Image != "backgrounds/keep.jpg" || r.AppID != "1" {
		t.Fatal(r, e)
	}
}
