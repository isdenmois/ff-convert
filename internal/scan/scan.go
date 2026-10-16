// Package scan collects convertible movie files from a file or folder path.
package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// VideoExts are the file extensions treated as movie files.
var VideoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true,
	".mov": true, ".ts": true, ".webm": true, ".wmv": true, ".flv": true,
}

// EnvOutputDir is the environment variable overriding the output folder.
const EnvOutputDir = "FF_CONVERT_OUTPUT_DIR"

// ConvertedDir is the default output folder (relative to the working
// directory) where converted files are written.
const ConvertedDir = "converted"

// ResolveOutputDir resolves the folder converted files are written to: the
// value of FF_CONVERT_OUTPUT_DIR when set, then the configured output_dir,
// then the caller-provided fallback (frontends pick their own: "converted"
// in the working directory for the TUI and web server, a Videos folder for
// the desktop app).
func ResolveOutputDir(configured, fallback string) string {
	if dir := os.Getenv(EnvOutputDir); dir != "" {
		return dir
	}
	if configured != "" {
		return configured
	}
	return fallback
}

// Skipped describes a file that was skipped because its output already exists.
type Skipped struct {
	Path   string `json:"path"`
	Output string `json:"output"`
}

// Result of a scan.
type Result struct {
	Files   []string  `json:"files"`
	Skipped []Skipped `json:"skipped"`
	// SourceFolder is the source's own folder name when the source is a
	// folder ("" for a single-file source): its outputs are written into a
	// subfolder of that name inside the output folder.
	SourceFolder string `json:"source_folder,omitempty"`
}

// IsVideoFile reports whether the path has a movie extension.
func IsVideoFile(path string) bool { return VideoExts[strings.ToLower(filepath.Ext(path))] }

// OutputPathIn returns the converted output path for a source file in the
// given folder: <dir>/<base-without-ext>.mkv
func OutputPathIn(dir, file string) string {
	base := filepath.Base(file)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(dir, base+".mkv")
}

// Scan resolves a source path (single file or folder) to the list of movie
// files to convert. Folder scans are top-level only, sorted naturally
// (case-insensitive, number-aware), and skip files whose output already exists
// in outDir — either <outDir>/<stem>.mkv or, for folder sources,
// <outDir>/<source folder>/<stem>.mkv.
func Scan(source, outDir string) (Result, error) {
	info, err := os.Stat(source)
	if err != nil {
		return Result{}, err
	}
	var res Result
	if !info.IsDir() {
		if !IsVideoFile(source) {
			return Result{}, fmt.Errorf("%s: not a video file", source)
		}
		if out := OutputPathIn(outDir, source); fileExists(out) {
			return Result{Skipped: []Skipped{{Path: source, Output: out}}}, nil
		}
		return Result{Files: []string{source}}, nil
	}

	entries, err := os.ReadDir(source)
	if err != nil {
		return Result{}, err
	}
	folder := filepath.Base(filepath.Clean(source))
	res.SourceFolder = folder
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(source, e.Name())
		if !IsVideoFile(p) {
			continue
		}
		out := OutputPathIn(outDir, p)
		// A source file already sitting in the output folder would map onto
		// itself; skip it rather than "convert" it in place.
		if out == p {
			res.Skipped = append(res.Skipped, Skipped{Path: p, Output: out})
			continue
		}
		if fileExists(out) {
			res.Skipped = append(res.Skipped, Skipped{Path: p, Output: out})
			continue
		}
		// Folder sources write into <outDir>/<source folder>/ (see
		// SourceFolder): an output from an earlier run must skip too.
		if nout := OutputPathIn(filepath.Join(outDir, folder), p); fileExists(nout) {
			res.Skipped = append(res.Skipped, Skipped{Path: p, Output: nout})
			continue
		}
		res.Files = append(res.Files, p)
	}
	Sort(res.Files)
	return res, nil
}

// Sort sorts file paths naturally: case-insensitive, with embedded numbers
// compared numerically ("E2" < "E10").
func Sort(files []string) {
	sort.Slice(files, func(i, j int) bool {
		return naturalLess(files[i], files[j])
	})
}

func naturalLess(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]
		ad, bd := isDigit(ca), isDigit(cb)
		switch {
		case ad && bd:
			// Compare the full number runs.
			aj, bj := ai, bi
			for aj < len(a) && isDigit(a[aj]) {
				aj++
			}
			for bj < len(b) && isDigit(b[bj]) {
				bj++
			}
			na, nb := strings.TrimLeft(a[ai:aj], "0"), strings.TrimLeft(b[bi:bj], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			ai, bi = aj, bj
		default:
			xa, xb := lower(ca), lower(cb)
			if xa != xb {
				return xa < xb
			}
			ai++
			bi++
		}
	}
	return len(a)-ai < len(b)-bi
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
