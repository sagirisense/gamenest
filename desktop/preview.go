package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

func servePreview(a *App, assets fs.FS) {
	random := make([]byte, 24)
	rand.Read(random)
	token := hex.EncodeToString(random)
	mux := http.NewServeMux()
	mux.HandleFunc("/library-image/", a.serveImage)
	mux.HandleFunc("/api/library", func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "127.0.0.1:8787" && r.Host != "localhost:8787" {
			http.Error(w, "forbidden", 403)
			return
		}
		d, e := a.GetLibrary()
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]any{"library": d, "token": token})
	})
	mux.HandleFunc("/api/call/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("X-Library-Token") != token || (r.Host != "127.0.0.1:8787" && r.Host != "localhost:8787") {
			http.Error(w, "forbidden", 403)
			return
		}
		var args []string
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&args); e != nil {
			http.Error(w, "invalid input", 400)
			return
		}
		arg := func(i int) string {
			if i < len(args) {
				return args[i]
			}
			return ""
		}
		var result any
		var e error
		switch strings.TrimPrefix(r.URL.Path, "/api/call/") {
		case "SetTheme":
			e = a.SetTheme(arg(0))
		case "ToggleFavorite":
			result, e = a.ToggleFavorite(arg(0))
		case "SaveMetadata":
			e = a.SaveMetadata(arg(0), arg(1), arg(2))
		case "ResetMetadata":
			e = a.ResetMetadata(arg(0))
		case "SearchMetadata":
			result, e = a.SearchMetadata(arg(0), arg(1), arg(2))
		case "UseCandidate":
			e = a.UseCandidate(arg(0), arg(1))
		case "KeepMetadata":
			e = a.KeepMetadata(arg(0))
		case "OpenFolder":
			e = a.OpenFolder(arg(0))
		case "LaunchGame":
			e = a.LaunchGame(arg(0), arg(1))
		case "ScanGames":
			result, e = a.ScanGames()
		case "GetScanSettings":
			result = a.GetScanSettings()
		case "SaveScanSettings":
			result, e = a.SaveScanSettings(arg(0))
		case "PickScanFolder":
			result, e = a.PickScanFolder()
		case "ImportCover":
			result, e = a.ImportCover(arg(0))
		case "FindBackground":
			result, e = a.FindBackground(arg(0), arg(1))
		case "EnrichGame":
			result, e = a.EnrichGame(arg(0))
		case "UseBackground":
			result, e = a.UseBackground(arg(0), arg(1))
		case "GetArtworkSettings":
			result = a.GetArtworkSettings()
		case "SaveSteamGridDBKey":
			result, e = a.SaveSteamGridDBKey(arg(0))
		case "ClearSteamGridDBKey":
			result, e = a.ClearSteamGridDBKey()
		case "TestSteamGridDBKey":
			result, e = a.TestSteamGridDBKey(arg(0))
		case "SearchArtworkGames":
			result, e = a.SearchArtworkGames(arg(0), arg(1))
		case "GetSteamGridDBArtwork":
			result, e = a.GetSteamGridDBArtwork(arg(0), arg(1), arg(2))
		case "UseArtwork":
			e = a.UseArtwork(arg(0), arg(1), arg(2))
		default:
			e = fmt.Errorf("未知操作")
		}
		w.Header().Set("Content-Type", "application/json")
		if e != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
			return
		}
		json.NewEncoder(w).Encode(result)
	})
	mux.Handle("/", http.FileServer(http.FS(assets)))
	log.Println("桌面版预览：http://127.0.0.1:8787")
	log.Fatal(http.ListenAndServe("127.0.0.1:8787", mux))
}
