package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func ensureGameIcons(dir string, games []Game) {
	type iconJob struct {
		Exe    string `json:"exe"`
		Target string `json:"target"`
	}
	jobs := []iconJob{}
	for _, g := range games {
		if g.ID == "" || filepath.Base(g.ID) != g.ID {
			continue
		}
		target := filepath.Join(dir, "icons", g.ID+".png")
		if exists(target) {
			continue
		}
		f, err := os.Open(g.Exe)
		if err != nil {
			continue
		}
		signature := make([]byte, 2)
		_, err = io.ReadFull(f, signature)
		f.Close()
		if err != nil || string(signature) != "MZ" {
			continue
		}
		jobs = append(jobs, iconJob{g.Exe, target})
	}
	if len(jobs) == 0 {
		return
	}
	if os.MkdirAll(filepath.Join(dir, "icons"), 0755) != nil {
		return
	}
	payload, _ := json.Marshal(jobs)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	script := `[Console]::InputEncoding = [System.Text.UTF8Encoding]::new()
 Add-Type -AssemblyName System.Drawing
 $jobs=[Console]::In.ReadToEnd()|ConvertFrom-Json
 foreach($job in $jobs){
  try{
   $icon=[System.Drawing.Icon]::ExtractAssociatedIcon($job.exe)
   if($icon){$bitmap=$icon.ToBitmap();try{$bitmap.Save($job.target,[System.Drawing.Imaging.ImageFormat]::Png)}finally{$bitmap.Dispose();$icon.Dispose()}}
  }catch{}
 }`
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.Stdin = bytes.NewReader(payload)
	_ = cmd.Run()
}
