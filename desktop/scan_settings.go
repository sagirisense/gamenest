package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ScanSettings struct {
	Roots    []string `json:"roots"`
	Excluded []string `json:"excluded"`
}

func (a *App) GetScanSettings() ScanSettings {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.prefs.Scan == nil {
		return ScanSettings{Roots: []string{`D:\`}, Excluded: []string{}}
	}
	return ScanSettings{Roots: append([]string{}, a.prefs.Scan.Roots...), Excluded: append([]string{}, a.prefs.Scan.Excluded...)}
}

func pathWithin(path, root string) bool {
	path, root = strings.ToLower(filepath.Clean(path)), strings.ToLower(filepath.Clean(root))
	return path == root || strings.HasPrefix(path, strings.TrimRight(root, string(filepath.Separator))+string(filepath.Separator))
}

func excludedPath(path string, excluded []string) bool {
	for _, root := range excluded {
		if pathWithin(path, root) {
			return true
		}
	}
	return false
}

func normalizeScanPaths(paths []string) ([]string, error) {
	if len(paths) > 64 {
		return nil, errors.New("每类最多添加 64 个位置")
	}
	result := []string{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) || strings.ContainsAny(path, "*?\x00\r\n") || strings.HasPrefix(path, `\\.\`) || strings.HasPrefix(path, `\\?\`) {
			return nil, fmt.Errorf("请填写完整的盘符或文件夹路径：%s", path)
		}
		path = filepath.Clean(path)
		if excludedPath(path, result) {
			continue
		}
		kept := result[:0]
		for _, old := range result {
			if !pathWithin(old, path) {
				kept = append(kept, old)
			}
		}
		result = append(kept, path)
	}
	return result, nil
}

func (a *App) SaveScanSettings(raw string) (ScanSettings, error) {
	var settings ScanSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return ScanSettings{}, errors.New("扫描设置格式无效")
	}
	var err error
	if settings.Roots, err = normalizeScanPaths(settings.Roots); err != nil {
		return ScanSettings{}, err
	}
	if settings.Excluded, err = normalizeScanPaths(settings.Excluded); err != nil {
		return ScanSettings{}, err
	}
	if len(settings.Roots) == 0 {
		return ScanSettings{}, errors.New("请至少添加一个扫描位置")
	}
	for _, root := range settings.Roots {
		if excludedPath(root, settings.Excluded) {
			return ScanSettings{}, fmt.Errorf("扫描位置已被排除：%s", root)
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	prefs := a.prefs
	prefs.Scan = &settings
	if err = writeJSON(filepath.Join(a.dir, "preferences.json"), prefs); err != nil {
		return ScanSettings{}, err
	}
	a.prefs = prefs
	return settings, nil
}

func (a *App) PickScanFolder() (string, error) {
	if a.ctx == nil {
		return "", errors.New("请在桌面客户端中选择文件夹，也可以直接填写完整路径")
	}
	return wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{Title: "选择盘符或文件夹"})
}

func validateScanRoots(settings ScanSettings) error {
	for _, root := range settings.Roots {
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("扫描位置不可用：%s。请检查盘符是否已连接，或在设置中修改；现有游戏清单已保留", root)
		}
	}
	return nil
}
