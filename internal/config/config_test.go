package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadFrom(filepath.Join(dir, "nope.yaml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.Exists {
		t.Fatal("Exists must be false for missing file")
	}
	if _, ok := cfg.Preset("av1"); !ok {
		t.Fatal("built-in av1 preset missing")
	}
	names := cfg.PresetNames()
	want := []string{"copy", "av1", "hevc", "hevc-nvenc", "hevc-vaapi", "av1-nvenc"}
	if len(names) != len(want) {
		t.Fatalf("PresetNames() = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("PresetNames()[%d] = %q, want %q", i, names[i], want[i])
		}
	}
	if cfg.Bitrate() != "128k" {
		t.Fatalf("Bitrate() = %q, want 128k", cfg.Bitrate())
	}
	if got := cfg.DefaultPresetName(); got != "copy" {
		t.Fatalf("DefaultPresetName() = %q, want copy", got)
	}
	if !cfg.NormalizeDefault() {
		t.Fatal("NormalizeDefault() must be true when unset")
	}
}

func TestLoadFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yml := `
default_preset: av1-fast
presets:
  av1-fast:
    codec: libsvtav1
    args: ["-preset", "8", "-crf", "30"]
    pix_fmt: yuv420p
  hevc: # overrides built-in
    codec: libx265
    args: ["-preset", "slow", "-crf", "20"]
audio:
  bitrate: 160k
  normalize: false
attachments: omit
output_dir: /mnt/media/converted
categories: ["TV Shows", "movies/2024", "TV Shows", "..."]
`
	if err := os.WriteFile(path, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if !cfg.Exists || cfg.Path != path {
		t.Fatalf("Exists=%v Path=%q", cfg.Exists, cfg.Path)
	}
	if cfg.DefaultPreset != "av1-fast" {
		t.Fatalf("DefaultPreset = %q", cfg.DefaultPreset)
	}
	p, ok := cfg.Preset("hevc")
	if !ok || p.Codec != "libx265" || len(p.Args) != 4 || p.Args[1] != "slow" {
		t.Fatalf("user override of built-in hevc not applied: %+v", p)
	}
	if cfg.Bitrate() != "160k" {
		t.Fatalf("Bitrate = %q", cfg.Bitrate())
	}
	if cfg.Audio.Normalize == nil || *cfg.Audio.Normalize {
		t.Fatal("normalize should be explicitly false")
	}
	if cfg.Attachments == nil || *cfg.Attachments != "omit" {
		t.Fatal("attachments should be omit")
	}
	if cfg.OutputDir != "/mnt/media/converted" {
		t.Fatalf("OutputDir = %q", cfg.OutputDir)
	}
	if len(cfg.Categories) != 2 || cfg.Categories[0] != "tv-shows" || cfg.Categories[1] != "movies-2024" {
		t.Fatalf("Categories = %v, want [tv-shows movies-2024]", cfg.Categories)
	}
	if !cfg.HasCategory("tv-shows") || cfg.HasCategory("TV Shows") || cfg.HasCategory("") {
		t.Fatal("HasCategory must match only the cleaned names")
	}
	if got := cfg.DefaultPresetName(); got != "av1-fast" {
		t.Fatalf("DefaultPresetName() = %q, want av1-fast", got)
	}
	if cfg.NormalizeDefault() {
		t.Fatal("NormalizeDefault() must honor explicit false")
	}
	names := cfg.PresetNames()
	// copy, av1, hevc, hevc-nvenc, hevc-vaapi, av1-nvenc, av1-fast
	if len(names) != 7 || names[6] != "av1-fast" {
		t.Fatalf("PresetNames = %v", names)
	}
}

func TestCategorySlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"TV Shows", "tv-shows"},
		{"Movies/2024", "movies-2024"},
		{"..", ""},
		{"  Action  ", "action"},
		{"Sci--Fi", "sci-fi"},
		{"4K HDR!", "4k-hdr"},
		{"", ""},
	}
	for _, c := range cases {
		if got := slugifyCategory(c.in); got != c.want {
			t.Errorf("slugifyCategory(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := cleanCategories([]string{"", "..", "Series", "series", "Movies"}); len(got) != 2 || got[0] != "series" || got[1] != "movies" {
		t.Errorf("cleanCategories = %v, want [series movies]", got)
	}
	if got := cleanCategories(nil); got != nil {
		t.Errorf("cleanCategories(nil) = %v, want nil", got)
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")

	if err := os.WriteFile(path, []byte("default_preset: nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("unknown default_preset must error")
	}

	if err := os.WriteFile(path, []byte("attachments: delete\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("invalid attachments value must error")
	}

	if err := os.WriteFile(path, []byte("presets: [not, a, map]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("malformed yaml must error")
	}
}
