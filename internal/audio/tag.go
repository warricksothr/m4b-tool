package audio

import "time"

// ItunesMediaType names the iTunes `stik` atom integer codes. The
// m4b-tool-specific extensions (Book, Ebook) are non-standard iTunes
// values but are accepted by iTunes and most audiobook players.
type ItunesMediaType int

const (
	MediaTypeUnset     ItunesMediaType = 0
	MediaTypeMusic     ItunesMediaType = 1
	MediaTypeAudioBook ItunesMediaType = 2
	MediaTypeMovie     ItunesMediaType = 9
	MediaTypeTVShow    ItunesMediaType = 10
	MediaTypeBooklet   ItunesMediaType = 11
	MediaTypeRingtone  ItunesMediaType = 14
	MediaTypePodcast   ItunesMediaType = 21
	MediaTypeITunesU   ItunesMediaType = 23

	// m4b-tool extensions, not standard iTunes values.
	MediaTypeBook  ItunesMediaType = 256
	MediaTypeEbook ItunesMediaType = 257
)

// Tag is the in-memory metadata bag. All fields are optional; the zero
// value of each field means "unset" for merge purposes.
// See spec/data-model.md §Tag.
type Tag struct {
	// Identity
	Title       string
	Album       string
	Artist      string
	AlbumArtist string
	Writer      string

	// Descriptive
	Genre           string
	Year            int
	Description     string
	LongDescription string
	Comment         string
	Copyright       string
	Publisher       string
	Encoder         string
	EncodedBy       string

	// Ordinals
	Track  int
	Tracks int
	Disk   int
	Disks  int

	// Series (pseudo-tags; stored in grouping on MP4 via a later pass)
	Series     string
	SeriesPart string

	// Sort keys
	SortTitle       string
	SortAlbum       string
	SortArtist      string
	SortAlbumArtist string
	SortWriter      string

	// MP3-specific (unused when writing MP4)
	Performer string
	Language  string
	Lyrics    string

	// iTunes
	MediaType    ItunesMediaType
	Grouping     string
	PurchaseDate time.Time

	// Cover
	CoverPath string

	// Chapters (transient; not a tag atom)
	Chapters []Chapter

	// Arbitrary extras — ASIN, ISBN, Audible ID, Overdrive markers, etc.
	Extra map[string]string

	// Properties to strip on write
	Remove []string
}

// MergeMissing fills in fields of t from other wherever t's field is zero.
// Used during tag-importer composition when a lower-priority source
// should only supply values the higher-priority source did not provide.
func (t *Tag) MergeMissing(other Tag) {
	if t.Title == "" {
		t.Title = other.Title
	}
	if t.Album == "" {
		t.Album = other.Album
	}
	if t.Artist == "" {
		t.Artist = other.Artist
	}
	if t.AlbumArtist == "" {
		t.AlbumArtist = other.AlbumArtist
	}
	if t.Writer == "" {
		t.Writer = other.Writer
	}
	if t.Genre == "" {
		t.Genre = other.Genre
	}
	if t.Year == 0 {
		t.Year = other.Year
	}
	if t.Description == "" {
		t.Description = other.Description
	}
	if t.LongDescription == "" {
		t.LongDescription = other.LongDescription
	}
	if t.Comment == "" {
		t.Comment = other.Comment
	}
	if t.Copyright == "" {
		t.Copyright = other.Copyright
	}
	if t.Publisher == "" {
		t.Publisher = other.Publisher
	}
	if t.Encoder == "" {
		t.Encoder = other.Encoder
	}
	if t.EncodedBy == "" {
		t.EncodedBy = other.EncodedBy
	}
	if t.Track == 0 {
		t.Track = other.Track
	}
	if t.Tracks == 0 {
		t.Tracks = other.Tracks
	}
	if t.Disk == 0 {
		t.Disk = other.Disk
	}
	if t.Disks == 0 {
		t.Disks = other.Disks
	}
	if t.Series == "" {
		t.Series = other.Series
	}
	if t.SeriesPart == "" {
		t.SeriesPart = other.SeriesPart
	}
	if t.SortTitle == "" {
		t.SortTitle = other.SortTitle
	}
	if t.SortAlbum == "" {
		t.SortAlbum = other.SortAlbum
	}
	if t.SortArtist == "" {
		t.SortArtist = other.SortArtist
	}
	if t.SortAlbumArtist == "" {
		t.SortAlbumArtist = other.SortAlbumArtist
	}
	if t.SortWriter == "" {
		t.SortWriter = other.SortWriter
	}
	if t.Performer == "" {
		t.Performer = other.Performer
	}
	if t.Language == "" {
		t.Language = other.Language
	}
	if t.Lyrics == "" {
		t.Lyrics = other.Lyrics
	}
	if t.MediaType == MediaTypeUnset {
		t.MediaType = other.MediaType
	}
	if t.Grouping == "" {
		t.Grouping = other.Grouping
	}
	if t.PurchaseDate.IsZero() {
		t.PurchaseDate = other.PurchaseDate
	}
	if t.CoverPath == "" {
		t.CoverPath = other.CoverPath
	}
	if len(t.Chapters) == 0 {
		t.Chapters = append(t.Chapters[:0], other.Chapters...)
	}
	if len(t.Remove) == 0 {
		t.Remove = append(t.Remove[:0], other.Remove...)
	}
	for k, v := range other.Extra {
		if _, exists := t.Extra[k]; exists {
			continue
		}
		if t.Extra == nil {
			t.Extra = map[string]string{}
		}
		t.Extra[k] = v
	}
}

// MergeOverwrite replaces fields of t with values from other wherever
// other's field is non-zero.
func (t *Tag) MergeOverwrite(other Tag) {
	if other.Title != "" {
		t.Title = other.Title
	}
	if other.Album != "" {
		t.Album = other.Album
	}
	if other.Artist != "" {
		t.Artist = other.Artist
	}
	if other.AlbumArtist != "" {
		t.AlbumArtist = other.AlbumArtist
	}
	if other.Writer != "" {
		t.Writer = other.Writer
	}
	if other.Genre != "" {
		t.Genre = other.Genre
	}
	if other.Year != 0 {
		t.Year = other.Year
	}
	if other.Description != "" {
		t.Description = other.Description
	}
	if other.LongDescription != "" {
		t.LongDescription = other.LongDescription
	}
	if other.Comment != "" {
		t.Comment = other.Comment
	}
	if other.Copyright != "" {
		t.Copyright = other.Copyright
	}
	if other.Publisher != "" {
		t.Publisher = other.Publisher
	}
	if other.Encoder != "" {
		t.Encoder = other.Encoder
	}
	if other.EncodedBy != "" {
		t.EncodedBy = other.EncodedBy
	}
	if other.Track != 0 {
		t.Track = other.Track
	}
	if other.Tracks != 0 {
		t.Tracks = other.Tracks
	}
	if other.Disk != 0 {
		t.Disk = other.Disk
	}
	if other.Disks != 0 {
		t.Disks = other.Disks
	}
	if other.Series != "" {
		t.Series = other.Series
	}
	if other.SeriesPart != "" {
		t.SeriesPart = other.SeriesPart
	}
	if other.SortTitle != "" {
		t.SortTitle = other.SortTitle
	}
	if other.SortAlbum != "" {
		t.SortAlbum = other.SortAlbum
	}
	if other.SortArtist != "" {
		t.SortArtist = other.SortArtist
	}
	if other.SortAlbumArtist != "" {
		t.SortAlbumArtist = other.SortAlbumArtist
	}
	if other.SortWriter != "" {
		t.SortWriter = other.SortWriter
	}
	if other.Performer != "" {
		t.Performer = other.Performer
	}
	if other.Language != "" {
		t.Language = other.Language
	}
	if other.Lyrics != "" {
		t.Lyrics = other.Lyrics
	}
	if other.MediaType != MediaTypeUnset {
		t.MediaType = other.MediaType
	}
	if other.Grouping != "" {
		t.Grouping = other.Grouping
	}
	if !other.PurchaseDate.IsZero() {
		t.PurchaseDate = other.PurchaseDate
	}
	if other.CoverPath != "" {
		t.CoverPath = other.CoverPath
	}
	if len(other.Chapters) > 0 {
		t.Chapters = append(t.Chapters[:0], other.Chapters...)
	}
	if len(other.Remove) > 0 {
		t.Remove = append(t.Remove[:0], other.Remove...)
	}
	for k, v := range other.Extra {
		if t.Extra == nil {
			t.Extra = map[string]string{}
		}
		t.Extra[k] = v
	}
}
