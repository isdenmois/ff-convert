package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOutputPathIn(t *testing.T) {
	cases := map[string]string{
		"/movies/Movie.mkv":    "/out/Movie.mkv",
		"/movies/Movie.mp4":    "/out/Movie.mkv",
		"/a b/Show.S01E01.avi": "/out/Show.S01E01.mkv",
	}
	for in, want := range cases {
		if got := OutputPathIn("/out", in); got != want {
			t.Errorf("OutputPathIn(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveOutputDir(t *testing.T) {
	// Fallback when neither env nor config set anything.
	if got := ResolveOutputDir("", ConvertedDir); got != ConvertedDir {
		t.Errorf("ResolveOutputDir(fallback) = %q, want %q", got, ConvertedDir)
	}
	// Configured value beats the fallback.
	if got := ResolveOutputDir("/from/config", ConvertedDir); got != "/from/config" {
		t.Errorf("ResolveOutputDir(configured) = %q", got)
	}
	// FF_CONVERT_OUTPUT_DIR beats both.
	t.Setenv(EnvOutputDir, "/from/env")
	if got := ResolveOutputDir("/from/config", ConvertedDir); got != "/from/env" {
		t.Errorf("ResolveOutputDir(env) = %q, want /from/env", got)
	}
}

func TestScanFolder(t *testing.T) {
	dir := t.TempDir()
	outDir := t.TempDir()
	files := []string{
		"b ep10.mkv", "B ep2.mkv", "a movie.mp4", "notes.txt", ".hidden.mkv",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Already converted: b ep10.
	if err := os.WriteFile(filepath.Join(outDir, "b ep10.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Scan(dir, outDir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a movie.mp4", "B ep2.mkv"} // natural sort: ep2 not skipped
	if len(res.Files) != len(want) {
		t.Fatalf("Files = %v, want %v", res.Files, want)
	}
	for i := range want {
		if filepath.Base(res.Files[i]) != want[i] {
			t.Fatalf("Files = %v, want %v", res.Files, want)
		}
	}
	if len(res.Skipped) != 1 || filepath.Base(res.Skipped[0].Path) != "b ep10.mkv" {
		t.Fatalf("Skipped = %+v", res.Skipped)
	}
	if res.Skipped[0].Output != filepath.Join(outDir, "b ep10.mkv") {
		t.Errorf("Skipped output = %q", res.Skipped[0].Output)
	}
}

func TestScanSkipsSourceInOutputDir(t *testing.T) {
	// A source file whose output path is the file itself (the source folder
	// doubles as the output folder) must be skipped, not converted in place.
	dir := t.TempDir()
	p := filepath.Join(dir, "movie.mkv")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Scan(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 0 || len(res.Skipped) != 1 || res.Skipped[0].Path != p {
		t.Fatalf("Scan(source in outDir) = %+v, want one skipped", res)
	}

	res, err = Scan(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 0 || len(res.Skipped) != 1 {
		t.Fatalf("Scan(folder in outDir) = %+v, want one skipped", res)
	}
}

func TestScanSingleFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "single.mkv")
	os.WriteFile(p, []byte("x"), 0o644)

	res, err := Scan(p, t.TempDir())
	if err != nil || len(res.Files) != 1 || res.Files[0] != p {
		t.Fatalf("Scan(file) = %+v, %v", res, err)
	}

	if _, err := Scan(filepath.Join(dir, "missing.mkv"), t.TempDir()); err == nil {
		t.Fatal("missing file must error")
	}
	if _, err := Scan(filepath.Join(dir, "no.txt"), t.TempDir()); err == nil {
		if err := os.WriteFile(filepath.Join(dir, "no.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Scan(filepath.Join(dir, "no.txt"), t.TempDir()); err == nil {
			t.Fatal("non-video file must error")
		}
	}
}

func TestNaturalSort(t *testing.T) {
	in := []string{"S01E10.mkv", "S01E2.mkv", "s01e1.MKV"}
	Sort(in)
	want := []string{"s01e1.MKV", "S01E2.mkv", "S01E10.mkv"}
	for i := range want {
		if in[i] != want[i] {
			t.Fatalf("Sort = %v, want %v", in, want)
		}
	}
}
