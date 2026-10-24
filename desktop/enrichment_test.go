package main

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnrichmentFillsMissingFieldsAndPreservesUserData(t *testing.T) {
	for _, custom := range []bool{false, true} {
		a := testApp(t)
		a.library.Games[0].AppID = "123"
		a.backgrounds = map[string]BackgroundRecord{"sample": {Image: "backgrounds/keep.jpg"}}
		if err := os.MkdirAll(filepath.Join(a.dir, "backgrounds"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a.dir, "backgrounds", "keep.jpg"), []byte("existing"), 0600); err != nil {
			t.Fatal(err)
		}
		if custom {
			a.meta["sample"] = map[string]any{"name": "手动名称", "image": "covers/keep.png", "manual": true, "preferred_exe": "keep.exe"}
			if err := os.MkdirAll(filepath.Join(a.dir, "covers"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(a.dir, "covers", "keep.png"), []byte("existing"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		var imageBytes bytes.Buffer
		if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 32, 48))); err != nil {
			t.Fatal(err)
		}
		old := http.DefaultTransport
		http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
			if custom {
				t.Fatalf("completed game made a network request: %s", r.URL)
			}
			if r.URL.Hostname() != "store.steampowered.com" && r.URL.Path != "/cover.png" {
				return emptySourceResponse(r), nil
			}
			body := io.NopCloser(strings.NewReader(`{"123":{"success":true,"data":{"name":"Official Name","steam_appid":123,"header_image":"https://cdn.akamai.steamstatic.com/cover.png"}}}`))
			if r.URL.Path == "/cover.png" {
				body = io.NopCloser(bytes.NewReader(imageBytes.Bytes()))
			}
			return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
		})
		result, err := a.EnrichGame("sample")
		http.DefaultTransport = old
		if err != nil || len(result.Errors) > 0 {
			t.Fatal(result, err)
		}
		if custom {
			if result.Name || result.Cover || result.Background || str(a.meta["sample"], "name") != "手动名称" || str(a.meta["sample"], "image") != "covers/keep.png" || str(a.meta["sample"], "preferred_exe") != "keep.exe" {
				t.Fatal(result, a.meta)
			}
		} else {
			if !result.Name || !result.Cover || str(a.meta["sample"], "name") != "Official Name" {
				t.Fatal(result, a.meta)
			}
			reloaded := newAppWithDataDir(a.dir)
			if str(reloaded.meta["sample"], "name") != "Official Name" || !exists(filepath.Join(a.dir, str(reloaded.meta["sample"], "image"))) {
				t.Fatal("enrichment not persisted")
			}
		}
	}
}

func TestEnrichmentKeepsAmbiguousCandidatesWithoutChangingName(t *testing.T) {
	a := testApp(t)
	a.backgrounds = map[string]BackgroundRecord{"sample": {Image: "backgrounds/keep.jpg"}}
	if err := os.MkdirAll(filepath.Join(a.dir, "backgrounds"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.dir, "backgrounds", "keep.jpg"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"results":[]}`
		status := 200
		if r.URL.Hostname() == "store.steampowered.com" {
			body = `{"items":[{"id":1,"name":"Original","tiny_image":""},{"id":2,"name":"Original","tiny_image":""}]}`
		}
		if r.URL.Hostname() == "www.dlsite.com" {
			status = 503
			body = "maintenance"
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	result, err := a.EnrichGame("sample")
	if err != nil || result.Name || result.Cover || !result.Review || str(a.meta["sample"], "name") != "" {
		t.Fatal(result, err, a.meta)
	}
	cs, ok := a.meta["sample"]["candidates"].([]Candidate)
	if !ok || len(cs) != 2 {
		t.Fatal("review candidates lost", a.meta)
	}
}
