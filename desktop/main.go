package main

import (
	"embed"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	if len(os.Args) > 1 && os.Args[1] == "--preview" {
		site, _ := fs.Sub(assets, "frontend/dist")
		servePreview(app, site)
		return
	}
	err := wails.Run(&options.App{Title: "游栖 · GameNest", Width: 1440, Height: 960, MinWidth: 980, MinHeight: 700, Frameless: true, BackgroundColour: &options.RGBA{R: 248, G: 248, B: 251, A: 255}, AssetServer: &assetserver.Options{Assets: assets, Handler: http.HandlerFunc(app.serveImage)}, OnStartup: app.startup, Bind: []interface{}{app}, Windows: &windows.Options{Theme: windows.SystemDefault}, SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "78cfa68c-623c-44ee-98f9-65eaf970a212", OnSecondInstanceLaunch: func(options.SecondInstanceData) {
		if app.ctx != nil {
			wr.WindowUnminimise(app.ctx)
			wr.WindowShow(app.ctx)
		}
	}}})
	if err != nil {
		log.Fatal(err)
	}
}
