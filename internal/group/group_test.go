package group

import (
	"testing"

	"github.com/isden/ff-convert/internal/probe"
)

func media(path string, audio, subs []probe.Stream) *probe.Media {
	return &probe.Media{Path: path, Audio: audio, Subtitles: subs}
}

func paths(ms []*probe.Media) string {
	out := ""
	for i, m := range ms {
		if i > 0 {
			out += ", "
		}
		out += m.Path
	}
	return "[" + out + "]"
}

func TestFilesStrict(t *testing.T) {
	trackSet := func(offset int) ([]probe.Stream, []probe.Stream) {
		audio := []probe.Stream{
			{Index: offset + 1, Codec: "ac3", Language: "eng", Title: "Surround", Channels: 6, Layout: "5.1"},
			{Index: offset + 2, Codec: "aac", Language: "rus"},
		}
		subs := []probe.Stream{{Index: offset + 3, Codec: "subrip", Language: "eng"}}
		return audio, subs
	}

	a1, s1 := trackSet(0)
	a2, s2 := trackSet(10)
	a3, s3 := trackSet(20)
	a4, s4 := trackSet(30) // different: extra rus sub
	s4 = append(s4, probe.Stream{Index: 44, Codec: "subrip", Language: "rus"})
	a5, s5 := trackSet(40) // different: eng track title differs
	a5[0].Title = "Commentary"

	files := []*probe.Media{
		media("ep1.mkv", a1, s1),
		media("ep2.mkv", a2, s2),
		media("movie1.mkv", a3, s3),
		media("odd.mkv", a4, s4),
		media("title-differs.mkv", a5, s5),
	}
	groups := Files(files)
	if len(groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(groups))
	}
	// ep1, ep2, movie1 share identical track lists (only absolute indexes
	// differ, which must not split the group).
	if len(groups[0].Files) != 3 || groups[0].Files[0].Path != "ep1.mkv" {
		t.Fatalf("group0 files = %s", paths(groups[0].Files))
	}
	if len(groups[1].Files) != 1 || groups[1].Files[0].Path != "odd.mkv" {
		t.Fatalf("group1 files = %s", paths(groups[1].Files))
	}
	if len(groups[2].Files) != 1 || groups[2].Files[0].Path != "title-differs.mkv" {
		t.Fatalf("group2 files = %s", paths(groups[2].Files))
	}
}

func TestDefaults(t *testing.T) {
	g := &Group{
		Audio: []probe.Stream{
			{Index: 1, Language: "rus"},
			{Index: 2, Language: "eng"},
			{Index: 3, Language: "eng"},
		},
		Subtitles: []probe.Stream{
			{Index: 4, Language: "rus"},
			{Index: 5, Language: "eng"},
		},
	}
	if got := DefaultAudio(g); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("DefaultAudio = %v", got)
	}
	if got := DefaultSubs(g); len(got) != 1 || got[0] != 5 {
		t.Fatalf("DefaultSubs = %v", got)
	}

	// No English anywhere: first audio, no subs.
	g2 := &Group{
		Audio:     []probe.Stream{{Index: 1, Language: "fra"}, {Index: 2, Language: "rus"}},
		Subtitles: []probe.Stream{{Index: 3, Language: "fra"}},
	}
	if got := DefaultAudio(g2); len(got) != 1 || got[0] != 1 {
		t.Fatalf("DefaultAudio(no eng) = %v", got)
	}
	if got := DefaultSubs(g2); len(got) != 0 {
		t.Fatalf("DefaultSubs(no eng) = %v", got)
	}
}
