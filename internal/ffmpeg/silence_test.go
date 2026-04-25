package ffmpeg

import (
	"testing"
	"time"
)

func TestParseSilenceLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantOK     bool
		wantStart  time.Duration
		wantLength time.Duration
	}{
		{
			name:       "typical line with prefix",
			line:       "[silencedetect @ 0x55f2d3a0] silence_end: 125.789 | silence_duration: 2.333",
			wantOK:     true,
			wantStart:  123456 * time.Millisecond,
			wantLength: 2333 * time.Millisecond,
		},
		{
			name:       "start rounds to 0 when duration rounds up",
			line:       "[silencedetect @ 0x0] silence_end: 1.0 | silence_duration: 1.0",
			wantOK:     true,
			wantStart:  0,
			wantLength: time.Second,
		},
		{
			name:       "integer seconds accepted",
			line:       "silence_end: 10 | silence_duration: 3",
			wantOK:     true,
			wantStart:  7 * time.Second,
			wantLength: 3 * time.Second,
		},
		{"silence_start ignored", "[silencedetect @ 0x0] silence_start: 1.234", false, 0, 0},
		{"unrelated stderr line", "Stream mapping:", false, 0, 0},
		{"empty line", "", false, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, ok := parseSilenceLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if s.Start != tc.wantStart {
				t.Errorf("Start = %v, want %v", s.Start, tc.wantStart)
			}
			if s.Length != tc.wantLength {
				t.Errorf("Length = %v, want %v", s.Length, tc.wantLength)
			}
		})
	}
}

func TestFormatSilenceSeconds(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{500 * time.Millisecond, "0.5"},
		{1500 * time.Millisecond, "1.5"},
		{2 * time.Second, "2"},
		{2500 * time.Millisecond, "2.5"},
		{1250 * time.Millisecond, "1.25"},
		{100 * time.Millisecond, "0.1"},
	}
	for _, tc := range tests {
		if got := formatSilenceSeconds(tc.in); got != tc.want {
			t.Errorf("formatSilenceSeconds(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
