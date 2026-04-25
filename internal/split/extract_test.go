package split

import (
	"reflect"
	"testing"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestBuildExtractMetadata_FullTag(t *testing.T) {
	got := buildExtractMetadata(audio.Tag{
		Title:       "ignored when chapter has its own",
		Artist:      "Jeff Hays",
		Album:       "Dungeon Crawler Carl",
		AlbumArtist: "Matt Dinniman",
		Genre:       "Audiobook",
		Writer:      "Matt Dinniman",
		Year:        2020,
		Comment:     "test rip",
		Copyright:   "© 2020",
	}, audio.Chapter{Name: "Opening Credits"}, 1, 50)

	want := map[string]string{
		"title":        "Opening Credits",
		"artist":       "Jeff Hays",
		"album":        "Dungeon Crawler Carl",
		"album_artist": "Matt Dinniman",
		"genre":        "Audiobook",
		"composer":     "Matt Dinniman",
		"date":         "2020",
		"comment":      "test rip",
		"copyright":    "© 2020",
		"track":        "1/50",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestBuildExtractMetadata_FallsBackToBaseTitle(t *testing.T) {
	got := buildExtractMetadata(audio.Tag{Title: "Fallback"}, audio.Chapter{Name: ""}, 3, 10)
	if got["title"] != "Fallback" {
		t.Errorf("title=%q, want Fallback", got["title"])
	}
}

func TestBuildExtractMetadata_OmitsZeroFields(t *testing.T) {
	got := buildExtractMetadata(audio.Tag{}, audio.Chapter{Name: "Only Title"}, 1, 1)
	want := map[string]string{
		"title": "Only Title",
		"track": "1/1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestBuildExtractMetadata_DropsAudiobookOnlyFields(t *testing.T) {
	// Series / SeriesPart don't have a standard ID3v2 mapping, so
	// they're deliberately omitted from the ffmpeg -metadata map.
	got := buildExtractMetadata(audio.Tag{
		Series:     "Stormlight Archive",
		SeriesPart: "1",
	}, audio.Chapter{Name: "X"}, 1, 1)
	for _, k := range []string{"series", "series_part", "series-part"} {
		if _, ok := got[k]; ok {
			t.Errorf("unexpected field %q in metadata", k)
		}
	}
}

func TestDefaultBitrate(t *testing.T) {
	cases := []struct {
		name      string
		sourceBps int
		want      string
	}{
		{"unknown source → empty (ffmpeg default)", 0, ""},
		{"negative (treated as unknown)", -1, ""},
		{"low-bitrate audiobook AAC", 64_000, "64k"},
		{"typical m4b (125 kbps)", 125_000, "125k"},
		{"at the cap", 192_000, "192k"},
		{"just over the cap", 192_001, "192k"},
		{"FLAC-ish lossless if it reports a number", 1_411_000, "192k"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultBitrate(tc.sourceBps); got != tc.want {
				t.Errorf("defaultBitrate(%d) = %q, want %q", tc.sourceBps, got, tc.want)
			}
		})
	}
}

func TestDefaultCodecForFormat(t *testing.T) {
	cases := []struct {
		format string
		want   string
	}{
		// MP4 family stream-copies the source AAC.
		{"mp4", "copy"},
		{"m4a", "copy"},
		{"m4b", "copy"},
		{"MP4", "copy"}, // case-insensitive

		// Non-MP4 containers must transcode — copy of an AAC stream
		// into an MP3/FLAC/Ogg/Wav muxer fails with "Invalid audio
		// stream", which is the bug this defaulter fixes.
		{"mp3", "libmp3lame"},
		{"flac", "flac"},
		{"ogg", "libvorbis"},
		{"opus", "libopus"},
		{"wav", "pcm_s16le"},

		// Unknown formats: fall through to copy and let ffmpeg surface
		// the error rather than guessing wrong.
		{"", "copy"},
		{"weirdformat", "copy"},
	}
	for _, tc := range cases {
		if got := defaultCodecForFormat(tc.format); got != tc.want {
			t.Errorf("defaultCodecForFormat(%q) = %q, want %q", tc.format, got, tc.want)
		}
	}
}
