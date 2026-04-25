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
