package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/warricksothr/m4b-tool/internal/exec"
)

// ConcatOptions tunes a concat invocation. Overwrite is on by default
// because the orchestrator has already validated whether the user
// wanted to overwrite via --force.
type ConcatOptions struct {
	// Format, e.g. "m4b" or "mp4". Empty lets ffmpeg pick by extension.
	Format string
	// Overwrite passes "-y" so ffmpeg will clobber an existing output.
	Overwrite bool
}

// Concat runs `ffmpeg -f concat` over a list of pre-encoded input files,
// stream-copying them into out. Inputs must share codec parameters —
// this is the stream-copy "no-conversion" path. Callers that need
// transcoding should use the transcode/encode pipeline (M7b).
//
// inputs are resolved to absolute paths before being written into the
// concat list file, and single quotes in paths are escaped per ffmpeg's
// concat demuxer rules.
func (c *Client) Concat(ctx context.Context, inputs []string, out string, opts ConcatOptions) error {
	if len(inputs) == 0 {
		return fmt.Errorf("ffmpeg concat: no inputs")
	}
	listFile, err := writeConcatList(inputs)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(listFile) }()

	args := []string{
		"-hide_banner",
	}
	if opts.Overwrite {
		args = append(args, "-y")
	}
	args = append(args,
		"-f", "concat",
		"-safe", "0",
		"-vn",
		"-i", listFile,
		"-max_muxing_queue_size", "9999",
		"-c", "copy",
	)
	if opts.Format != "" {
		args = append(args, "-f", opts.Format)
	}
	args = append(args, out)

	_, err = exec.Run(ctx, exec.Cmd{Name: c.Bin, Args: args})
	if err != nil {
		return fmt.Errorf("ffmpeg concat: %w", err)
	}
	return nil
}

// writeConcatList creates a temp file with ffmpeg concat-demuxer
// syntax, one `file 'path'` line per input. Returns the temp file path;
// caller is responsible for removing it.
func writeConcatList(inputs []string) (string, error) {
	f, err := os.CreateTemp("", "m4btool-concat-*.txt")
	if err != nil {
		return "", fmt.Errorf("concat list: %w", err)
	}
	bw := bufio.NewWriter(f)
	for _, p := range inputs {
		abs, err := filepath.Abs(p)
		if err != nil {
			_ = f.Close()
			return "", fmt.Errorf("concat list: abs %q: %w", p, err)
		}
		if _, err := fmt.Fprintf(bw, "file '%s'\n", escapeConcatPath(abs)); err != nil {
			_ = f.Close()
			return "", err
		}
	}
	if err := bw.Flush(); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// escapeConcatPath quotes a single-quoted concat-demuxer path. Per
// ffmpeg docs, the only special character is the single quote itself,
// escaped as `'\”`.
func escapeConcatPath(p string) string {
	if !strings.ContainsRune(p, '\'') {
		return p
	}
	return strings.ReplaceAll(p, "'", `'\''`)
}
