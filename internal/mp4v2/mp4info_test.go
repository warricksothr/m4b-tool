package mp4v2

import (
	"testing"
	"time"
)

func TestParseMp4InfoDuration_MsForm(t *testing.T) {
	in := `Track   Type    Info
1       audio   MPEG-4 AAC LC, 19.012 secs, 32 kbps, 44100 Hz
duration:    19012 ms
`
	got, err := parseMp4InfoDuration(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 19012*time.Millisecond {
		t.Errorf("got %v, want 19.012s", got)
	}
}

func TestParseMp4InfoDuration_SecsForm(t *testing.T) {
	in := `Track   Type    Info
1       audio   MPEG-4 AAC LC, 0.684 secs, 32 kbps, 44100 Hz
`
	got, err := parseMp4InfoDuration(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 684*time.Millisecond {
		t.Errorf("got %v, want 684ms", got)
	}
}

func TestParseMp4InfoDuration_PrefersMsOverSecs(t *testing.T) {
	// When both forms appear the ms line wins — it has higher precision.
	in := `1 audio AAC, 19.012 secs, 32 kbps
duration:    19234 ms
`
	got, err := parseMp4InfoDuration(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 19234*time.Millisecond {
		t.Errorf("got %v, want 19.234s", got)
	}
}

func TestParseMp4InfoDuration_NoMatch(t *testing.T) {
	_, err := parseMp4InfoDuration("something else entirely\n")
	if err == nil {
		t.Fatal("expected error when no duration present")
	}
}
