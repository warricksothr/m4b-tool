package audio

import "time"

// Silence is a detected silent region in an audio stream.
// See spec/data-model.md §Silence.
type Silence struct {
	Start          time.Duration
	Length         time.Duration
	IsChapterStart bool
}

// End returns the silence's end offset.
func (s Silence) End() time.Duration { return s.Start + s.Length }

// Midpoint returns the offset halfway between Start and End. Used by
// chapter-to-silence alignment so a chapter boundary snaps to the middle
// of a silent gap rather than an edge.
func (s Silence) Midpoint() time.Duration { return s.Start + s.Length/2 }
