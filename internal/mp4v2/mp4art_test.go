package mp4v2

import "testing"

func TestCountMp4ArtListRows_Empty(t *testing.T) {
	// mp4art prints "IDX  BYTES  CRC32  TYPE  FILE" header lines and
	// separator dashes even with no covers.
	in := `IDX  BYTES     CRC32     TYPE   FILE
---  --------  --------  -----  -----
`
	if got := countMp4ArtListRows(in); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestCountMp4ArtListRows_TwoCovers(t *testing.T) {
	in := `IDX  BYTES     CRC32     TYPE   FILE
---  --------  --------  -----  ---------------
  0    12345   abcd1234  image  fixture.m4b
  1     9876   beefface  image  fixture.m4b
`
	if got := countMp4ArtListRows(in); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}
