package audio

import (
	"testing"
	"time"
)

func TestChapterEnd(t *testing.T) {
	c := Chapter{Start: 10 * time.Second, Length: 5 * time.Second}
	if got := c.End(); got != 15*time.Second {
		t.Errorf("End = %v, want 15s", got)
	}
}

func TestSilenceEndAndMidpoint(t *testing.T) {
	s := Silence{Start: 10 * time.Second, Length: 4 * time.Second}
	if got := s.End(); got != 14*time.Second {
		t.Errorf("End = %v, want 14s", got)
	}
	if got := s.Midpoint(); got != 12*time.Second {
		t.Errorf("Midpoint = %v, want 12s", got)
	}
}
