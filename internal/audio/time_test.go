package audio

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"00:00:00", 0, false},
		{"00:00:10", 10 * time.Second, false},
		{"00:01:30", 90 * time.Second, false},
		{"01:00:00", time.Hour, false},
		{"01:23:45.678", time.Hour + 23*time.Minute + 45*time.Second + 678*time.Millisecond, false},
		{"00:00:10.500", 10500 * time.Millisecond, false},
		{"10", 10 * time.Second, false},
		{"10.5", 10500 * time.Millisecond, false},
		{"0.250", 250 * time.Millisecond, false},
		{"  00:00:10  ", 10 * time.Second, false},

		{"", 0, true},
		{"abc", 0, true},
		{"00:60:00", 0, true},   // minutes out of range
		{"00:00:60", 0, true},   // seconds out of range
		{"-1", 0, true},         // negative
		{"00:00:10:0", 0, true}, // too many fields
		{"00:00", 0, true},      // too few fields
	}
	for _, tc := range tests {
		got, err := ParseDuration(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseDuration(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseDuration(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseDuration(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestFormatHMS(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "00:00:00.000"},
		{10 * time.Second, "00:00:10.000"},
		{90 * time.Second, "00:01:30.000"},
		{time.Hour + 23*time.Minute + 45*time.Second + 678*time.Millisecond, "01:23:45.678"},
		{-5 * time.Second, "00:00:00.000"},
	}
	for _, tc := range tests {
		if got := FormatHMS(tc.in); got != tc.want {
			t.Errorf("FormatHMS(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
