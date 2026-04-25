package mp4v2

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/exec"
)

// WriteTags writes the non-cover, non-chapter portion of tag onto the
// MP4 file at path via mp4tags. Fields that are zero are skipped.
// Sort-name and purchase-date fields are only written when the
// underlying binary was detected to support them.
//
// Cover (covr) goes through mp4art; chapters go through mp4chaps -i.
// Callers should invoke WriteTags before those two per
// spec/metadata-mapping.md §Write order (MP4 outputs).
func (c *Client) WriteTags(ctx context.Context, path string, tag *audio.Tag) error {
	args := buildWriteTagsArgs(tag, c.SupportsSortNames, c.SupportsPurchaseDate)
	if len(args) == 0 {
		return nil
	}
	args = append(args, path)
	if _, err := exec.Run(ctx, exec.Cmd{Name: c.MP4Tags, Args: args}); err != nil {
		return fmt.Errorf("mp4tags: %w", err)
	}
	return nil
}

// RemoveTags runs `mp4tags -r "<flags>" path`. flags is a list of the
// single-character atoms to remove (e.g. "a", "s", "A"). An empty list
// is a no-op. See the spec's Property → flag mapping.
func (c *Client) RemoveTags(ctx context.Context, path string, flags []string) error {
	if len(flags) == 0 {
		return nil
	}
	if _, err := exec.Run(ctx, exec.Cmd{
		Name: c.MP4Tags,
		Args: []string{"-r", strings.Join(flags, ","), path},
	}); err != nil {
		return fmt.Errorf("mp4tags -r: %w", err)
	}
	return nil
}

// buildWriteTagsArgs assembles the argv (without the trailing file
// path) from a Tag. Pure function so the mapping is unit-testable
// without touching the binary.
func buildWriteTagsArgs(tag *audio.Tag, sortNames, purchaseDate bool) []string {
	if tag == nil {
		return nil
	}
	var args []string
	addStr := func(flag, value string) {
		if value == "" {
			return
		}
		args = append(args, flag, value)
	}
	addInt := func(flag string, value int) {
		if value == 0 {
			return
		}
		args = append(args, flag, strconv.Itoa(value))
	}

	// Order follows the spec mapping table; within a group,
	// alphabetical on the flag letter for stable diffs.
	addStr("-a", tag.Artist)
	addStr("-A", tag.Album)
	addStr("-c", tag.Comment)
	addStr("-C", tag.Copyright)
	addStr("-e", tag.EncodedBy)
	addStr("-E", tag.Encoder)
	addStr("-g", tag.Genre)
	addStr("-G", tag.Grouping)
	addStr("-l", tag.LongDescription)
	addStr("-L", tag.Lyrics)
	addStr("-m", tag.Description)
	addStr("-R", tag.AlbumArtist)
	addStr("-s", tag.Title)
	addStr("-w", tag.Writer)

	addInt("-d", tag.Disk)
	addInt("-D", tag.Disks)
	addInt("-t", tag.Track)
	addInt("-T", tag.Tracks)
	addInt("-y", tag.Year)

	// MediaType: iTunes "stik" atom. mp4tags accepts an integer string
	// in the -i argument and maps it internally.
	if tag.MediaType != audio.MediaTypeUnset {
		args = append(args, "-i", strconv.Itoa(int(tag.MediaType)))
	}

	// Feature-gated flags. The short forms (-f/-F/-k/-U) are the
	// sandreas-fork pattern; the long forms are the new-upstream
	// build we ship on this system. Either way we use the long form
	// which both builds accept when the feature is present.
	if sortNames {
		addStr("-sortname", tag.SortTitle)
		addStr("-sortalbum", tag.SortAlbum)
		addStr("-sortartist", tag.SortArtist)
		addStr("-sortalbumartist", tag.SortAlbumArtist)
		addStr("-sortcomposer", tag.SortWriter)
	}
	if purchaseDate && !tag.PurchaseDate.IsZero() {
		args = append(args, "-purchasedate", tag.PurchaseDate.UTC().Format("2006-01-02"))
	}
	return args
}
