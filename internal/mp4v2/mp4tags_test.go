package mp4v2

import (
	"reflect"
	"testing"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

func TestBuildWriteTagsArgs_Empty(t *testing.T) {
	got := buildWriteTagsArgs(&audio.Tag{}, true, true)
	if got != nil {
		t.Errorf("expected nil for empty tag, got %v", got)
	}
}

func TestBuildWriteTagsArgs_Basic(t *testing.T) {
	tag := &audio.Tag{
		Title:  "T",
		Album:  "AL",
		Artist: "AR",
		Track:  3,
		Tracks: 12,
		Year:   2024,
	}
	got := buildWriteTagsArgs(tag, false, false)
	want := []string{
		"-a", "AR",
		"-A", "AL",
		"-s", "T",
		"-t", "3",
		"-T", "12",
		"-y", "2024",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got:  %v\nwant: %v", got, want)
	}
}

func TestBuildWriteTagsArgs_SkipsZeros(t *testing.T) {
	got := buildWriteTagsArgs(&audio.Tag{Title: "Only"}, false, false)
	if !reflect.DeepEqual(got, []string{"-s", "Only"}) {
		t.Errorf("got %v", got)
	}
}

func TestBuildWriteTagsArgs_MediaTypeAsInt(t *testing.T) {
	got := buildWriteTagsArgs(&audio.Tag{MediaType: audio.MediaTypeAudioBook}, false, false)
	if !reflect.DeepEqual(got, []string{"-i", "2"}) {
		t.Errorf("got %v", got)
	}
}

func TestBuildWriteTagsArgs_SortNamesGated(t *testing.T) {
	tag := &audio.Tag{SortTitle: "Pratchett, Terry"}

	if got := buildWriteTagsArgs(tag, false, false); len(got) != 0 {
		t.Errorf("expected empty when SortNames unsupported, got %v", got)
	}
	got := buildWriteTagsArgs(tag, true, false)
	if !reflect.DeepEqual(got, []string{"-sortname", "Pratchett, Terry"}) {
		t.Errorf("got %v", got)
	}
}

func TestBuildWriteTagsArgs_PurchaseDateGated(t *testing.T) {
	when := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	tag := &audio.Tag{PurchaseDate: when}

	if got := buildWriteTagsArgs(tag, false, false); len(got) != 0 {
		t.Errorf("expected no flags when PurchaseDate unsupported, got %v", got)
	}
	got := buildWriteTagsArgs(tag, false, true)
	if !reflect.DeepEqual(got, []string{"-purchasedate", "2024-06-01"}) {
		t.Errorf("got %v", got)
	}
}

func TestBuildWriteTagsArgs_PurchaseDateSkippedWhenZero(t *testing.T) {
	got := buildWriteTagsArgs(&audio.Tag{}, false, true)
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestBuildWriteTagsArgs_AllSortFields(t *testing.T) {
	tag := &audio.Tag{
		SortTitle:       "st",
		SortAlbum:       "sal",
		SortArtist:      "sar",
		SortAlbumArtist: "saa",
		SortWriter:      "sw",
	}
	got := buildWriteTagsArgs(tag, true, false)
	want := []string{
		"-sortname", "st",
		"-sortalbum", "sal",
		"-sortartist", "sar",
		"-sortalbumartist", "saa",
		"-sortcomposer", "sw",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got:  %v\nwant: %v", got, want)
	}
}
