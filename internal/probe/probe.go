// Package probe wraps ffprobe and parses stream metadata for a media file.
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// FFprobePath can be overridden for tests.
var FFprobePath = "ffprobe"

// Stream types reported by ffprobe.
const (
	TypeVideo      = "video"
	TypeAudio      = "audio"
	TypeSubtitle   = "subtitle"
	TypeAttachment = "attachment"
	TypeData       = "data"
)

// Stream is a single interesting ffprobe stream. Absolute ffprobe indexes are
// used everywhere so that ffmpeg "-map 0:<index>" arguments can be built
// directly from them.
type Stream struct {
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Channels int    `json:"channels"`
	Layout   string `json:"layout"`
}

// Label returns a human readable track label, e.g. "eng  AC3  5.1  Commentary".
func (s Stream) Label() string {
	var b strings.Builder
	if s.Language != "" {
		b.WriteString(s.Language)
	} else {
		b.WriteString("und")
	}
	if s.Codec != "" {
		b.WriteString("  ")
		b.WriteString(s.Codec)
	}
	if s.Layout != "" {
		b.WriteString("  ")
		b.WriteString(s.Layout)
	} else if s.Channels > 0 {
		b.WriteString(fmt.Sprintf("  %dch", s.Channels))
	}
	if s.Title != "" {
		b.WriteString("  —  ")
		b.WriteString(s.Title)
	}
	return b.String()
}

// IsEnglish reports whether the stream language is English (en or eng).
func (s Stream) IsEnglish() bool { return IsEnglish(s.Language) }

// IsEnglish reports whether an ISO 639 language code is English.
func IsEnglish(lang string) bool {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "en", "eng", "английский":
		return true
	}
	return false
}

// Media is the probe result for one file.
type Media struct {
	Path        string   `json:"path"`
	Duration    float64  `json:"duration"` // seconds, 0 if unknown
	Video       []Stream `json:"video"`
	Audio       []Stream `json:"audio"`
	Subtitles   []Stream `json:"subtitles"`
	Attachments []Stream `json:"attachments"`
	HasChapters bool     `json:"has_chapters"`

	// MainVideoIndex is the absolute index of the "main" video stream (the
	// first one that is not cover art). Falls back to the first video stream.
	MainVideoIndex int `json:"main_video_index"`
}

type ffStream struct {
	Index         int    `json:"index"`
	CodecName     string `json:"codec_name"`
	CodecType     string `json:"codec_type"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
	Disposition   struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
	Tags struct {
		Language string `json:"language"`
		Title    string `json:"title"`
	} `json:"tags"`
}

type ffOutput struct {
	Streams  []ffStream `json:"streams"`
	Chapters []struct{} `json:"chapters"`
	Format   struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// File probes a single media file.
func File(ctx context.Context, path string) (*Media, error) {
	cmd := exec.CommandContext(ctx, FFprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format", "-show_streams", "-show_chapters",
		"--", path,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("ffprobe %s: %w: %s", path, err, msg)
		}
		return nil, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	return parse(path, out)
}

func parse(path string, data []byte) (*Media, error) {
	var f ffOutput
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse ffprobe output for %s: %w", path, err)
	}
	m := &Media{Path: path, MainVideoIndex: -1}
	for _, s := range f.Streams {
		st := Stream{
			Index:    s.Index,
			Type:     s.CodecType,
			Codec:    s.CodecName,
			Language: strings.ToLower(strings.TrimSpace(s.Tags.Language)),
			Title:    s.Tags.Title,
			Channels: s.Channels,
			Layout:   s.ChannelLayout,
		}
		switch s.CodecType {
		case TypeVideo:
			m.Video = append(m.Video, st)
			if m.MainVideoIndex < 0 && s.Disposition.AttachedPic == 0 {
				m.MainVideoIndex = s.Index
			}
		case TypeAudio:
			m.Audio = append(m.Audio, st)
		case TypeSubtitle:
			m.Subtitles = append(m.Subtitles, st)
		case TypeAttachment:
			m.Attachments = append(m.Attachments, st)
		}
	}
	if m.MainVideoIndex < 0 && len(m.Video) > 0 {
		m.MainVideoIndex = m.Video[0].Index
	}
	m.HasChapters = len(f.Chapters) > 0
	if d, err := strconv.ParseFloat(f.Format.Duration, 64); err == nil && d > 0 {
		m.Duration = d
	}
	return m, nil
}

// DurationOf returns the media duration as a time.Duration (0 if unknown).
func (m *Media) DurationOf() time.Duration { return time.Duration(m.Duration * float64(time.Second)) }
