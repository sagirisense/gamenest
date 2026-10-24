package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	g := Game{ID: "sample", Name: "Original", Folder: dir, Exe: filepath.Join(dir, "game.exe"), Executables: []string{filepath.Join(dir, "game.exe")}, Source: "本地游戏"}
	return &App{dir: dir, library: Library{Games: []Game{g}}, meta: map[string]map[string]any{}, prefs: Preferences{Theme: "system", Favorites: map[string]bool{}, Played: map[string]string{}}}
}
func TestMetadataPreservedOnRename(t *testing.T) {
	a := testApp(t)
	a.meta["sample"] = map[string]any{"image": "covers/original.png", "source_url": "https://vndb.org/v1", "candidates": []string{"candidate"}}
	if e := a.SaveMetadata("sample", "新名称", a.library.Games[0].Exe); e != nil {
		t.Fatal(e)
	}
	var stored map[string]map[string]any
	if e := readJSON(filepath.Join(a.dir, "metadata.json"), &stored); e != nil {
		t.Fatal(e)
	}
	if str(stored["sample"], "image") != "covers/original.png" || str(stored["sample"], "name") != "新名称" {
		t.Fatal(stored)
	}
	d, e := a.GetLibrary()
	if e != nil || d.Games[0].Name != "新名称" || d.Games[0].OriginalName != "Original" {
		t.Fatal(d, e)
	}
}
func TestThemeAndFavoritePersist(t *testing.T) {
	a := testApp(t)
	if e := a.SetTheme("dark"); e != nil {
		t.Fatal(e)
	}
	if _, e := a.ToggleFavorite("sample"); e != nil {
		t.Fatal(e)
	}
	var prefs Preferences
	if e := readJSON(filepath.Join(a.dir, "preferences.json"), &prefs); e != nil {
		t.Fatal(e)
	}
	if prefs.Theme != "dark" || !prefs.Favorites["sample"] {
		t.Fatal(prefs)
	}
	if e := a.SetTheme("unknown"); e == nil {
		t.Fatal("invalid theme accepted")
	}
}
func TestLaunchRejectsUnregisteredExecutable(t *testing.T) {
	a := testApp(t)
	if e := a.LaunchGame("sample", `C:\Windows\System32\calc.exe`); e == nil {
		t.Fatal("unregistered executable accepted")
	}
	if e := a.LaunchGame("sample", ""); e == nil {
		t.Fatal("missing executable accepted")
	}
}

func TestLaunchUsesGameDirectoryAndRecordsRecentPlay(t *testing.T) {
	a := testApp(t)
	source := filepath.Join(a.dir, "launch_helper.go")
	code := "package main\nimport \"os\"\nfunc main(){cwd,_:=os.Getwd();_ = os.WriteFile(\"launch-result.txt\",[]byte(cwd),0600)}\n"
	if e := os.WriteFile(source, []byte(code), 0600); e != nil {
		t.Fatal(e)
	}
	helper := a.library.Games[0].Exe
	if output, e := exec.Command("go", "build", "-ldflags=-H=windowsgui", "-o", helper, source).CombinedOutput(); e != nil {
		t.Fatalf("build fixture: %v %s", e, output)
	}
	if e := a.LaunchGame("sample", ""); e != nil {
		t.Fatal(e)
	}
	result := filepath.Join(a.dir, "launch-result.txt")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, e := os.ReadFile(result); e == nil {
			if !strings.EqualFold(string(b), a.dir) {
				t.Fatalf("wrong workdir: %s", b)
			}
			if a.prefs.Played["sample"] == "" {
				t.Fatal("recent play not recorded")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("launch fixture did not run")
}
func TestAppIDDetectionAndURLValidation(t *testing.T) {
	a := testApp(t)
	if e := os.WriteFile(filepath.Join(a.dir, "steam_appid.txt"), []byte("\xef\xbb\xbf2179300\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if id := detectAppID(a.library.Games[0]); id != "2179300" {
		t.Fatal(id)
	}
	if parseAppID("https://store.steampowered.com/app/2179300/Name/") != "2179300" {
		t.Fatal("URL lookup failed")
	}
	if parseAppID("https://store.steampowered.com.evil.example/app/2179300") != "" {
		t.Fatal("unsafe URL accepted")
	}
	if allowedImage("http://127.0.0.1/secret") || allowedImage("https://vndb.org.attacker.example/x.jpg") {
		t.Fatal("unsafe image URL accepted")
	}
}
func makePNG(c color.RGBA) []byte {
	im := image.NewRGBA(image.Rect(0, 0, 2, 2))
	im.Set(0, 0, c)
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.Bytes()
}
func TestCoverUpdatesURLAndHandler(t *testing.T) {
	a := testApp(t)
	p, e := a.saveImage("sample", makePNG(color.RGBA{R: 255, A: 255}))
	if e != nil {
		t.Fatal(e)
	}
	a.meta["sample"] = map[string]any{"image": p}
	before, _ := a.GetLibrary()
	p2, e := a.saveImage("sample", makePNG(color.RGBA{B: 255, A: 255}))
	if e != nil {
		t.Fatal(e)
	}
	a.meta["sample"]["image"] = p2
	after, _ := a.GetLibrary()
	if p == p2 || before.Games[0].Image == after.Games[0].Image {
		t.Fatal("cover URL did not change")
	}
	r := httptest.NewRecorder()
	a.serveImage(r, httptest.NewRequest("GET", after.Games[0].Image, nil))
	if r.Code != 200 || !strings.HasPrefix(r.Header().Get("Content-Type"), "image/png") {
		t.Fatal(r.Code, r.Header())
	}
	bad := httptest.NewRecorder()
	a.serveImage(bad, httptest.NewRequest("GET", "/library-image/../../metadata.json", nil))
	if bad.Code != 404 {
		t.Fatal("unregistered image path served")
	}
}
func TestLibrarySnapshotDoesNotMutateStorage(t *testing.T) {
	a := testApp(t)
	a.meta["sample"] = map[string]any{"name": "Display"}
	d, _ := a.GetLibrary()
	d.Games[0].Metadata["name"] = "changed"
	if str(a.meta["sample"], "name") != "Display" {
		t.Fatal("returned metadata aliases storage")
	}
	b, _ := json.Marshal(d)
	if len(b) == 0 {
		t.Fatal("not serializable")
	}
}
func TestInvalidUploadIsRejected(t *testing.T) {
	a := testApp(t)
	if _, e := a.saveImage("sample", []byte("<html>not an image</html>")); e == nil {
		t.Fatal("HTML was accepted as image")
	}
}

func TestMissingHeroDoesNotExposeDirectory(t *testing.T) {
	a := testApp(t)
	r := httptest.NewRecorder()
	a.serveImage(r, httptest.NewRequest("GET", "/library-image/sample?kind=hero", nil))
	if r.Code != 404 {
		t.Fatalf("missing hero should be 404, got %d", r.Code)
	}
}
