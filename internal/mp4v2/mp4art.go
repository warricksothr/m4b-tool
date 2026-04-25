package mp4v2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/warricksothr/m4b-tool/internal/exec"
)

// AddCover embeds imagePath as the single cover atom on audioPath via
// `mp4art --add imagePath audioPath`. mp4art replaces any existing
// cover when --add is used, but to be explicit about intent callers
// typically invoke [Client.RemoveCover] first when overwriting.
func (c *Client) AddCover(ctx context.Context, audioPath, imagePath string) error {
	if _, err := exec.Run(ctx, exec.Cmd{
		Name: c.MP4Art,
		Args: []string{"--add", imagePath, audioPath},
	}); err != nil {
		return fmt.Errorf("mp4art --add: %w", err)
	}
	return nil
}

// RemoveCover strips every cover atom from audioPath.
func (c *Client) RemoveCover(ctx context.Context, audioPath string) error {
	if _, err := exec.Run(ctx, exec.Cmd{
		Name: c.MP4Art,
		Args: []string{"--remove", "--art-any", audioPath},
	}); err != nil {
		return fmt.Errorf("mp4art --remove: %w", err)
	}
	return nil
}

// ExtractCover writes the cover at the given index to a sidecar file
// next to audioPath. mp4art picks the extension (.jpg or .png) based on
// the atom's content type; the final path is returned so callers can
// locate the written file without guessing at the extension.
//
// Returns an empty string with no error when audioPath has no covers.
func (c *Client) ExtractCover(ctx context.Context, audioPath string, index int) (string, error) {
	// mp4art writes `<basename-no-ext>.art[<index>].<ext>` where ext
	// is chosen from the atom's content type (.jpg, .png, .gif, .bmp,
	// .dat). The square brackets are glob metacharacters, so we scan
	// the directory instead of using filepath.Glob.
	if _, err := exec.Run(ctx, exec.Cmd{
		Name: c.MP4Art,
		Args: []string{"--art-index", strconv.Itoa(index), "--extract", audioPath},
	}); err != nil {
		return "", fmt.Errorf("mp4art --extract: %w", err)
	}

	dir := filepath.Dir(audioPath)
	base := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	prefix := fmt.Sprintf("%s.art[%d].", base, index)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("mp4art extract readdir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", nil
}

// reMp4ArtListRow matches one row of mp4art --list output, which has
// columns IDX BYTES CRC32 TYPE FILE. We only need to know how many rows
// came out after the header separator.
var reMp4ArtListRow = regexp.MustCompile(`^\s*\d+\s+\d+\s+[0-9a-fA-F]+\s+\S+`)

// ListCovers returns the number of cover atoms present in audioPath.
// Uses `mp4art --list` and counts data rows in its tabular output.
func (c *Client) ListCovers(ctx context.Context, audioPath string) (int, error) {
	res, err := exec.Run(ctx, exec.Cmd{
		Name: c.MP4Art,
		Args: []string{"--list", audioPath},
	})
	if err != nil {
		return 0, fmt.Errorf("mp4art --list: %w", err)
	}
	return countMp4ArtListRows(string(res.Stdout)), nil
}

func countMp4ArtListRows(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if reMp4ArtListRow.MatchString(line) {
			n++
		}
	}
	return n
}
