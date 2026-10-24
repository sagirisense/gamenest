package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func fixtureGame(t *testing.T, root, name string) string {
	t.Helper()
	folder := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(folder, "Game_Data"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "Game.exe"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	return folder
}

func saveScope(t *testing.T, a *App, roots, excluded []string) {
	t.Helper()
	b, _ := json.Marshal(ScanSettings{Roots: roots, Excluded: excluded})
	if _, err := a.SaveScanSettings(string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestFirstScanWithoutLibraryData(t *testing.T) {
	for _, missingDir := range []bool{true, false} {
		t.Run(fmt.Sprintf("missingDir=%t", missingDir), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "data")
			if !missingDir {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			a := newAppWithDataDir(dir)
			if lib, err := a.GetLibrary(); err != nil || len(lib.Games) != 0 {
				t.Fatalf("initial library: %+v %v", lib, err)
			}
			root := t.TempDir()
			fixtureGame(t, root, "FirstGame")
			// Set the scope in memory so ScanGames must create the data directory itself.
			a.prefs.Scan = &ScanSettings{Roots: []string{root}}
			lib, err := a.ScanGames()
			if err != nil || len(lib.Games) != 1 {
				t.Fatalf("first scan: %+v %v", lib, err)
			}
			reloaded, err := newAppWithDataDir(dir).GetLibrary()
			if err != nil || len(reloaded.Games) != 1 || reloaded.Games[0].ID != lib.Games[0].ID {
				t.Fatalf("reloaded library: %+v %v", reloaded, err)
			}
		})
	}
}

func TestCorruptLibraryStillReportsLoadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "games.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newAppWithDataDir(dir).GetLibrary(); err == nil {
		t.Fatal("corrupt library accepted")
	}
}

func TestScanScopeExcludesOldGamesAndKeepsMetadata(t *testing.T) {
	root := t.TempDir()
	allowed := fixtureGame(t, root, "Allowed")
	blocked := fixtureGame(t, root, "Blocked")
	sibling := fixtureGame(t, root, "BlockedSibling")
	outside := fixtureGame(t, t.TempDir(), "Outside")
	a := &App{dir: t.TempDir(), meta: map[string]map[string]any{}, prefs: Preferences{Theme: "dark", Favorites: map[string]bool{}, Played: map[string]string{}}, backgrounds: map[string]BackgroundRecord{}}
	for _, folder := range []string{allowed, blocked, outside} {
		id := stableID(folder)
		a.library.Games = append(a.library.Games, Game{ID: id, Name: filepath.Base(folder), Folder: folder, Exe: filepath.Join(folder, "Game.exe")})
		a.meta[id] = map[string]any{"name": "Custom", "image": "covers/kept.jpg"}
		a.prefs.Favorites[id] = true
	}
	saveScope(t, a, []string{root, allowed}, []string{blocked})
	if len(a.GetScanSettings().Roots) != 1 {
		t.Fatal("overlapping roots not deduplicated")
	}
	lib, err := a.ScanGames()
	if err != nil || len(lib.Games) != 2 {
		t.Fatalf("%+v %v", lib, err)
	}
	for _, g := range lib.Games {
		if g.Folder != allowed && g.Folder != sibling {
			t.Fatal("out of scope game", g.Folder)
		}
	}
	if a.meta[stableID(blocked)]["image"] != "covers/kept.jpg" || !a.prefs.Favorites[stableID(blocked)] || a.prefs.Theme != "dark" {
		t.Fatal("custom data changed")
	}
	var saved Preferences
	if err := readJSON(filepath.Join(a.dir, "preferences.json"), &saved); err != nil || saved.Scan == nil || len(saved.Scan.Excluded) != 1 {
		t.Fatal(saved, err)
	}
	// Excluded names must use a path boundary, not a raw string prefix.
	if pathWithin(sibling, blocked) {
		t.Fatal("sibling was excluded")
	}
}

func TestUnavailableRootAndInvalidScopePreserveLibrary(t *testing.T) {
	a := &App{dir: t.TempDir(), library: Library{Games: []Game{{ID: "keep"}}}}
	if got := a.GetScanSettings(); len(got.Roots) != 1 || got.Roots[0] != `D:\` {
		t.Fatal(got)
	}
	for _, raw := range []string{`{"roots":[]}`, `{"roots":["relative"]}`, `{"roots":["D:\\Games"],"excluded":["D:\\"]}`} {
		if _, err := a.SaveScanSettings(raw); err == nil {
			t.Fatal("invalid scope accepted", raw)
		}
	}
	saveScope(t, a, []string{filepath.Join(t.TempDir(), "missing")}, nil)
	if _, err := a.ScanGames(); err == nil {
		t.Fatal("missing root accepted")
	}
	if len(a.library.Games) != 1 || a.library.Games[0].ID != "keep" {
		t.Fatal("library changed")
	}
	if _, err := os.Stat(filepath.Join(a.dir, "games.json")); !os.IsNotExist(err) {
		t.Fatal("library overwritten")
	}
}
