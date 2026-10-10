package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOutputPath(t *testing.T) {
	cases := map[string]string{
		"/movies/Movie.mkv":  "/movies/converted/Movie.mkv",
		"/movies/Movie.mp4":  "/movies/converted/Movie.mkv",
		"/a b/Show.S01E01.avi": "/a b/converted/Show.S01E01.mkv",
	}
	for in, want := range cases {
		if got := OutputPath(in); got != want {
			t.Errorf("OutputPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScanFolder(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"b ep10.mkv", "B ep2.mkv", "a movie.mp4", "notes.txt", ".hidden.mkv",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Already converted: b ep10.
	if err := os.Mkdir(filepath.Join(dir, ConvertedDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConvertedDir, "b ep10.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Scan(dir)
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
}

func TestScanSingleFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "single.mkv")
	os.WriteFile(p, []byte("x"), 0o644)

	res, err := Scan(p)
	if err != nil || len(res.Files) != 1 || res.Files[0] != p {
		t.Fatalf("Scan(file) = %+v, %v", res, err)
	}

	if _, err := Scan(filepath.Join(dir, "missing.mkv")); err == nil {
		t.Fatal("missing file must error")
	}
	if _, err := Scan(filepath.Join(dir, "no.txt")); err == nil {
		if err := os.WriteFile(filepath.Join(dir, "no.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Scan(filepath.Join(dir, "no.txt")); err == nil {
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
