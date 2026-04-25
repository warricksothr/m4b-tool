package ffmpeg

import (
	"testing"
	"time"
)

const sampleHeader = `Input #0, mp3, from '/tmp/x.mp3':
  Metadata:
    encoder         : Lavf58.76.100
  Duration: 00:02:30.45, start: 0.000000, bitrate: 32 kb/s
  Stream #0:0: Audio: mp3, 22050 Hz, mono, fltp, 32 kb/s
At least one output file must be specified
`

const sampleStats = `size=N/A time=00:00:00.50 bitrate=N/A speed=1.5x    ` + "\r" +
	`size=N/A time=00:00:01.00 bitrate=N/A speed=1.6x    ` + "\r" +
	`size=N/A time=00:00:02.50 bitrate=N/A speed=1.7x    ` + "\r" +
	`size=N/A time=00:00:02.68 bitrate=N/A speed=1.7x    ` + "\n"

func TestParseDurationFromHeader(t *testing.T) {
	d, err := parseDurationFromHeader(sampleHeader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := 2*time.Minute + 30*time.Second + 450*time.Millisecond
	if d != want {
		t.Errorf("got %v, want %v", d, want)
	}
}

func TestParseDurationFromHeader_Missing(t *testing.T) {
	_, err := parseDurationFromHeader("ffmpeg version 6.1.1\nnope\n")
	if err == nil {
		t.Fatal("expected error when Duration absent")
	}
}

func TestParseDurationFromTimeStats_ReturnsLast(t *testing.T) {
	d, err := parseDurationFromTimeStats(sampleStats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := 2*time.Second + 680*time.Millisecond
	if d != want {
		t.Errorf("got %v, want %v", d, want)
	}
}

func TestParseDurationFromTimeStats_Missing(t *testing.T) {
	_, err := parseDurationFromTimeStats("no stats here")
	if err == nil {
		t.Fatal("expected error when no time= stat present")
	}
}

func TestNewClient_MissingBinary(t *testing.T) {
	_, err := NewClient("/absolutely/not/a/real/binary/ffmpeg-xyzzy")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}
