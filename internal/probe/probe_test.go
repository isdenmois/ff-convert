package probe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// hasFFmpeg reports whether ffmpeg is available for fixture generation.
func hasFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func TestParse(t *testing.T) {
	js := `{
 "streams": [
  {"index":0,"codec_name":"h264","codec_type":"video","disposition":{}},
  {"index":1,"codec_name":"mjpeg","codec_type":"video","disposition":{"attached_pic":1},"tags":{"language":"eng"}},
  {"index":2,"codec_name":"ac3","codec_type":"audio","channels":6,"channel_layout":"5.1(side)","tags":{"language":"ENG","title":"Surround"}},
  {"index":3,"codec_name":"aac","codec_type":"audio","channels":2,"channel_layout":"stereo","tags":{"language":"rus"}},
  {"index":4,"codec_name":"subrip","codec_type":"subtitle","tags":{"language":"eng"}},
  {"index":5,"codec_name":"ttf","codec_type":"attachment","tags":{"filename":"font.ttf"}}
 ],
 "chapters": [{"id":0}],
 "format": {"duration": "1234.5"}
}`
	m, err := parse("x.mkv", []byte(js))
	if err != nil {
		t.Fatal(err)
	}
	if m.MainVideoIndex != 0 {
		t.Fatalf("MainVideoIndex = %d, want 0", m.MainVideoIndex)
	}
	if len(m.Audio) != 2 || len(m.Subtitles) != 1 || len(m.Attachments) != 1 || len(m.Video) != 2 {
		t.Fatalf("stream split wrong: %+v", m)
	}
	a := m.Audio[0]
	if a.Index != 2 || a.Language != "eng" || !a.IsEnglish() || a.Channels != 6 || a.Title != "Surround" {
		t.Fatalf("audio[0] = %+v", a)
	}
	if m.Audio[1].IsEnglish() {
		t.Fatal("rus track must not be english")
	}
	if m.Duration != 1234.5 {
		t.Fatalf("Duration = %v", m.Duration)
	}
	if !m.HasChapters {
		t.Fatal("HasChapters should be true")
	}
	if got := m.Audio[0].Label(); got != "eng  ac3  5.1(side)  —  Surround" {
		t.Fatalf("Label = %q", got)
	}
}

func TestParseCoverArtOnly(t *testing.T) {
	js := `{"streams":[
	 {"index":0,"codec_name":"mjpeg","codec_type":"video","disposition":{"attached_pic":1}},
	 {"index":1,"codec_name":"hevc","codec_type":"video","disposition":{}}
	],"format":{"duration":"10"}}`
	m, err := parse("x.mp4", []byte(js))
	if err != nil {
		t.Fatal(err)
	}
	if m.MainVideoIndex != 1 {
		t.Fatalf("MainVideoIndex = %d, want 1 (skip cover art)", m.MainVideoIndex)
	}
}

// TestFileWithFFmpeg generates a real fixture with ffmpeg and probes it.
func TestFileWithFFmpeg(t *testing.T) {
	if !hasFFmpeg() {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.mkv")
	srt := filepath.Join(dir, "sub.srt")
	txt := filepath.Join(dir, "poster.txt")
	os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nhello\n"), 0o644)
	os.WriteFile(txt, []byte("poster"), 0o644)

	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=160x120:rate=10",
		"-f", "lavfi", "-i", "anoisesrc=duration=1:colour=pink:seed=1,pan=5.1|c0=c0|c1=c0|c2=c0|c3=c0|c4=c0|c5=c0",
		"-f", "lavfi", "-i", "anoisesrc=duration=1:colour=white:seed=2,pan=stereo|c0=c0|c1=c0",
		"-i", srt,
		"-map", "0:v", "-map", "1:a", "-map", "2:a", "-map", "3:s",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-c:a", "aac", "-metadata:s:a:0", "language=eng", "-metadata:s:a:0", "title=Surround",
		"-metadata:s:a:1", "language=rus",
		"-c:s", "srt", "-metadata:s:s:0", "language=eng",
		"-attach", txt, "-metadata:s:t:0", "filename=poster.txt", "-metadata:s:t:0", "mimetype=text/plain",
		fixture,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg fixture: %v\n%s", err, out)
	}
	m, err := File(context.Background(), fixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Audio) != 2 {
		t.Fatalf("audio tracks = %d, want 2 (%+v)", len(m.Audio), m)
	}
	if !m.Audio[0].IsEnglish() || m.Audio[0].Channels != 6 || m.Audio[0].Layout != "5.1" {
		t.Fatalf("audio[0] = %+v", m.Audio[0])
	}
	if m.Audio[1].Language != "rus" || m.Audio[1].Channels != 2 {
		t.Fatalf("audio[1] = %+v", m.Audio[1])
	}
	if len(m.Subtitles) != 1 || !m.Subtitles[0].IsEnglish() {
		t.Fatalf("subtitles = %+v", m.Subtitles)
	}
	if len(m.Attachments) != 1 {
		t.Fatalf("attachments = %+v", m.Attachments)
	}
	if m.Duration < 0.9 || m.Duration > 2 {
		t.Fatalf("Duration = %v", m.Duration)
	}
}
