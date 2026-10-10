// Command ffconvert-tui is the terminal frontend of ff-convert.
//
// Usage: ffconvert-tui <file-or-folder>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/ffmpegx"
	"github.com/isden/ff-convert/internal/pool"
	"github.com/isden/ff-convert/internal/probe"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <movie-file-or-folder>\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}
	source := os.Args[1]
	if _, err := os.Stat(source); err != nil {
		fatal(err)
	}
	if err := ffmpegx.Verify(probe.FFprobePath); err != nil {
		fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}

	p := tea.NewProgram(newModel(source, cfg, pool.NewDefault()),
		tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
