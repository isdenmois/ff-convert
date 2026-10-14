// Package config loads ff-convert preset configuration from a YAML file.
//
// Lookup order: $FF_CONVERT_CONFIG, then ~/.config/ff-convert/config.yaml.
// When no file exists the built-in presets are used and every question is
// asked interactively.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// EnvConfigPath is the environment variable overriding the config location.
const EnvConfigPath = "FF_CONVERT_CONFIG"

// DefaultPath returns the default config file location.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ff-convert", "config.yaml"), nil
}

// Preset describes how the video stream is encoded. A preset with Codec
// "copy" remuxes the video untouched.
type Preset struct {
	Codec  string   `yaml:"codec"`
	Args   []string `yaml:"args,omitempty"`
	PixFmt string   `yaml:"pix_fmt,omitempty"`
	// VAAPIDevice, when set, adds "-vaapi_device <dev>" before the input
	// (used by VAAPI presets).
	VAAPIDevice string `yaml:"vaapi_device,omitempty"`
	// VF, when set, adds "-vf <filter>" for the video stream
	// (e.g. "format=nv12|vaapi,hwupload" for VAAPI).
	VF string `yaml:"vf,omitempty"`
}

// IsCopy reports whether the preset copies the video stream.
func (p Preset) IsCopy() bool { return p.Codec == "" || p.Codec == "copy" }

// Audio carries optional audio defaults. When a value is set the
// corresponding interactive question is skipped.
type Audio struct {
	Bitrate string `yaml:"bitrate,omitempty"`
	// Normalize is a pointer so that an explicit `false` is distinguished
	// from "unset". When set, the loudnorm question is skipped.
	Normalize *bool `yaml:"normalize,omitempty"`
}

// Config is the parsed configuration file.
type Config struct {
	// Path is the file the config was loaded from ("" for built-ins).
	Path string
	// Exists reports whether a config file was found.
	Exists bool

	DefaultPreset string            `yaml:"default_preset,omitempty"`
	Presets       map[string]Preset `yaml:"presets,omitempty"`
	Audio         *Audio            `yaml:"audio,omitempty"`
	// Attachments, when set ("copy" or "omit"), skips the attachment question.
	Attachments *string `yaml:"attachments,omitempty"`
	// Categories are the output-subfolder choices the web UI offers on the
	// answers modal; entries are slugified at load. Absent/empty ⇒ the web UI
	// offers no choice and outputs stay flat.
	Categories []string `yaml:"categories,omitempty"`
	// OutputDir, when set, is the default folder converted files are written
	// to (an absolute path is recommended). FF_CONVERT_OUTPUT_DIR overrides
	// it; frontends apply their own fallback when both are unset.
	OutputDir string `yaml:"output_dir,omitempty"`

	builtins map[string]Preset
}

// Builtins returns the built-in video presets.
func Builtins() map[string]Preset {
	return map[string]Preset{
		"copy":       {Codec: "copy"},
		"av1":        {Codec: "libsvtav1", Args: []string{"-preset", "6", "-crf", "28"}, PixFmt: "yuv420p"},
		"hevc":       {Codec: "libx265", Args: []string{"-preset", "medium", "-crf", "23"}, PixFmt: "yuv420p"},
		"hevc-nvenc": {Codec: "hevc_nvenc", Args: []string{"-preset", "p5", "-cq", "26"}},
		"hevc-vaapi": {Codec: "hevc_vaapi", Args: []string{"-qp", "24"}, VAAPIDevice: "/dev/dri/renderD128", VF: "format=nv12|vaapi,hwupload"},
		// av1-nvenc needs an RTX 40+ GPU (AV1 NVENC); 10-bit keeps HDR
		// sources intact via ffmpeg's color-metadata passthrough.
		"av1-nvenc": {Codec: "av1_nvenc", Args: []string{"-preset", "p7", "-tune", "hq", "-cq", "28"}, PixFmt: "yuv420p10le"},
	}
}

// builtinOrder is the display order of built-in preset names.
var builtinOrder = []string{"copy", "av1", "hevc", "hevc-nvenc", "hevc-vaapi", "av1-nvenc"}

// Load reads the configuration. A missing file is not an error: the returned
// config contains the built-in presets with Exists=false.
func Load() (*Config, error) {
	path := os.Getenv(EnvConfigPath)
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, fmt.Errorf("config path: %w", err)
		}
	}
	return LoadFrom(path)
}

// LoadFrom reads the configuration from a specific path.
func LoadFrom(path string) (*Config, error) {
	cfg := &Config{builtins: Builtins()}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.Categories = cleanCategories(cfg.Categories)
	cfg.Path = path
	cfg.Exists = true
	if cfg.DefaultPreset != "" {
		if _, ok := cfg.Preset(cfg.DefaultPreset); !ok {
			return nil, fmt.Errorf("config %s: default_preset %q is not a defined or built-in preset", path, cfg.DefaultPreset)
		}
	}
	if cfg.Audio != nil && cfg.Audio.Bitrate == "" {
		cfg.Audio.Bitrate = "128k"
	}
	if a := cfg.Attachments; a != nil && *a != "copy" && *a != "omit" {
		return nil, fmt.Errorf("config %s: attachments must be \"copy\" or \"omit\", got %q", path, *a)
	}
	return cfg, nil
}

// Preset resolves a preset by name: user-defined presets override built-ins
// with the same name.
func (c *Config) Preset(name string) (Preset, bool) {
	if p, ok := c.Presets[name]; ok {
		return p, true
	}
	if c.builtins == nil {
		c.builtins = Builtins()
	}
	p, ok := c.builtins[name]
	return p, ok
}

// PresetNames returns all selectable preset names, built-ins first (in their
// canonical order) followed by user-defined presets in sorted order.
// Built-in names shadowed by user presets are listed only once.
func (c *Config) PresetNames() []string {
	seen := make(map[string]bool, len(builtinOrder)+len(c.Presets))
	names := make([]string, 0, len(builtinOrder)+len(c.Presets))
	for _, n := range builtinOrder {
		seen[n] = true
		names = append(names, n)
	}
	custom := make([]string, 0, len(c.Presets))
	for n := range c.Presets {
		if !seen[n] {
			custom = append(custom, n)
		}
	}
	sort.Strings(custom)
	return append(names, custom...)
}

// Bitrate returns the audio bitrate to use (default 128k).
func (c *Config) Bitrate() string {
	if c.Audio != nil && c.Audio.Bitrate != "" {
		return c.Audio.Bitrate
	}
	return "128k"
}

// DefaultPresetName returns the configured default preset, or "copy" when
// unset. It is the preselected codec for every queue item without an
// explicit choice.
func (c *Config) DefaultPresetName() string {
	if c.DefaultPreset != "" {
		return c.DefaultPreset
	}
	return "copy"
}

// NormalizeDefault reports whether loudness normalization is enabled when the
// config does not decide (on by default).
func (c *Config) NormalizeDefault() bool {
	return c.Audio == nil || c.Audio.Normalize == nil || *c.Audio.Normalize
}

// HasCategory reports whether name is one of the configured categories.
func (c *Config) HasCategory(name string) bool {
	return slices.Contains(c.Categories, name)
}

// slugifyCategory turns a configured category label into a folder name:
// lowercased, unicode letters and digits kept, every other run of
// characters collapses to one "-", leading/trailing "-" trimmed.
// "TV Shows" → "tv-shows"; "Movies/2024" → "movies-2024"; ".." → "".
func slugifyCategory(s string) string {
	var b strings.Builder
	sep := true // nothing written yet: no leading "-"
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			sep = false
			continue
		}
		if !sep {
			b.WriteByte('-')
			sep = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// cleanCategories slugifies the labels, drops entries that collapse to ""
// and de-duplicates keeping the first occurrence.
func cleanCategories(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, c := range in {
		c = slugifyCategory(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}
