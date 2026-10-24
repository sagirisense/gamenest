package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var unwanted = regexp.MustCompile(`(?i)crash|unins|redist|setup|install|update|helper|notification|config|trainer|修改器|补丁|cheat|startup|opensave|filecheck|reset`)

func exists(path string) bool { _, e := os.Stat(path); return e == nil }
func stableID(key string) string {
	h := sha256.Sum256([]byte(strings.ToLower(key)))
	return hex.EncodeToString(h[:])[:16]
}
func isEngineGame(exe string) bool {
	folder := filepath.Dir(exe)
	stem := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	if exists(filepath.Join(folder, stem+"_Data")) {
		return true
	}
	if exists(filepath.Join(folder, "renpy")) && exists(filepath.Join(folder, "game")) {
		return true
	}
	if strings.EqualFold(stem, "game") || strings.HasPrefix(strings.ToLower(stem), "game_") {
		for _, p := range []string{"www", "data", "Game.ini", "Game.rgss3a", "Game.rgss2a", "Game.rgssad"} {
			if exists(filepath.Join(folder, p)) {
				return true
			}
		}
	}
	return exists(filepath.Join(folder, stem, "Binaries"))
}
func (a *App) ScanGames() (Library, error) {
	settings := a.GetScanSettings()
	if err := validateScanRoots(settings); err != nil {
		return Library{}, err
	}
	a.mu.Lock()
	old := append([]Game{}, a.library.Games...)
	a.mu.Unlock()
	known := map[string]Game{}
	for _, g := range old {
		known[strings.ToLower(filepath.Clean(g.Folder))] = g
	}
	manifests := map[string]Game{}
	exes := []string{}
	unreadable := []string{}
	skip := map[string]bool{"node_modules": true, ".git": true, "$recycle.bin": true, "system volume information": true, "lib": true, "engine": true, "plugins": true, "_commonredist": true, "redist": true, "binaries": true, "mtool": true, "melonloader": true, "原版备份": true, "bk角色包制作器": true}
	topSkip := map[string]bool{"git": true, "java": true, "gpt-sovits-1007-cu128": true, "mysql-8.0.33-winx64": true, "microsoft vs code": true, "nvm": true, "epic games": true, "tencent": true, "netease": true, "ps2022": true, "project": true, "docker": true, "programdata": true, "program files": true, "maven": true, "hyperv": true}
	keys := regexp.MustCompile(`"(appid|name|installdir)"\s+"([^"]+)"`)
	for _, scanRoot := range settings.Roots {
		e := filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if path == scanRoot {
					return err
				}
				unreadable = append(unreadable, path)
				return nil
			}
			lower := strings.ToLower(d.Name())
			if excludedPath(path, settings.Excluded) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				// An explicitly selected folder takes precedence over built-in name filters.
				if path != scanRoot && (skip[lower] || (filepath.Dir(path) == filepath.VolumeName(path)+string(filepath.Separator) && topSkip[lower])) {
					return filepath.SkipDir
				}
				if lower == "wallpaper_engine" || lower == "live2dviewerex" || lower == "steamworks shared" || lower == "3dmark" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(lower, "appmanifest_") && strings.HasSuffix(lower, ".acf") {
				b, e := os.ReadFile(path)
				if e == nil {
					v := map[string]string{}
					for _, m := range keys.FindAllStringSubmatch(string(b), -1) {
						v[m[1]] = m[2]
					}
					if v["appid"] != "" && v["installdir"] != "" && v["appid"] != "223850" && v["appid"] != "228980" && v["appid"] != "431960" && v["appid"] != "616720" {
						folder := filepath.Join(filepath.Dir(path), "common", v["installdir"])
						if pathWithin(folder, scanRoot) && !excludedPath(folder, settings.Excluded) {
							manifests[strings.ToLower(folder)] = Game{Name: v["name"], AppID: v["appid"], Source: "Steam", Folder: folder, ID: stableID("steam" + v["appid"])}
						}
					}
				}
				return nil
			}
			if strings.HasSuffix(lower, ".exe") && !unwanted.MatchString(lower) {
				exes = append(exes, path)
			}
			return nil
		})
		if e != nil {
			return Library{}, fmt.Errorf("扫描 %s 失败，现有游戏清单已保留：%w", scanRoot, e)
		}
	}
	groups := map[string]Game{}
	for _, exe := range exes {
		folder := filepath.Dir(exe)
		key := strings.ToLower(folder)
		g, recognized := known[key]
		manifestKey := ""
		for root, m := range manifests {
			if strings.HasPrefix(strings.ToLower(exe), root+string(filepath.Separator)) {
				g = m
				if previous, ok := known[root]; ok {
					g.Image = previous.Image
				}
				manifestKey = root
				recognized = true
				break
			}
		}
		if !recognized && !isEngineGame(exe) {
			continue
		}
		if manifestKey != "" {
			key = manifestKey
		}
		if old, ok := groups[key]; ok {
			old.Executables = append(old.Executables, exe)
			groups[key] = old
			continue
		}
		if !recognized {
			stem := strings.TrimSuffix(filepath.Base(exe), ".exe")
			name := stem
			if strings.EqualFold(stem, "game") {
				name = filepath.Base(folder)
			}
			g = Game{ID: stableID(key), Name: name, Source: "本地游戏", Folder: folder}
		}
		g.Executables = []string{exe}
		g.Exe = exe
		groups[key] = g
	}
	// Keep recognised unusual engines and launchers without relaxing executable heuristics globally.
	for _, g := range old {
		key := strings.ToLower(filepath.Clean(g.Folder))
		inScope := false
		for _, root := range settings.Roots {
			inScope = inScope || pathWithin(g.Exe, root)
		}
		if _, ok := groups[key]; !ok && inScope && !excludedPath(g.Exe, settings.Excluded) && (exists(g.Exe) || excludedPath(g.Exe, unreadable)) {
			groups[key] = g
		}
	}
	result := []Game{}
	for _, g := range groups {
		sort.Slice(g.Executables, func(i, j int) bool { return len(g.Executables[i]) < len(g.Executables[j]) })
		if previous, ok := known[strings.ToLower(filepath.Clean(g.Folder))]; ok && exists(previous.Exe) && !excludedPath(previous.Exe, settings.Excluded) && excludedPath(previous.Exe, settings.Roots) {
			g.Exe = previous.Exe
		} else if len(g.Executables) > 0 {
			g.Exe = g.Executables[0]
		}
		g.Folder = filepath.Dir(g.Exe)
		result = append(result, g)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	a.mu.Lock()
	next := a.library
	next.Games = result
	next.Updated = time.Now().Format(time.RFC3339)
	next.Note = "扫描完成，手动资料已保留"
	if len(unreadable) > 0 {
		next.Note = fmt.Sprintf("扫描完成，跳过 %d 个无法读取的位置；这些位置的已有记录已保留", len(unreadable))
	}
	err := writeJSON(filepath.Join(a.dir, "games.json"), next)
	if err == nil {
		a.library = next
	}
	a.mu.Unlock()
	if err != nil {
		return Library{}, err
	}
	ensureGameIcons(a.dir, result)
	return a.GetLibrary()
}
