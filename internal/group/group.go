// Package group groups probed files by identical audio and subtitle track
// lists (language, title, codec, order and counts all equal).
package group

import (
	"fmt"
	"sort"
	"strings"

	"github.com/isden/ff-convert/internal/probe"
)

// Group is a set of files sharing the same audio+subtitle track layout.
type Group struct {
	// Key is the grouping key (unique per distinct track layout).
	Key string
	// Files are the probed files in this group, in scan order.
	Files []*probe.Media
	// Audio and Subtitles are the representative track lists (from Files[0]).
	Audio     []probe.Stream
	Subtitles []probe.Stream
}

// Name returns a short human label for the group.
func (g *Group) Name() string {
	langs := make([]string, 0, len(g.Audio)+len(g.Subtitles))
	for _, a := range g.Audio {
		l := a.Language
		if l == "" {
			l = "und"
		}
		if !contains(langs, l) {
			langs = append(langs, l)
		}
	}
	return fmt.Sprintf("%d file(s), audio: %s", len(g.Files), strings.Join(langs, ","))
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// key builds the strict grouping key for a file's track lists. Absolute
// stream indexes are deliberately excluded: two files whose tracks live at
// different indexes (e.g. one has cover art shifting the numbering) but with
// identical language/title/codec lists in the same order still group together.
func key(m *probe.Media) string {
	var b strings.Builder
	for _, s := range m.Audio {
		fmt.Fprintf(&b, "a:%s|%s|%s;", s.Codec, s.Language, s.Title)
	}
	b.WriteString("||")
	for _, s := range m.Subtitles {
		fmt.Fprintf(&b, "s:%s|%s|%s;", s.Codec, s.Language, s.Title)
	}
	return b.String()
}

// Files groups probed files. Groups are returned in first-seen order.
func Files(files []*probe.Media) []*Group {
	var groups []*Group
	byKey := make(map[string]*Group)
	for _, m := range files {
		k := key(m)
		g, ok := byKey[k]
		if !ok {
			g = &Group{Key: k, Audio: m.Audio, Subtitles: m.Subtitles}
			byKey[k] = g
			groups = append(groups, g)
		}
		g.Files = append(g.Files, m)
	}
	return groups
}

// DefaultAudio returns the pre-selected audio stream indexes for a group:
// every English track, or the first audio track when there is no English one.
func DefaultAudio(g *Group) []int {
	var out []int
	for _, s := range g.Audio {
		if s.IsEnglish() {
			out = append(out, s.Index)
		}
	}
	if len(out) == 0 && len(g.Audio) > 0 {
		out = append(out, g.Audio[0].Index)
	}
	return out
}

// DefaultSubs returns the pre-selected subtitle stream indexes: every English
// subtitle track, or none.
func DefaultSubs(g *Group) []int {
	var out []int
	for _, s := range g.Subtitles {
		if s.IsEnglish() {
			out = append(out, s.Index)
		}
	}
	return out
}

// SortGroupsByFirstFile orders groups by the first file path of each group.
func SortGroupsByFirstFile(groups []*Group) {
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].Files[0].Path < groups[j].Files[0].Path
	})
}
