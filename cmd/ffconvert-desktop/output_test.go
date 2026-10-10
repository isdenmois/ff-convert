package main

import "testing"

func TestParseVideosDir(t *testing.T) {
	cases := []struct {
		name, content, want string
	}{
		{"home relative", `XDG_VIDEOS_DIR="$HOME/Videos"`, "/home/u/Videos"},
		{"home only", "XDG_VIDEOS_DIR=\"$HOME\"\n", "/home/u"},
		{"absolute", `XDG_VIDEOS_DIR="/mnt/media/videos"`, "/mnt/media/videos"},
		{"escaped", `XDG_VIDEOS_DIR="$HOME/My\"Videos"`, `/home/u/My"Videos`},
		{"other entries first", "XDG_MUSIC_DIR=\"$HOME/Music\"\nXDG_VIDEOS_DIR=\"$HOME/Films\"\n", "/home/u/Films"},
		{"missing entry", "XDG_MUSIC_DIR=\"$HOME/Music\"\n", ""},
		{"relative ignored", `XDG_VIDEOS_DIR="Videos"`, ""},
		{"unquoted ignored", `XDG_VIDEOS_DIR=$HOME/Videos`, ""},
		{"comment line", "# XDG_VIDEOS_DIR=\"$HOME/nope\"\nXDG_VIDEOS_DIR=\"$HOME/yes\"\n", "/home/u/yes"},
	}
	for _, c := range cases {
		if got := parseVideosDir(c.content, "/home/u"); got != c.want {
			t.Errorf("%s: parseVideosDir = %q, want %q", c.name, got, c.want)
		}
	}
}
