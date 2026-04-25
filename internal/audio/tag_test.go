package audio

import (
	"reflect"
	"testing"
	"time"
)

func TestMergeMissing_FillsEmptyFields(t *testing.T) {
	base := Tag{Title: "Keep", Artist: ""}
	base.MergeMissing(Tag{Title: "Ignored", Artist: "Added", Year: 1999})

	if base.Title != "Keep" {
		t.Errorf("Title = %q, want %q", base.Title, "Keep")
	}
	if base.Artist != "Added" {
		t.Errorf("Artist = %q, want %q", base.Artist, "Added")
	}
	if base.Year != 1999 {
		t.Errorf("Year = %d, want 1999", base.Year)
	}
}

func TestMergeMissing_PreservesExistingChapters(t *testing.T) {
	base := Tag{Chapters: []Chapter{{Name: "A"}}}
	base.MergeMissing(Tag{Chapters: []Chapter{{Name: "B"}, {Name: "C"}}})

	if len(base.Chapters) != 1 || base.Chapters[0].Name != "A" {
		t.Errorf("chapters clobbered: %+v", base.Chapters)
	}
}

func TestMergeMissing_FillsChaptersWhenEmpty(t *testing.T) {
	base := Tag{}
	base.MergeMissing(Tag{Chapters: []Chapter{{Name: "B"}}})

	if len(base.Chapters) != 1 || base.Chapters[0].Name != "B" {
		t.Errorf("chapters not filled: %+v", base.Chapters)
	}
}

func TestMergeMissing_ExtraKeepsBaseKeys(t *testing.T) {
	base := Tag{Extra: map[string]string{"asin": "OLD"}}
	base.MergeMissing(Tag{Extra: map[string]string{"asin": "NEW", "isbn": "123"}})

	if base.Extra["asin"] != "OLD" {
		t.Errorf("asin overwritten: %q", base.Extra["asin"])
	}
	if base.Extra["isbn"] != "123" {
		t.Errorf("isbn not added: %q", base.Extra["isbn"])
	}
}

func TestMergeOverwrite_OverridesNonZeroFields(t *testing.T) {
	base := Tag{Title: "Old", Artist: "Keep-me", MediaType: MediaTypeMusic}
	base.MergeOverwrite(Tag{Title: "New", MediaType: MediaTypeAudioBook})

	if base.Title != "New" {
		t.Errorf("Title = %q, want New", base.Title)
	}
	if base.Artist != "Keep-me" {
		t.Errorf("Artist clobbered: %q", base.Artist)
	}
	if base.MediaType != MediaTypeAudioBook {
		t.Errorf("MediaType = %d, want %d", base.MediaType, MediaTypeAudioBook)
	}
}

func TestMergeOverwrite_LeavesFieldsAloneWhenOtherIsZero(t *testing.T) {
	base := Tag{Title: "Keep", Year: 2020}
	base.MergeOverwrite(Tag{})

	if base.Title != "Keep" || base.Year != 2020 {
		t.Errorf("fields changed: %+v", base)
	}
}

func TestMergeOverwrite_PurchaseDate(t *testing.T) {
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	fresh := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	base := Tag{PurchaseDate: old}
	base.MergeOverwrite(Tag{PurchaseDate: fresh})

	if !base.PurchaseDate.Equal(fresh) {
		t.Errorf("PurchaseDate = %v, want %v", base.PurchaseDate, fresh)
	}
}

func TestMergeOverwrite_ExtraMapMergesAndCreates(t *testing.T) {
	base := Tag{}
	base.MergeOverwrite(Tag{Extra: map[string]string{"asin": "X"}})

	if !reflect.DeepEqual(base.Extra, map[string]string{"asin": "X"}) {
		t.Errorf("Extra = %v", base.Extra)
	}
}
