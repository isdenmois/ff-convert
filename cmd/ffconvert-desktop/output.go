package main

import (
	"os"
	"path/filepath"
	"strings"
)

// defaultOutputDir is the desktop fallback output folder when neither
// FF_CONVERT_OUTPUT_DIR nor the config output_dir is set: <videos>/converted,
// where <videos> is the XDG videos directory (~/Videos when user-dirs.dirs
// has no entry).
func defaultOutputDir() string {
	return filepath.Join(videosDir(), "converted")
}

// videosDir returns the user's videos directory from the XDG
// user-dirs.dirs file, falling back to ~/Videos.
func videosDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "Videos"
	}
	if dir := videosDirFromUserDirs(home); dir != "" {
		return dir
	}
	return filepath.Join(home, "Videos")
}

// userDirsPath returns the location of the XDG user-dirs.dirs file.
func userDirsPath() (string, bool) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "user-dirs.dirs"), true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(home, ".config", "user-dirs.dirs"), true
}

// videosDirFromUserDirs extracts XDG_VIDEOS_DIR from the user-dirs.dirs
// content: a shell-quoted path, either absolute or $HOME-relative. Returns
// "" when absent or unusable.
func videosDirFromUserDirs(home string) string {
	path, ok := userDirsPath()
	if !ok {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return parseVideosDir(string(data), home)
}

// parseVideosDir parses XDG_VIDEOS_DIR from user-dirs.dirs content.
func parseVideosDir(content, home string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "XDG_VIDEOS_DIR=") {
			continue
		}
		val := unquoteShell(strings.TrimSpace(strings.TrimPrefix(line, "XDG_VIDEOS_DIR=")))
		switch {
		case val == "":
			return ""
		case val == "$HOME" || val == home:
			return home
		case strings.HasPrefix(val, "$HOME/"):
			return filepath.Join(home, strings.TrimPrefix(val, "$HOME/"))
		case filepath.IsAbs(val):
			return filepath.Clean(val)
		default: // relative or malformed: ignore, use the fallback
			return ""
		}
	}
	return ""
}

// unquoteShell strips surrounding double quotes and resolves backslash
// escapes, per the user-dirs.dirs quoting rules.
func unquoteShell(s string) string {
	if !strings.HasPrefix(s, `"`) {
		return ""
	}
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			if i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			}
		case '"':
			return b.String()
		default:
			b.WriteByte(c)
		}
	}
	return ""
}
