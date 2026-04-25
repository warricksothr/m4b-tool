package chapter

import (
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestOverloadFromTracks_AssignsByBiggestOverlap(t *testing.T) {
	tracks := []audio.Chapter{
		ch(0, 30*time.Second, "file-01.mp3"),
		ch(30*time.Second, 30*time.Second, "file-02.mp3"),
	}
	named := []audio.Chapter{
		ch(0, 25*time.Second, "First Named"),               // big overlap with track 0
		ch(25*time.Second, 35*time.Second, "Second Named"), // big overlap with track 1
	}
	out := OverloadFromTracks(tracks, named)
	if out[0].Name != "First Named" {
		t.Errorf("track 0 name = %q, want First Named", out[0].Name)
	}
	if out[1].Name != "Second Named" {
		t.Errorf("track 1 name = %q, want Second Named", out[1].Name)
	}
}

func TestOverloadFromTracks_NoOverlapKeepsOriginal(t *testing.T) {
	tracks := []audio.Chapter{ch(0, 30*time.Second, "original")}
	named := []audio.Chapter{ch(100*time.Second, 30*time.Second, "elsewhere")}
	out := OverloadFromTracks(tracks, named)
	if out[0].Name != "original" {
		t.Errorf("got %q, want original", out[0].Name)
	}
}

func TestOverloadFromTracks_CopiesIntroduction(t *testing.T) {
	tracks := []audio.Chapter{ch(0, 30*time.Second, "t")}
	named := []audio.Chapter{{Start: 0, Length: 30 * time.Second, Name: "Named", Introduction: "snippet"}}
	out := OverloadFromTracks(tracks, named)
	if out[0].Introduction != "snippet" {
		t.Errorf("Introduction = %q, want snippet", out[0].Introduction)
	}
}

func TestOverloadFromTracks_Empty(t *testing.T) {
	out := OverloadFromTracks(nil, nil)
	if len(out) != 0 {
		t.Errorf("got %d", len(out))
	}
}
