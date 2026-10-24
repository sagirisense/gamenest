package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Game struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Source        string           `json:"source"`
	Executables   []string         `json:"executables"`
	AppID         any              `json:"appid"`
	Exe           string           `json:"exe"`
	Folder        string           `json:"folder"`
	Image         string           `json:"image"`
	HeroImage     string           `json:"hero_image"`
	Background    BackgroundRecord `json:"background"`
	HasCover      bool             `json:"has_cover"`
	OriginalName  string           `json:"original_name"`
	DetectedAppID string           `json:"detected_appid"`
	Favorite      bool             `json:"favorite"`
	LastPlayed    string           `json:"last_played"`
	Metadata      map[string]any   `json:"metadata"`
	Review        MetadataReview   `json:"review"`
}
type Library struct {
	Updated string `json:"updated"`
	Games   []Game `json:"games"`
	Note    string `json:"note"`
	Theme   string `json:"theme"`
	DataDir string `json:"data_dir"`
}
type Preferences struct {
	Theme     string            `json:"theme"`
	Favorites map[string]bool   `json:"favorites"`
	Played    map[string]string `json:"played"`
	Scan      *ScanSettings     `json:"scan,omitempty"`
}
type App struct {
	ctx               context.Context
	mu                sync.Mutex
	dir               string
	library           Library
	meta              map[string]map[string]any
	prefs             Preferences
	loadError         error
	backgrounds       map[string]BackgroundRecord
	integrations      IntegrationConfig
	integrationError  error
	artworkGames      map[string][]ArtworkGame
	artworkCandidates map[string]map[string]Candidate
}

func NewApp() *App {
	executable, _ := os.Executable()
	choices := []string{filepath.Join(filepath.Dir(executable), "data"), "data", filepath.Join("desktop", "data")}
	dir := choices[0]
	for _, p := range choices {
		if _, e := os.Stat(filepath.Join(p, "games.json")); e == nil {
			dir = p
			break
		}
	}
	dir, _ = filepath.Abs(dir)
	return newAppWithDataDir(dir)
}

func newAppWithDataDir(dir string) *App {
	a := &App{dir: dir, meta: map[string]map[string]any{}, prefs: Preferences{Theme: "system", Favorites: map[string]bool{}, Played: map[string]string{}}}
	a.backgrounds = map[string]BackgroundRecord{}
	if e := readJSON(filepath.Join(dir, "integrations.json"), &a.integrations); e != nil && !os.IsNotExist(e) {
		a.integrationError = e
	}
	if e := readJSON(filepath.Join(dir, "games.json"), &a.library); e != nil && !os.IsNotExist(e) {
		a.loadError = e
	}
	if e := readJSON(filepath.Join(dir, "backgrounds.json"), &a.backgrounds); e != nil && !os.IsNotExist(e) {
		a.loadError = e
	}
	if e := readJSON(filepath.Join(dir, "metadata.json"), &a.meta); e != nil && !os.IsNotExist(e) {
		a.loadError = e
	}
	if e := readJSON(filepath.Join(dir, "preferences.json"), &a.prefs); e != nil && !os.IsNotExist(e) {
		a.loadError = e
	}
	if a.prefs.Favorites == nil {
		a.prefs.Favorites = map[string]bool{}
	}
	if a.prefs.Played == nil {
		a.prefs.Played = map[string]string{}
	}
	return a
}
func readJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	tmp := path + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func (a *App) startup(ctx context.Context) { a.ctx = ctx }
func (a *App) game(id string) (Game, error) {
	for _, g := range a.library.Games {
		if g.ID == id {
			return g, nil
		}
	}
	return Game{}, errors.New("游戏不存在，请重新选择")
}
func str(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func (a *App) imagePath(g Game) string {
	image := str(a.meta[g.ID], "image")
	if image == "" {
		image = g.Image
	}
	if image != "" {
		if !filepath.IsAbs(image) {
			image = filepath.Join(a.dir, filepath.FromSlash(image))
		}
		if st, e := os.Stat(image); e == nil && !st.IsDir() {
			return image
		}
	}
	return filepath.Join(a.dir, "icons", g.ID+".png")
}
func (a *App) heroPath(g Game) string {
	if rel := a.backgrounds[g.ID].Image; rel != "" && strings.HasPrefix(rel, "backgrounds/") && !strings.Contains(rel, "..") {
		if p := filepath.Join(a.dir, filepath.FromSlash(rel)); exists(p) {
			return p
		}
	}
	appid := detectAppID(g)
	if appid == "" {
		appid = parseAppID(str(a.meta[g.ID], "source_url"))
	}
	if appid == "" {
		return ""
	}
	cacheDir := filepath.Join(a.dir, "backgrounds")
	cached := filepath.Join(cacheDir, g.ID+"-"+appid+".jpg")
	if exists(cached) {
		return cached
	}
	// Use Steam's real panoramic art, never its library_hero_blur preview.
	files, _ := filepath.Glob(filepath.Join(`D:\Steam\appcache\librarycache`, appid, "*", "library_hero.jpg"))
	for _, p := range files {
		if st, e := os.Stat(p); e == nil && st.Size() > 20000 {
			b, e := os.ReadFile(p)
			if e != nil {
				continue
			}
			if os.MkdirAll(cacheDir, 0755) == nil && os.WriteFile(cached, b, 0600) == nil {
				return cached
			}
			return p
		}
	}
	return ""
}
func (a *App) GetLibrary() (Library, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loadError != nil {
		return Library{}, fmt.Errorf("读取游戏资料失败：%w", a.loadError)
	}
	out := a.library
	out.Games = make([]Game, 0, len(a.library.Games))
	out.Theme = a.prefs.Theme
	out.DataDir = a.dir
	for _, g := range a.library.Games {
		g.OriginalName = g.Name
		metadataBytes, _ := json.Marshal(a.meta[g.ID])
		_ = json.Unmarshal(metadataBytes, &g.Metadata)
		g.Review = metadataReview(g.Metadata)
		if n := str(g.Metadata, "name"); n != "" {
			g.Name = n
		}
		if exe := str(g.Metadata, "preferred_exe"); exe != "" {
			g.Exe = exe
		}
		p := a.imagePath(g)
		if hero := a.heroPath(g); hero != "" {
			if st, e := os.Stat(hero); e == nil {
				g.HeroImage = fmt.Sprintf("/library-image/%s?kind=hero&v=%d", g.ID, st.ModTime().UnixNano())
			}
		}
		g.Background = a.backgrounds[g.ID]
		g.HasCover = !strings.Contains(p, string(filepath.Separator)+"icons"+string(filepath.Separator))
		st, e := os.Stat(p)
		if e == nil {
			g.Image = fmt.Sprintf("/library-image/%s?v=%d", g.ID, st.ModTime().UnixNano())
		} else {
			g.Image = ""
		}
		g.DetectedAppID = detectAppID(g)
		g.Favorite = a.prefs.Favorites[g.ID]
		g.LastPlayed = a.prefs.Played[g.ID]
		out.Games = append(out.Games, g)
	}
	return out, nil
}
func (a *App) serveImage(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/library-image/") {
		http.NotFound(w, r)
		return
	}
	a.mu.Lock()
	g, e := a.game(strings.TrimPrefix(r.URL.Path, "/library-image/"))
	p := a.imagePath(g)
	if r.URL.Query().Get("kind") == "hero" {
		p = a.heroPath(g)
	}
	a.mu.Unlock()
	if e != nil || p == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, p)
}
func (a *App) SetTheme(theme string) error {
	if theme != "light" && theme != "dark" && theme != "system" {
		return errors.New("无效主题")
	}
	a.mu.Lock()
	a.prefs.Theme = theme
	e := writeJSON(filepath.Join(a.dir, "preferences.json"), a.prefs)
	a.mu.Unlock()
	if e != nil {
		return e
	}
	if a.ctx != nil {
		switch theme {
		case "light":
			wr.WindowSetLightTheme(a.ctx)
		case "dark":
			wr.WindowSetDarkTheme(a.ctx)
		default:
			wr.WindowSetSystemDefaultTheme(a.ctx)
		}
	}
	return nil
}
func (a *App) ToggleFavorite(id string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, e := a.game(id); e != nil {
		return false, e
	}
	v := !a.prefs.Favorites[id]
	a.prefs.Favorites[id] = v
	return v, writeJSON(filepath.Join(a.dir, "preferences.json"), a.prefs)
}
func (a *App) LaunchGame(id, exe string) error {
	a.mu.Lock()
	g, e := a.game(id)
	preferred := str(a.meta[id], "preferred_exe")
	a.mu.Unlock()
	if e != nil {
		return e
	}
	if exe == "" {
		exe = preferred
	}
	if exe == "" {
		exe = g.Exe
	}
	allowed := exe == g.Exe
	for _, p := range g.Executables {
		if p == exe {
			allowed = true
		}
	}
	if !allowed {
		return errors.New("启动程序不属于所选游戏")
	}
	if _, e = os.Stat(exe); e != nil {
		return errors.New("启动文件不存在，请重新扫描")
	}
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	if e = cmd.Start(); e != nil {
		return fmt.Errorf("启动失败：%w", e)
	}
	go cmd.Wait()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prefs.Played[id] = time.Now().Format(time.RFC3339)
	return writeJSON(filepath.Join(a.dir, "preferences.json"), a.prefs)
}
func (a *App) OpenFolder(id string) error {
	a.mu.Lock()
	g, e := a.game(id)
	preferred := str(a.meta[id], "preferred_exe")
	a.mu.Unlock()
	if e != nil {
		return e
	}
	if preferred != "" {
		g.Exe = preferred
	}
	if _, e = os.Stat(g.Exe); e != nil {
		return errors.New("启动文件不存在，请重新扫描")
	}
	return revealFile(g.Exe)
}
func (a *App) SaveMetadata(id, name, exe string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 200 {
		return errors.New("名称需为 1–200 个字符")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	g, e := a.game(id)
	if e != nil {
		return e
	}
	if exe != "" {
		valid := exe == g.Exe
		for _, p := range g.Executables {
			if p == exe {
				valid = true
			}
		}
		if !valid {
			return errors.New("请选择该游戏已有的启动文件")
		}
	}
	if a.meta[id] == nil {
		a.meta[id] = map[string]any{}
	}
	a.meta[id]["name"] = name
	a.meta[id]["manual"] = true
	if exe != "" {
		a.meta[id]["preferred_exe"] = exe
	}
	return writeJSON(filepath.Join(a.dir, "metadata.json"), a.meta)
}
func (a *App) ResetMetadata(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, e := a.game(id); e != nil {
		return e
	}
	delete(a.meta, id)
	return writeJSON(filepath.Join(a.dir, "metadata.json"), a.meta)
}
func (a *App) ImportCover(id string) (bool, error) {
	if a.ctx == nil {
		return false, errors.New("上传封面请在桌面客户端中操作")
	}
	path, e := wr.OpenFileDialog(a.ctx, wr.OpenDialogOptions{Title: "选择游戏封面", Filters: []wr.FileFilter{{DisplayName: "图片 (PNG / JPG / WebP)", Pattern: "*.png;*.jpg;*.jpeg;*.webp"}}})
	if e != nil || path == "" {
		return false, e
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return false, e
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, e = a.game(id); e != nil {
		return false, e
	}
	image, e := a.saveImage(id, b)
	if e != nil {
		return false, e
	}
	if a.meta[id] == nil {
		a.meta[id] = map[string]any{}
	}
	a.meta[id]["image"] = image
	a.meta[id]["manual"] = true
	a.meta[id]["provider"] = "手动封面"
	a.meta[id]["cover_provider"] = "本地上传"
	a.meta[id]["cover_source_url"] = ""
	a.meta[id]["cover_author"] = ""
	return true, writeJSON(filepath.Join(a.dir, "metadata.json"), a.meta)
}
