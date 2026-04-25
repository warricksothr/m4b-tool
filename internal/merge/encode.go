package merge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

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
func encodeAll(ctx context.Context, ff *ffmpeg.Client, inputs []string, tmpDir string, jobs int, opts ffmpeg.EncodeOptions) ([]string, error) {
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

	worker := func() {
		defer wg.Done()
		for j := range jobsCh {
			outPath := filepath.Join(tmpDir, fmt.Sprintf("part-%04d.m4a", j.idx))

			perOpts := opts
			isFirst := j.idx == 0
			isLast := j.idx == len(inputs)-1
			if isFirst || isLast {
				perOpts.TrimSilenceStart = false
				perOpts.TrimSilenceEnd = false
			}

			if err := ff.Transcode(ctx, j.in, outPath, perOpts); err != nil {
				recordErr(err)
				return
			}
			parts[j.idx] = outPath
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
