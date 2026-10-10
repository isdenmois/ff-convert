// Package webui embeds the built SPA (web/dist).
package webui

import "embed"

//go:embed all:dist
var dist embed.FS

// Dist returns the embedded SPA filesystem.
func Dist() embed.FS { return dist }
