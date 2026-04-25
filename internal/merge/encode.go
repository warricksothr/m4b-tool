package merge

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
)

// encodeAll transcodes each input into a uniform intermediate file in
// tmpDir, returning the encoded part paths in input order. Concurrency
// is bounded by jobs (1 = sequential). When opts.TrimSilenceStart or
// TrimSilenceEnd is true at the call site, encodeAll only applies the
// trim to MIDDLE files: the first input keeps its leading silence
// (typically intro music), and the last input keeps its trailing
// silence (outro). This matches spec/cli-surface.md §merge:
// "Apply --trim-silence except on boundary files."
//
// On any per-file error, the context is cancelled to wind down the
// other workers; the first error encountered is returned.
func encodeAll(ctx context.Context, ff *ffmpeg.Client, inputs []string, tmpDir string, jobs int, opts ffmpeg.EncodeOptions, verbose bool, stderr io.Writer) ([]string, error) {
	if jobs < 1 {
		jobs = 1
	}
	parts := make([]string, len(inputs))

	type job struct {
		idx int
		in  string
	}
	jobsCh := make(chan job)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	recordErr := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	total := len(inputs)
	worker := func() {
		defer wg.Done()
		for j := range jobsCh {
			outPath := filepath.Join(tmpDir, fmt.Sprintf("part-%04d.m4a", j.idx))

			// Per-input "starting" line. With concurrent workers
			// these can interleave, but each line names its own
			// 1-based position so the user can still tell what's
			// running.
			_, _ = fmt.Fprintf(stderr, "[%d/%d] %s -> %s\n", j.idx+1, total, j.in, outPath)

			perOpts := opts
			isFirst := j.idx == 0
			isLast := j.idx == total-1
			if isFirst || isLast {
				perOpts.TrimSilenceStart = false
				perOpts.TrimSilenceEnd = false
			}

			started := time.Now()
			if err := ff.Transcode(ctx, j.in, outPath, perOpts); err != nil {
				recordErr(err)
				return
			}
			parts[j.idx] = outPath
			if verbose {
				_, _ = fmt.Fprintf(stderr, "[%d/%d] done in %s\n", j.idx+1, total, time.Since(started).Round(100*time.Millisecond))
			}
		}
	}

	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go worker()
	}

dispatch:
	for i, in := range inputs {
		select {
		case <-ctx.Done():
			break dispatch
		case jobsCh <- job{idx: i, in: in}:
		}
	}
	close(jobsCh)
	wg.Wait()

	if firstErr != nil {
		// Best-effort cleanup of any partial outputs the caller
		// would otherwise have to clean up itself.
		for _, p := range parts {
			if p != "" {
				_ = os.Remove(p)
			}
		}
		return nil, firstErr
	}
	return parts, nil
}
