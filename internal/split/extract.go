package split

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
	"github.com/warricksothr/m4b-tool/internal/mp4v2"
)

// buildExtractMetadata returns the ffmpeg -metadata key=value map for
// one chapter's output when the muxer isn't MP4-family. ffmpeg picks
// the on-disk frame format per muxer: ID3v2 for MP3, Vorbis comments
// for FLAC, etc. Only fields with broadly-supported standard mappings
// are included — series/series-part have no standard ID3 frame, so
// they're dropped here rather than written to a non-portable TXXX.
func buildExtractMetadata(base audio.Tag, ch audio.Chapter, track, total int) map[string]string {
	m := map[string]string{
		"track": fmt.Sprintf("%d/%d", track, total),
	}
	if ch.Name != "" {
		m["title"] = ch.Name
	} else if base.Title != "" {
		m["title"] = base.Title
	}
	if base.Artist != "" {
		m["artist"] = base.Artist
	}
	if base.Album != "" {
		m["album"] = base.Album
	}
	if base.AlbumArtist != "" {
		m["album_artist"] = base.AlbumArtist
	}
	if base.Genre != "" {
		m["genre"] = base.Genre
	}
	if base.Writer != "" {
		m["composer"] = base.Writer
	}
	if base.Year != 0 {
		m["date"] = strconv.Itoa(base.Year)
	}
	if base.Comment != "" {
		m["comment"] = base.Comment
	}
	if base.Copyright != "" {
		m["copyright"] = base.Copyright
	}
	return m
}

// splitJob is one chapter's worth of extraction work.
type splitJob struct {
	track, total int
	chapter      audio.Chapter
	out          string
}

// extractAll runs each splitJob through ffmpeg.ExtractSegment and
// (when mp is non-nil) mp4v2.WriteTags. Concurrency is bounded by
// jobs (1 = sequential). On any per-job error, the context is
// cancelled to wind down other workers; the first error is returned.
//
// Mirrors the worker-pool shape used by internal/merge/encode.go so
// the cancel/cleanup semantics are consistent across the two
// commands.
func extractAll(
	ctx context.Context,
	ff *ffmpeg.Client,
	mp *mp4v2.Client,
	input string,
	jobs []splitJob,
	extOpts ffmpeg.ExtractOptions,
	tagBase audio.Tag,
	jobCount int,
	stderr io.Writer,
) error {
	if jobCount < 1 {
		jobCount = 1
	}
	if jobCount > len(jobs) {
		jobCount = len(jobs)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobsCh := make(chan splitJob)

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
			// Log lines from concurrent workers can interleave; that's
			// fine — each line names its own track index, and order
			// across workers carries no meaning.
			_, _ = fmt.Fprintf(stderr, "[%d/%d] %s (%s+%s)\n", j.track, j.total, j.out, j.chapter.Start, j.chapter.Length)

			perOpts := extOpts
			if mp == nil {
				// Non-MP4 output: tag via ffmpeg -metadata in the same
				// extraction call. mp4-family outputs continue to be
				// post-tagged with mp4tags below.
				perOpts.Metadata = buildExtractMetadata(tagBase, j.chapter, j.track, j.total)
			}

			if err := ff.ExtractSegment(ctx, input, j.out, j.chapter.Start, j.chapter.Length, perOpts); err != nil {
				recordErr(err)
				return
			}
			if mp != nil {
				perTag := perTrackTag(tagBase, j.chapter, j.track, j.total)
				if err := mp.WriteTags(ctx, j.out, &perTag); err != nil {
					recordErr(fmt.Errorf("write tags %q: %w", j.out, err))
					return
				}
			}
		}
	}

	for i := 0; i < jobCount; i++ {
		wg.Add(1)
		go worker()
	}

dispatch:
	for _, j := range jobs {
		select {
		case <-ctx.Done():
			break dispatch
		case jobsCh <- j:
		}
	}
	close(jobsCh)
	wg.Wait()

	if firstErr != nil {
		// Best-effort cleanup of partial outputs from successful
		// workers, since the run as a whole failed.
		for _, j := range jobs {
			_ = os.Remove(j.out)
		}
		return firstErr
	}
	return nil
}
