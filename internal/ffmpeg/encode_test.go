package ffmpeg

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildTranscodeArgs_Defaults(t *testing.T) {
	got := buildTranscodeArgs("/in.mp3", "/out.m4a", EncodeOptions{})
	want := []string{"-hide_banner", "-i", "/in.mp3", "-vn", "/out.m4a"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v\nwant: %v", got, want)
	}
}

func TestBuildTranscodeArgs_FullSet(t *testing.T) {
	opts := EncodeOptions{
		Format:     "mp4",
		Codec:      "aac",
		Bitrate:    "64k",
		SampleRate: 44100,
		Channels:   2,
		Threads:    4,
		Overwrite:  true,
	}
	got := buildTranscodeArgs("/in.mp3", "/out.m4a", opts)
	want := []string{
		"-hide_banner", "-y", "-i", "/in.mp3",
		"-threads", "4", "-vn",
		"-c:a", "aac", "-b:a", "64k", "-ar", "44100", "-ac", "2",
		"-f", "mp4", "/out.m4a",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v\nwant: %v", got, want)
	}
}

func TestBuildTranscodeArgs_TrimSilenceStartOnly(t *testing.T) {
	got := buildTranscodeArgs("/in.mp3", "/out.m4a", EncodeOptions{TrimSilenceStart: true})
	if !contains(got, "-af") {
		t.Errorf("expected -af flag, got %v", got)
	}
	idx := indexOf(got, "-af")
	filter := got[idx+1]
	if !strings.Contains(filter, "silenceremove") {
		t.Errorf("filter = %q, want silenceremove", filter)
	}
	if strings.Contains(filter, "areverse") {
		t.Errorf("start-only should not reverse: %q", filter)
	}
}

func TestBuildTranscodeArgs_TrimSilenceBoth(t *testing.T) {
	got := buildTranscodeArgs("/in.mp3", "/out.m4a", EncodeOptions{TrimSilenceStart: true, TrimSilenceEnd: true})
	idx := indexOf(got, "-af")
	if idx < 0 {
		t.Fatal("expected -af flag")
	}
	filter := got[idx+1]
	// Both ends → silenceremove,areverse,silenceremove,areverse
	if got, want := strings.Count(filter, "silenceremove"), 2; got != want {
		t.Errorf("got %d silenceremove ops in %q, want %d", got, filter, want)
	}
	if got, want := strings.Count(filter, "areverse"), 2; got != want {
		t.Errorf("got %d areverse ops, want %d", got, want)
	}
}

func TestBuildTranscodeArgs_CustomThreshold(t *testing.T) {
	got := buildTranscodeArgs("/in.mp3", "/out.m4a", EncodeOptions{
		TrimSilenceStart: true,
		SilenceThreshold: "-40dB",
	})
	idx := indexOf(got, "-af")
	if !strings.Contains(got[idx+1], "-40dB") {
		t.Errorf("threshold not applied: %q", got[idx+1])
	}
}

func TestBuildTranscodeArgs_ExtraArgsBeforeOutput(t *testing.T) {
	got := buildTranscodeArgs("/in.mp3", "/out.m4a", EncodeOptions{
		Codec:     "aac",
		ExtraArgs: []string{"-movflags", "+faststart"},
		Format:    "mp4",
	})
	// ExtraArgs must appear after codec/bitrate/etc but before -f or output.
	idxCodec := indexOf(got, "-c:a")
	idxExtra := indexOf(got, "-movflags")
	idxFmt := indexOf(got, "-f")
	if idxCodec >= idxExtra || idxExtra >= idxFmt {
		t.Errorf("argv ordering wrong: %v", got)
	}
}

func TestBuildSilenceArgs_Defaults(t *testing.T) {
	got := buildSilenceArgs("/silence.m4a", 0.5, SilenceOptions{Overwrite: true})
	// Spot-check the key parts.
	if !contains(got, "-y") {
		t.Errorf("missing -y: %v", got)
	}
	if !contains(got, "-c:a") || !contains(got, "aac") {
		t.Errorf("missing default codec: %v", got)
	}
	if !contains(got, "0.500") {
		t.Errorf("missing duration: %v", got)
	}
	idx := indexOf(got, "-i")
	if idx < 0 || !strings.Contains(got[idx+1], "anullsrc") {
		t.Errorf("missing anullsrc input: %v", got)
	}
	if !strings.Contains(got[idx+1], "channel_layout=stereo") {
		t.Errorf("default channel layout wrong: %q", got[idx+1])
	}
	if !strings.Contains(got[idx+1], "sample_rate=44100") {
		t.Errorf("default sample rate wrong: %q", got[idx+1])
	}
}

func TestBuildSilenceArgs_MonoOverride(t *testing.T) {
	got := buildSilenceArgs("/s.m4a", 1.0, SilenceOptions{Channels: 1, SampleRate: 22050})
	idx := indexOf(got, "-i")
	if !strings.Contains(got[idx+1], "channel_layout=mono") {
		t.Errorf("mono not honored: %q", got[idx+1])
	}
	if !strings.Contains(got[idx+1], "sample_rate=22050") {
		t.Errorf("rate not honored: %q", got[idx+1])
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}
