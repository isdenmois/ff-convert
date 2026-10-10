package plan

import (
	"strings"
	"testing"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/probe"
)

func testMedia() *probe.Media {
	return &probe.Media{
		Path:           "/movies/Movie.mkv",
		Duration:       100,
		MainVideoIndex: 0,
		Video:          []probe.Stream{{Index: 0, Type: "video", Codec: "h264"}},
		Audio: []probe.Stream{
			{Index: 1, Type: "audio", Codec: "ac3", Language: "eng", Channels: 6, Layout: "5.1"},
			{Index: 2, Type: "audio", Codec: "aac", Language: "rus", Channels: 2, Layout: "stereo"},
			{Index: 3, Type: "audio", Codec: "eac3", Language: "eng", Channels: 8, Layout: "7.1"},
			{Index: 4, Type: "audio", Codec: "ac3", Language: "jpn", Channels: 1, Layout: "mono"},
		},
		Subtitles: []probe.Stream{
			{Index: 5, Type: "subtitle", Codec: "subrip", Language: "eng"},
			{Index: 6, Type: "subtitle", Codec: "hdmv_pgs_subtitle", Language: "rus"},
		},
		Attachments: []probe.Stream{{Index: 7, Type: "attachment", Codec: "ttf"}},
	}
}

func TestBuildAv1Full(t *testing.T) {
	m := testMedia()
	ans := Answers{
		Audio:       []int{1, 2},
		Subs:        []int{5, 6},
		Normalize:   true,
		Attachments: true,
	}
	presets := PresetsMap(&config.Config{})
	task, err := Build(m, "av1", ans, presets, "128k", "/movies/converted/Movie.mkv")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(task.Args, " ")
	mustContain := []string{
		"-c:v libsvtav1 -preset 6 -crf 28 -pix_fmt yuv420p",
		"-map 0:0",
		// 5.1 eng: pan + loudnorm via filter_complex
		"-filter_complex [0:1]pan=stereo|c0=c2+0.30*c0+0.30*c4|c1=c2+0.30*c1+0.30*c5,loudnorm=i=-13:lra=15:tp=-1[a0]",
		// stereo rus: only loudnorm
		"[0:2]loudnorm=i=-13:lra=15:tp=-1[a1]",
		"-map [a0] -map [a1]",
		"-c:a libopus -b:a 128k -ar 48000",
		"-map 0:5 -map 0:6 -c:s copy",
		"-map 0:t? -c:t copy",
		"-map_chapters 0",
		"-progress pipe:1 -nostats -f matroska /movies/converted/Movie.mkv.tmp",
	}
	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q\nargs: %s", want, joined)
		}
	}
	if task.Tmp != task.Output+".tmp" {
		t.Errorf("Tmp = %q", task.Tmp)
	}
	if task.Duration != 100_000_000_000 {
		t.Errorf("Duration = %v", task.Duration)
	}
}

func TestBuildCopyNoFilters(t *testing.T) {
	m := testMedia()
	ans := Answers{Audio: []int{2}, Attachments: false}
	task, err := Build(m, "copy", ans, PresetsMap(&config.Config{}), "128k", "/o.mkv")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(task.Args, " ")
	if strings.Contains(joined, "filter_complex") {
		t.Errorf("stereo+no-normalize must not use filter_complex:\n%s", joined)
	}
	if !strings.Contains(joined, "-c:v copy") {
		t.Errorf("video must be copied:\n%s", joined)
	}
	if strings.Contains(joined, "0:t") {
		t.Errorf("attachments must be dropped:\n%s", joined)
	}
	if !strings.Contains(joined, "-map 0:2 -c:a libopus") {
		t.Errorf("audio not mapped directly:\n%s", joined)
	}
}

func TestDownmixLayouts(t *testing.T) {
	cases := []struct {
		channels int
		layout   string
		want     string
	}{
		{2, "stereo", ""},
		{1, "mono", "pan=stereo|c0=c0|c1=c0"},
		{6, "5.1", "pan=stereo|c0=c2+0.30*c0+0.30*c4|c1=c2+0.30*c1+0.30*c5"},
		{6, "5.1(side)", "pan=stereo|c0=c2+0.30*c0+0.30*c4|c1=c2+0.30*c1+0.30*c5"},
		{6, "", "pan=stereo|c0=c2+0.30*c0+0.30*c4|c1=c2+0.30*c1+0.30*c5"},
		{8, "7.1", "pan=stereo|c0=c2+0.30*c0+0.30*c4+0.30*c6|c1=c2+0.30*c1+0.30*c5+0.30*c7"},
		{4, "quad", "aformat=channel_layouts=stereo"},
		{6, "hexagonal", "aformat=channel_layouts=stereo"}, // 6ch but not 5.1
	}
	for _, c := range cases {
		got := downmix(probe.Stream{Channels: c.channels, Layout: c.layout})
		if got != c.want {
			t.Errorf("downmix(%d, %q) = %q, want %q", c.channels, c.layout, got, c.want)
		}
	}
}

func TestBuildVAAPI(t *testing.T) {
	m := testMedia()
	ans := Answers{Audio: []int{2}}
	task, err := Build(m, "hevc-vaapi", ans, PresetsMap(&config.Config{}), "128k", "/o.mkv")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(task.Args, " ")
	for _, want := range []string{
		"-vaapi_device /dev/dri/renderD128",
		"-c:v hevc_vaapi -qp 24 -vf format=nv12|vaapi,hwupload",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	// -vaapi_device must come before -i.
	if strings.Index(joined, "-vaapi_device") > strings.Index(joined, " -i ") {
		t.Errorf("-vaapi_device must precede -i:\n%s", joined)
	}
}

func TestBuildErrors(t *testing.T) {
	m := testMedia()
	presets := PresetsMap(&config.Config{})
	if _, err := Build(m, "nope", Answers{Audio: []int{1}}, presets, "128k", "/o.mkv"); err == nil {
		t.Error("unknown preset must error")
	}
	if _, err := Build(m, "av1", Answers{}, presets, "128k", "/o.mkv"); err == nil {
		t.Error("no audio selected must error")
	}
	if _, err := Build(m, "av1", Answers{Audio: []int{99}}, presets, "128k", "/o.mkv"); err == nil {
		t.Error("missing audio stream must error")
	}
	if _, err := Build(m, "av1", Answers{Audio: []int{1}, Subs: []int{99}}, presets, "128k", "/o.mkv"); err == nil {
		t.Error("missing sub stream must error")
	}
}
