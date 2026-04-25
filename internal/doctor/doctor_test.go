package doctor

import (
	"bytes"
	"strings"
	"testing"
)

func TestFirstNonEmptyLine(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"first stream wins", []string{"hello\nworld", "stderr"}, "hello"},
		{"skip blank prefix", []string{"\n\n  real line\n"}, "real line"},
		{"fall through to second stream", []string{"", "second"}, "second"},
		{"no content", []string{"", "  \n  "}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstNonEmptyLine(tc.in...); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRender_AllPresent(t *testing.T) {
	results := []Result{
		{Tool: Tool{Bin: "ffmpeg", Required: true, Purpose: "encode"}, Path: "/x/ffmpeg", Version: "ffmpeg 6.1.1"},
		{Tool: Tool{Bin: "tone", Required: false, Purpose: "atoms"}, Path: "/x/tone", Version: "tone 1.0"},
	}
	var buf bytes.Buffer
	ok := Render(&buf, results)
	if !ok {
		t.Errorf("expected ok=true")
	}
	out := buf.String()
	for _, want := range []string{"ffmpeg", "[required] OK", "/x/ffmpeg", "ffmpeg 6.1.1", "all required tools present"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRender_RequiredMissingFailsRun(t *testing.T) {
	results := []Result{
		{Tool: Tool{Bin: "ffmpeg", Required: true, Purpose: "encode"}}, // not found
		{Tool: Tool{Bin: "tone", Required: false, Purpose: "atoms"}},   // also not found, optional
	}
	var buf bytes.Buffer
	ok := Render(&buf, results)
	if ok {
		t.Errorf("expected ok=false when required tool missing")
	}
	if !strings.Contains(buf.String(), "MISSING") {
		t.Errorf("expected MISSING marker:\n%s", buf.String())
	}
}

func TestRender_OptionalMissingStillOk(t *testing.T) {
	results := []Result{
		{Tool: Tool{Bin: "ffmpeg", Required: true, Purpose: "encode"}, Path: "/x/ffmpeg"},
		{Tool: Tool{Bin: "tone", Required: false, Purpose: "atoms"}}, // optional, missing
	}
	var buf bytes.Buffer
	ok := Render(&buf, results)
	if !ok {
		t.Errorf("optional missing should still be ok")
	}
}
