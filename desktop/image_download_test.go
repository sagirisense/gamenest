package main

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
)

type interruptedImage struct{}

func (interruptedImage) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (interruptedImage) Close() error             { return nil }

func TestImageDownloadRetriesInterruptedResponse(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	for _, bodyFailure := range []bool{false, true} {
		calls := 0
		http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if calls < 3 {
				if bodyFailure {
					return &http.Response{StatusCode: 200, Body: interruptedImage{}, Header: make(http.Header)}, nil
				}
				return nil, io.ErrUnexpectedEOF
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("complete")), Header: make(http.Header)}, nil
		})
		b, err := fetchLibraryImage("https://shared.akamai.steamstatic.com/cover.jpg", 1024)
		if err != nil || string(b) != "complete" || calls != 3 {
			t.Fatal(string(b), calls, err)
		}
	}
}

func TestCoverDownloadFallsBackToSteamHeader(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 32, 48))); err != nil {
		t.Fatal(err)
	}
	calls := 0
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) {
		body := ""
		if strings.Contains(r.URL.Path, "capsule_") {
			calls++
			return nil, io.ErrUnexpectedEOF
		}
		if r.URL.Hostname() == "store.steampowered.com" {
			body = `{"1227890":{"success":true,"data":{"name":"Example","steam_appid":1227890,"header_image":"https://shared.akamai.steamstatic.com/header.jpg"}}}`
		} else if r.URL.Path == "/header.jpg" {
			body = data.String()
		} else {
			t.Fatalf("unexpected request %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	a := testApp(t)
	a.meta["sample"] = map[string]any{"name": "keep", "image": "covers/keep.png", "candidates": []Candidate{{Key: "steam:1227890", Name: "Example", Image: "https://shared.akamai.steamstatic.com/capsule_231x87_schinese.jpg"}}}
	if err := a.UseCandidate("sample", "steam:1227890"); err != nil {
		t.Fatal(err)
	}
	if calls != 3 || str(a.meta["sample"], "image") == "covers/keep.png" {
		t.Fatal(calls, a.meta)
	}
}

func TestPermanentImageFailureIsBoundedAndKeepsMetadata(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	calls := 0
	http.DefaultTransport = backgroundTransport(func(r *http.Request) (*http.Response, error) { calls++; return nil, io.ErrUnexpectedEOF })
	a := testApp(t)
	a.meta["sample"] = map[string]any{"name": "keep", "image": "covers/keep.png", "candidates": []Candidate{{Key: "dlsite:RJ123456", Name: "Other", Image: "https://img.dlsite.jp/main.jpg"}}}
	if err := a.UseCandidate("sample", "dlsite:RJ123456"); err == nil {
		t.Fatal("failure accepted")
	}
	if calls != 3 || str(a.meta["sample"], "name") != "keep" || str(a.meta["sample"], "image") != "covers/keep.png" {
		t.Fatal(calls, a.meta)
	}
}
