package audio

import "time"

// Chapter is a time-bounded named section of an audio stream.
// See spec/data-model.md §Chapter.
type Chapter struct {
	Start        time.Duration
	Length       time.Duration
	Name         string
	Introduction string
}

// End returns the chapter's end offset (Start + Length).
func (c Chapter) End() time.Duration { return c.Start + c.Length }

// Reserved chapter names. These are synthetic and matched by string
// equality in normalization passes.
const (
	ChapterIntro = "Intro"
	ChapterOutro = "Outro"
)
