package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractProgramIconFallback(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ensureGameIcons(dir, []Game{{ID: "sample", Exe: exe}})
	f, err := os.Open(filepath.Join(dir, "icons", "sample.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		t.Fatal(cfg, err)
	}
}
