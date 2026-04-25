package ffmpeg

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildExtractArgs_StreamCopyDefault(t *testing.T) {
	got := buildExtractArgs("/in.m4b", "/out.m4a", 12500*time.Millisecond, 30*time.Second, ExtractOptions{})
	want := []string{
		"-hide_banner",
		"-ss", "12.500",
		"-i", "/in.m4b",
		"-t", "30.000",
		"-vn",
		"-c:a", "copy",
		"/out.m4a",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestBuildExtractArgs_OmitsSeekWhenZero(t *testing.T) {
	got := buildExtractArgs("/in.m4b", "/out.m4a", 0, 5*time.Second, ExtractOptions{})
	if contains(got, "-ss") {
		t.Errorf("zero start should drop -ss: %v", got)
	}
}

func TestBuildExtractArgs_OmitsDurationWhenZero(t *testing.T) {
	got := buildExtractArgs("/in.m4b", "/out.m4a", 5*time.Second, 0, ExtractOptions{})
	if contains(got, "-t") {
		t.Errorf("zero duration should drop -t: %v", got)
	}
}

func TestBuildExtractArgs_TranscodeOpts(t *testing.T) {
	got := buildExtractArgs("/in.m4b", "/out.mp3", 0, 10*time.Second, ExtractOptions{
		Format:     "mp3",
		Codec:      "libmp3lame",
		Bitrate:    "128k",
		SampleRate: 44100,
		Channels:   2,
		Overwrite:  true,
	})
	for _, want := range []string{"-y", "-c:a", "libmp3lame", "-b:a", "128k", "-ar", "44100", "-ac", "2", "-f", "mp3"} {
		if !contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	// Stream-copy fallback must not appear.
	if strings.Contains(strings.Join(got, " "), "-c:a copy") {
		t.Errorf("explicit codec should not be 'copy': %v", got)
	}
}

func TestBuildExtractArgs_MetadataSortedAndPlaced(t *testing.T) {
	got := buildExtractArgs("/in.m4b", "/out.mp3", 0, 10*time.Second, ExtractOptions{
		Format: "mp3",
		Codec:  "libmp3lame",
		Metadata: map[string]string{
			"title":  "Chapter One",
			"artist": "Narrator",
			"track":  "1/50",
		},
	})
	// -metadata pairs must (a) appear in sorted key order, (b) come
	// after -c:a, and (c) come before -f and the output path.
	idxCodec := indexOf(got, "-c:a")
	idxFmt := indexOf(got, "-f")
	want := []string{"artist=Narrator", "title=Chapter One", "track=1/50"}
	var found []int
	for i, v := range got {
		for _, w := range want {
			if v == w {
				found = append(found, i)
			}
		}
	}
	if len(found) != len(want) {
		t.Fatalf("metadata pairs missing: %v\nargs: %v", want, got)
	}
	for _, i := range found {
		if i <= idxCodec || i >= idxFmt {
			t.Errorf("metadata at %d outside [%d, %d): %v", i, idxCodec, idxFmt, got)
		}
	}
	for i := 1; i < len(found); i++ {
		if found[i] <= found[i-1] {
			t.Errorf("metadata not sorted: %v", got)
		}
	}
}

func TestFormatSeconds3(t *testing.T) {
	cases := map[time.Duration]string{
		0:                       "0.000",
		1500 * time.Millisecond: "1.500",
		3725 * time.Millisecond: "3.725",
	}
	for in, want := range cases {
		if got := formatSeconds3(in); got != want {
			t.Errorf("%v -> %q, want %q", in, got, want)
		}
	}
}
