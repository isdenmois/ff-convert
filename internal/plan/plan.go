// Package plan builds ffmpeg command lines for converting a probed file
// according to a set of answers.
package plan

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/probe"
)

// LoudnormFilter is the audio normalization filter (single-pass EBU R128).
const LoudnormFilter = "loudnorm=i=-13:lra=15:tp=-1"

// Answers is the user's decision for one group of files.
type Answers struct {
	// Audio holds the absolute ffprobe indexes of the audio streams to keep,
	// in the desired output order.
	Audio []int `json:"audio"`
	// Subs holds the absolute ffprobe indexes of the subtitle streams to keep.
	Subs []int `json:"subs"`
	// Presets holds the video preset name for each file of the group,
	// index-aligned with the group's files (the video codec is a per-task
	// choice). Empty entries fall back to the configured default preset.
	Presets []string `json:"presets"`
	// Normalize enables the loudnorm filter.
	Normalize bool `json:"normalize"`
	// Attachments copies embedded attachments (fonts, posters) when true.
	Attachments bool `json:"attachments"`
}

// Task is a fully resolved conversion for one file.
type Task struct {
	Input    string
	Output   string
	Tmp      string // staged output, renamed to Output on success
	Duration time.Duration
	Args     []string // ffmpeg arguments (excluding the binary)
}

// Build constructs the ffmpeg arguments for one file. presetName selects the
// video preset (codec) for this task.
func Build(m *probe.Media, presetName string, ans Answers, presets map[string]config.Preset, bitrate, output string) (*Task, error) {
	preset, ok := presets[presetName]
	if !ok {
		return nil, fmt.Errorf("unknown preset %q", presetName)
	}
	if len(ans.Audio) == 0 {
		return nil, fmt.Errorf("%s: no audio tracks selected", m.Path)
	}
	if !preset.IsCopy() && preset.Codec == "" {
		return nil, fmt.Errorf("preset %q has no codec", presetName)
	}

	audioByIndex := make(map[int]probe.Stream, len(m.Audio))
	for _, s := range m.Audio {
		audioByIndex[s.Index] = s
	}
	subByIndex := make(map[int]probe.Stream, len(m.Subtitles))
	for _, s := range m.Subtitles {
		subByIndex[s.Index] = s
	}

	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-y"}
	if preset.VAAPIDevice != "" {
		args = append(args, "-vaapi_device", preset.VAAPIDevice)
	}
	args = append(args, "-i", m.Path)

	// Video.
	args = append(args, "-map", fmt.Sprintf("0:%d", m.MainVideoIndex))
	if preset.IsCopy() {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-c:v", preset.Codec)
		args = append(args, preset.Args...)
		if preset.PixFmt != "" {
			args = append(args, "-pix_fmt", preset.PixFmt)
		}
		if preset.VF != "" {
			args = append(args, "-vf", preset.VF)
		}
	}

	// Audio. Streams needing a filter (downmix and/or loudnorm) are routed
	// through -filter_complex with labeled pads; the rest are mapped plain.
	var fc []string
	for i, idx := range ans.Audio {
		s, ok := audioByIndex[idx]
		if !ok {
			return nil, fmt.Errorf("%s: audio stream %d not found", m.Path, idx)
		}
		if chain := audioChain(s, ans.Normalize); chain != "" {
			fc = append(fc, fmt.Sprintf("[0:%d]%s[a%d]", idx, chain, i))
			args = append(args, "-map", fmt.Sprintf("[a%d]", i))
		} else {
			args = append(args, "-map", fmt.Sprintf("0:%d", idx))
		}
	}
	if len(fc) > 0 {
		args = append(args, "-filter_complex", strings.Join(fc, ";"))
	}
	args = append(args, "-c:a", "libopus", "-b:a", bitrate, "-ar", "48000")
	// Filters drop stream metadata tags, so re-apply language/title per
	// output audio stream.
	for i, idx := range ans.Audio {
		s := audioByIndex[idx]
		if s.Language != "" {
			args = append(args, "-metadata:s:a:"+itoa(i), "language="+s.Language)
		}
		if s.Title != "" {
			args = append(args, "-metadata:s:a:"+itoa(i), "title="+s.Title)
		}
	}

	// Subtitles: always stream-copied (text and image based).
	for _, idx := range ans.Subs {
		if _, ok := subByIndex[idx]; !ok {
			return nil, fmt.Errorf("%s: subtitle stream %d not found", m.Path, idx)
		}
		args = append(args, "-map", fmt.Sprintf("0:%d", idx))
	}
	if len(ans.Subs) > 0 {
		args = append(args, "-c:s", "copy")
	}

	// Attachments and chapters.
	if ans.Attachments && len(m.Attachments) > 0 {
		args = append(args, "-map", "0:t?", "-c:t", "copy")
	}
	args = append(args, "-map_chapters", "0")

	// Progress protocol and staged output (explicit muxer: the tmp name does
	// not carry a matroska extension).
	args = append(args, "-progress", "pipe:1", "-nostats",
		"-f", "matroska", output+".tmp")

	return &Task{
		Input:    m.Path,
		Output:   output,
		Tmp:      output + ".tmp",
		Duration: m.DurationOf(),
		Args:     args,
	}, nil
}

// audioChain builds the per-stream audio filter chain: downmix to stereo
// (only when needed) followed by optional loudnorm. Returns "" when no
// filtering is required.
func audioChain(s probe.Stream, normalize bool) string {
	var parts []string
	if f := downmix(s); f != "" {
		parts = append(parts, f)
	}
	if normalize {
		parts = append(parts, LoudnormFilter)
	}
	return strings.Join(parts, ",")
}

// downmix returns the stereo downmix filter for a stream, or "" when the
// source is already stereo.
func downmix(s probe.Stream) string {
	switch {
	case s.Channels <= 0, s.Channels == 2:
		return ""
	case s.Channels == 1:
		return "pan=stereo|c0=c0|c1=c0"
	case s.Channels == 6 && hasLayout(s.Layout, "5.1"):
		// Center + 0.30*FL + 0.30*SL (SL/BR are c4/c5 in 5.1).
		return "pan=stereo|c0=c2+0.30*c0+0.30*c4|c1=c2+0.30*c1+0.30*c5"
	case s.Channels == 8 && hasLayout(s.Layout, "7.1"):
		// FL FR FC LFE BL BR SL SR: center + 0.30*(sides + backs).
		return "pan=stereo|c0=c2+0.30*c0+0.30*c4+0.30*c6|c1=c2+0.30*c1+0.30*c5+0.30*c7"
	default:
		// Unknown multi-channel layout: let ffmpeg insert its default
		// downmix to stereo.
		return "aformat=channel_layouts=stereo"
	}
}

func hasLayout(layout, want string) bool {
	return layout == "" || strings.Contains(layout, want)
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

// PresetsMap converts a config into the name→preset map used by Build.
func PresetsMap(cfg *config.Config) map[string]config.Preset {
	out := config.Builtins()
	for k, v := range cfg.Presets {
		out[k] = v
	}
	return out
}

// SortedIndexes returns the answers' track indexes sorted ascending —
// convenient for UIs that toggle selections.
func SortedIndexes(in []int) []int {
	out := append([]int(nil), in...)
	sort.Ints(out)
	return out
}
