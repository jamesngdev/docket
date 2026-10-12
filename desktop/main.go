// Command docket-desktop is the macOS desktop client of docket (Wails v2 + React).
// The same binary is also the docket CLI: with arguments it runs `cli.Run`, so the
// daemon and agents launched from the .app call back into this very executable.
package main

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"docket/internal/cli"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if args := cliArgs(); len(args) > 0 {
		if err := cli.Run(args); err != nil {
			fmt.Fprintln(os.Stderr, "docket:", err)
			os.Exit(1)
		}
		return
	}
	fixPath() // Finder-launched apps get a minimal PATH; agents need claude/gh/tmux/git
	app := NewApp()
	err := wails.Run(&options.App{
		Title:            "Docket",
		Width:            1360,
		Height:           860,
		MinWidth:         960,
		MinHeight:        600,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 14, G: 15, B: 18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
			About:                &mac.AboutInfo{Title: "Docket", Message: "Docs in, pull requests out."},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "docket desktop:", err)
		os.Exit(1)
	}
}

// cliArgs returns CLI arguments, ignoring macOS launch noise like -psn_0_12345.
func cliArgs() []string {
	var out []string
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "-psn_") {
			continue
		}
		out = append(out, a)
	}
	return out
}
