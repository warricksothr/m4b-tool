package split

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// DefaultFilenameTemplate is the default for --filename-template.
//
// PHP's m4b-tool defaulted to a Twig template
// (`{{"%03d"|format(track)}}-{{title|raw}}`); v1 of the Go port
// substitutes Go's text/template, so the variables are the same but
// the syntax differs. spec/cli-surface.md flags this as a breaking
// change from the PHP tool.
const DefaultFilenameTemplate = `{{printf "%03d" .Track}}-{{.Title}}`

// TemplateData is the value passed to a filename template. The fields
// are only those that are reasonable to put in a filename — long
// description, comment, etc. are deliberately omitted.
type TemplateData struct {
	Track       int
	TrackTotal  int
	Title       string
	Album       string
	Artist      string
	AlbumArtist string
	Genre       string
	Writer      string
	Year        int
	Series      string
	SeriesPart  string
}

// CompileFilenameTemplate parses the user-provided template (or the
// default, if empty) into something callable. Returned errors are
// user-readable; orchestrators should not wrap them further.
func CompileFilenameTemplate(s string) (*template.Template, error) {
	if strings.TrimSpace(s) == "" {
		s = DefaultFilenameTemplate
	}
	t, err := template.New("filename").Parse(s)
	if err != nil {
		return nil, fmt.Errorf("filename template: %w", err)
	}
	return t, nil
}

// Render executes the template with d, sanitizes the result for use
// as a filename component (no path separators or control chars), and
// returns it. Empty results fall back to the rendered Track number.
func Render(t *template.Template, d TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("filename template: %w", err)
	}
	out := SanitizeFilename(buf.String())
	if out == "" {
		out = fmt.Sprintf("%03d", d.Track)
	}
	return out, nil
}

// SanitizeFilename strips characters that are unsafe in filenames on
// any of the platforms we support, and trims leading/trailing dots and
// whitespace. Replacement is "_" so the output is always non-empty
// when the input was non-empty (modulo the leading/trailing trim).
func SanitizeFilename(s string) string {
	const unsafe = `/\:*?"<>|`
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || strings.IndexByte(unsafe, c) >= 0 {
			b = append(b, '_')
			continue
		}
		b = append(b, c)
	}
	return strings.Trim(string(b), " .")
}
