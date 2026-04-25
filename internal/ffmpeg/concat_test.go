package ffmpeg

import "testing"

func TestEscapeConcatPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/a/b/c.m4a", "/a/b/c.m4a"},
		{"/a/with space/c.m4a", "/a/with space/c.m4a"},
		{"/a/it's/c.m4a", `/a/it'\''s/c.m4a`},
		{`/a/many''quotes/d`, `/a/many'\'''\''quotes/d`},
	}
	for _, tc := range tests {
		if got := escapeConcatPath(tc.in); got != tc.want {
			t.Errorf("escapeConcatPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
