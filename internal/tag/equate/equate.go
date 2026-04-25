// Package equate implements the `equate` tag importer: propagates one
// field's value across a list of target fields. Driven by --equate CSV
// specs. See spec/tag-importers/equate.md.
package equate

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// Options configures the importer.
type Options struct {
	// Specs is a list of comma-separated field names. Each spec's first
	// name is the source; the rest are overwrite targets. Example:
	//   "artist,albumartist,sortartist"
	Specs []string
	// Logger is called once per invalid field reference. When nil,
	// stdlib log is used.
	Logger func(format string, args ...any)
}

// Importer copies tag field values across user-specified aliases.
type Importer struct{ opts Options }

// New returns an importer with the given options.
func New(opts Options) *Importer { return &Importer{opts: opts} }

// Name implements tag.Importer.
func (*Importer) Name() string { return "equate" }

// Improve applies each --equate spec in order, unconditionally
// overwriting the named target fields with the source field's value.
// Unknown field names emit a warning via opts.Logger and are skipped.
func (i *Importer) Improve(_ context.Context, tag audio.Tag, _ string) (audio.Tag, error) {
	warn := i.opts.Logger
	if warn == nil {
		warn = func(f string, args ...any) { log.Printf(f, args...) }
	}
	for _, spec := range i.opts.Specs {
		parts := splitAndTrim(spec)
		if len(parts) < 2 {
			continue
		}
		srcName := parts[0]
		srcVal, ok := getField(tag, srcName)
		if !ok {
			warn("equate: unknown source field %q", srcName)
			continue
		}
		if srcVal == "" {
			continue
		}
		for _, tgt := range parts[1:] {
			if !setField(&tag, tgt, srcVal) {
				warn("equate: unknown target field %q", tgt)
			}
		}
	}
	return tag, nil
}

func splitAndTrim(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// normalizeFieldName lowercases and strips non-alphanumeric characters
// so "album-artist", "album_artist", "AlbumArtist" all normalize to
// "albumartist".
func normalizeFieldName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// getField returns the string representation of a Tag field by the
// user-friendly name. Numeric fields are formatted as decimal strings;
// PurchaseDate as ISO date. Returns (_, false) for unknown names.
func getField(t audio.Tag, name string) (string, bool) {
	switch normalizeFieldName(name) {
	case "title":
		return t.Title, true
	case "album":
		return t.Album, true
	case "artist":
		return t.Artist, true
	case "albumartist":
		return t.AlbumArtist, true
	case "writer", "composer":
		return t.Writer, true
	case "genre":
		return t.Genre, true
	case "description":
		return t.Description, true
	case "longdescription", "longdesc":
		return t.LongDescription, true
	case "comment":
		return t.Comment, true
	case "copyright":
		return t.Copyright, true
	case "publisher":
		return t.Publisher, true
	case "encoder":
		return t.Encoder, true
	case "encodedby":
		return t.EncodedBy, true
	case "grouping":
		return t.Grouping, true
	case "series":
		return t.Series, true
	case "seriespart":
		return t.SeriesPart, true
	case "sorttitle", "sortname":
		return t.SortTitle, true
	case "sortalbum":
		return t.SortAlbum, true
	case "sortartist":
		return t.SortArtist, true
	case "sortalbumartist":
		return t.SortAlbumArtist, true
	case "sortwriter", "sortcomposer":
		return t.SortWriter, true
	case "performer":
		return t.Performer, true
	case "language":
		return t.Language, true
	case "lyrics":
		return t.Lyrics, true
	case "year", "date":
		if t.Year == 0 {
			return "", true
		}
		return strconv.Itoa(t.Year), true
	case "track":
		if t.Track == 0 {
			return "", true
		}
		return strconv.Itoa(t.Track), true
	case "tracks":
		if t.Tracks == 0 {
			return "", true
		}
		return strconv.Itoa(t.Tracks), true
	case "disk", "disc":
		if t.Disk == 0 {
			return "", true
		}
		return strconv.Itoa(t.Disk), true
	case "disks", "discs":
		if t.Disks == 0 {
			return "", true
		}
		return strconv.Itoa(t.Disks), true
	case "purchasedate":
		if t.PurchaseDate.IsZero() {
			return "", true
		}
		return t.PurchaseDate.UTC().Format("2006-01-02"), true
	}
	return "", false
}

// setField assigns a value to a Tag field by user-friendly name.
// Returns false when the name is unknown. Numeric fields parse the
// value and silently set 0 on parse failure (mirrors the spec guidance
// that equate should not error on field type mismatch).
func setField(t *audio.Tag, name, value string) bool {
	switch normalizeFieldName(name) {
	case "title":
		t.Title = value
	case "album":
		t.Album = value
	case "artist":
		t.Artist = value
	case "albumartist":
		t.AlbumArtist = value
	case "writer", "composer":
		t.Writer = value
	case "genre":
		t.Genre = value
	case "description":
		t.Description = value
	case "longdescription", "longdesc":
		t.LongDescription = value
	case "comment":
		t.Comment = value
	case "copyright":
		t.Copyright = value
	case "publisher":
		t.Publisher = value
	case "encoder":
		t.Encoder = value
	case "encodedby":
		t.EncodedBy = value
	case "grouping":
		t.Grouping = value
	case "series":
		t.Series = value
	case "seriespart":
		t.SeriesPart = value
	case "sorttitle", "sortname":
		t.SortTitle = value
	case "sortalbum":
		t.SortAlbum = value
	case "sortartist":
		t.SortArtist = value
	case "sortalbumartist":
		t.SortAlbumArtist = value
	case "sortwriter", "sortcomposer":
		t.SortWriter = value
	case "performer":
		t.Performer = value
	case "language":
		t.Language = value
	case "lyrics":
		t.Lyrics = value
	case "year", "date":
		t.Year = parseIntOrZero(value)
	case "track":
		t.Track = parseIntOrZero(value)
	case "tracks":
		t.Tracks = parseIntOrZero(value)
	case "disk", "disc":
		t.Disk = parseIntOrZero(value)
	case "disks", "discs":
		t.Disks = parseIntOrZero(value)
	case "purchasedate":
		if d, err := time.Parse("2006-01-02", value); err == nil {
			t.PurchaseDate = d
		}
	default:
		return false
	}
	return true
}

func parseIntOrZero(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
