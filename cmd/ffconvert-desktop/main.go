// Command ffconvert-desktop is the Linux desktop frontend of ff-convert,
// built with Wails v2. Files and folders are picked with the native system
// dialogs; conversions run in a queue through the shared core.
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed icon.png
var icon []byte

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:     "ff-convert",
		Width:     940,
		Height:    720,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Linux: &linux.Options{
			// Stabilizes WM_CLASS to "ffconvert" so the window matches
			// StartupWMClass in packaging/ffconvert.desktop.
			ProgramName: "ffconvert",
			// Window icon (X11/KDE); the launcher icon comes from the
			// hicolor theme + .desktop entry installed by the package.
			Icon: icon,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
