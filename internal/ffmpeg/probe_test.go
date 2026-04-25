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

const sampleM4BHeader = `Input #0, mov,mp4,m4a,3gp,3g2,mj2, from '/work/book.m4b':
  Metadata:
    major_brand     : isom
  Duration: 16:54:48.29, start: 0.000000, bitrate: 127 kb/s
  Stream #0:0[0x1](und): Audio: aac (LC) (mp4a / 0x6134706D), 44100 Hz, stereo, fltp, 125 kb/s (default)
    Metadata:
      handler_name    : SoundHandler
  Stream #0:1[0x2](eng): Data: bin_data (text / 0x74786574)
  Stream #0:2[0x0]: Video: mjpeg (Baseline), yuvj420p(pc, bt470bg/unknown/unknown), 500x500, 90k tbr, 90k tbn (attached pic)
At least one output file must be specified
`

const sampleFLACHeader = `Input #0, flac, from '/tmp/x.flac':
  Metadata:
    encoder         : Lavf58.76.100
  Duration: 00:03:14.00, start: 0.000000
  Stream #0:0: Audio: flac, 44100 Hz, stereo, s16
At least one output file must be specified
`

func TestParseAudioBitrateFromHeader_M4B(t *testing.T) {
	// Must pick the 125 kb/s on the audio stream line, NOT the 127 kb/s
	// on the Duration line (which is the container bitrate).
	bps, err := parseAudioBitrateFromHeader(sampleM4BHeader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bps != 125000 {
		t.Errorf("got %d, want 125000", bps)
	}
}

func TestParseAudioBitrateFromHeader_MP3(t *testing.T) {
	bps, err := parseAudioBitrateFromHeader(sampleHeader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bps != 32000 {
		t.Errorf("got %d, want 32000", bps)
	}
}

func TestParseAudioBitrateFromHeader_LosslessReturnsZero(t *testing.T) {
	// FLAC streams typically lack a kb/s on the Audio line; the parser
	// returns (0, nil) so the caller can fall back to a codec default.
	bps, err := parseAudioBitrateFromHeader(sampleFLACHeader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bps != 0 {
		t.Errorf("got %d, want 0 (unknown)", bps)
	}
}

func TestParseAudioBitrateFromHeader_NoAudio(t *testing.T) {
	bps, err := parseAudioBitrateFromHeader("ffmpeg version 6.1.1\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bps != 0 {
		t.Errorf("got %d, want 0", bps)
	}
}
