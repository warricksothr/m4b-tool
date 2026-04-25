package merge

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// Batch placeholder semantics.
//
// A pattern is a slash-delimited template applied to a directory's
// path relative to the batch root (or to --batch-pattern-path when
// given). Each placeholder consumes one or more non-slash characters
// and binds the matched substring to a tag field.
//
// Recognized placeholders:
//
//	%a  Artist        %g  Genre
//	%n  Title (Name)  %t  Album
//	%s  Series        %p  SeriesPart
//	%%  literal '%'
//
// Everything else in the pattern is treated as a literal that must
// match byte-for-byte (regex-escaped). The pattern is anchored at both
// ends — partial matches don't count.
//
// The placeholder set is a deliberate subset of the PHP tool's; new
// placeholders should be added here and documented in
// spec/cli-surface.md.
var placeholderField = map[byte]string{
	'a': "Artist",
	'n': "Title",
	'g': "Genre",
	's': "Series",
	'p': "SeriesPart",
	't': "Album",
}

// BatchPattern is a compiled --batch-pattern.
type BatchPattern struct {
	Raw    string
	regex  *regexp.Regexp
	fields []string // capture-group index → tag field name
}

// ParseBatchPattern compiles a --batch-pattern string. Returns an
// error on unknown placeholder, dangling '%', or empty input.
func ParseBatchPattern(pat string) (*BatchPattern, error) {
	if pat == "" {
		return nil, errors.New("batch pattern: empty")
	}
	// Trim a single leading and trailing slash so users can write
	// either "%a/%n" or "%a/%n/" interchangeably.
	pat = strings.TrimSuffix(pat, "/")

	var (
		buf    strings.Builder
		fields []string
	)
	buf.WriteByte('^')
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		if c != '%' {
			if c == '/' {
				buf.WriteString(string(filepath.Separator))
			} else {
				buf.WriteString(regexp.QuoteMeta(string(c)))
			}
			continue
		}
		if i+1 >= len(pat) {
			return nil, fmt.Errorf("batch pattern %q: dangling %% at end", pat)
		}
		next := pat[i+1]
		i++
		if next == '%' {
			buf.WriteString("%")
			continue
		}
		field, ok := placeholderField[next]
		if !ok {
			return nil, fmt.Errorf("batch pattern %q: unknown placeholder %%%c", pat, next)
		}
		// One or more non-separator characters, lazily, so adjacent
		// placeholders separated by literals (e.g., "%p - %n") split
		// at the literal rather than greedily eating it.
		buf.WriteString(`([^`)
		buf.WriteString(regexp.QuoteMeta(string(filepath.Separator)))
		buf.WriteString(`]+?)`)
		fields = append(fields, field)
	}
	buf.WriteByte('$')

	re, err := regexp.Compile(buf.String())
	if err != nil {
		return nil, fmt.Errorf("batch pattern %q: %w", pat, err)
	}
	return &BatchPattern{Raw: pat, regex: re, fields: fields}, nil
}

// Match attempts to apply this pattern to a slash-normalized relative
// path. The second return value reports whether the path matched.
func (p *BatchPattern) Match(rel string) (audio.Tag, bool) {
	rel = filepath.FromSlash(rel)
	m := p.regex.FindStringSubmatch(rel)
	if m == nil {
		return audio.Tag{}, false
	}
	var t audio.Tag
	for i, val := range m[1:] {
		setTagField(&t, p.fields[i], val)
	}
	return t, true
}

// setTagField writes val into the named field of t. The set is small
// enough that an explicit switch is clearer (and faster) than reflection.
func setTagField(t *audio.Tag, field, val string) {
	switch field {
	case "Title":
		t.Title = val
	case "Album":
		t.Album = val
	case "Artist":
		t.Artist = val
	case "Genre":
		t.Genre = val
	case "Series":
		t.Series = val
	case "SeriesPart":
		t.SeriesPart = val
	}
}

// BatchEntry is one merge job enumerated from a batch run.
type BatchEntry struct {
	// SourceDir is the absolute path to the matched directory.
	SourceDir string
	// RelPath is the path that matched the pattern (slash-normalized).
	RelPath string
	// Tags carries the values extracted from placeholders.
	Tags audio.Tag
}

// EnumerateOptions configures EnumerateBatch.
type EnumerateOptions struct {
	// Patterns is the list of compiled patterns; the first matching
	// pattern wins for any given directory.
	Patterns []*BatchPattern
	// TrimBase, when non-empty, is the path against which directory
	// paths are made relative before pattern matching. Falls back to
	// the input root.
	TrimBase string
	// Extensions filters which files count as "audio" when deciding
	// whether a directory is a candidate. When empty, DefaultExtensions
	// applies.
	Extensions []string
	// Filter is a substring; directories whose RelPath does not
	// contain it are skipped. Empty means no filter.
	Filter string
	// Resume is a set of already-completed SourceDir paths; matching
	// entries are excluded from the result.
	Resume map[string]bool
}

// EnumerateBatch walks each root, finds directories that contain at
// least one audio file, tries to match each candidate's relative path
// against the configured patterns, and returns one BatchEntry per
// successful match.
//
// Results are sorted by RelPath for stable output. Directories whose
// relative path does not match any pattern are silently skipped — that
// is by design; patterns are filters as well as extractors.
func EnumerateBatch(roots []string, opts EnumerateOptions) ([]BatchEntry, error) {
	if len(opts.Patterns) == 0 {
		return nil, errors.New("batch: at least one pattern required")
	}
	exts := opts.Extensions
	if len(exts) == 0 {
		exts = DefaultExtensions
	}
	extSet := make(map[string]bool, len(exts))
	for _, e := range exts {
		extSet["."+strings.ToLower(strings.TrimPrefix(e, "."))] = true
	}

	var entries []BatchEntry
	seen := make(map[string]bool)

	for _, root := range roots {
		base := opts.TrimBase
		if base == "" {
			base = root
		}
		baseAbs, err := filepath.Abs(base)
		if err != nil {
			return nil, fmt.Errorf("batch base %q: %w", base, err)
		}

		// candidates[absDir] = true iff the directory contains audio.
		candidates := make(map[string]bool)
		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !extSet[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			abs, err := filepath.Abs(filepath.Dir(path))
			if err != nil {
				return err
			}
			candidates[abs] = true
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("batch walk %q: %w", root, walkErr)
		}

		for abs := range candidates {
			if seen[abs] {
				continue
			}
			rel, err := filepath.Rel(baseAbs, abs)
			if err != nil || strings.HasPrefix(rel, "..") {
				// Directory is outside the base; the pattern can't
				// reach it, so skip rather than try to match the
				// absolute path.
				continue
			}
			relSlash := filepath.ToSlash(rel)
			if opts.Filter != "" && !strings.Contains(relSlash, opts.Filter) {
				continue
			}
			if opts.Resume[abs] {
				continue
			}

			for _, p := range opts.Patterns {
				if t, ok := p.Match(rel); ok {
					entries = append(entries, BatchEntry{
						SourceDir: abs,
						RelPath:   relSlash,
						Tags:      t,
					})
					seen[abs] = true
					break
				}
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].RelPath < entries[j].RelPath
	})
	return entries, nil
}

// LoadResumeFile reads a resume file into a set of absolute paths.
// A missing file is treated as an empty set; other I/O errors bubble
// up. Blank lines and lines starting with '#' are ignored, matching
// the convention used by other shell-style state files.
func LoadResumeFile(path string) (map[string]bool, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("read resume file %q: %w", path, err)
	}
	out := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[line] = true
	}
	return out, nil
}

// AppendResume opens the resume file in append mode and writes one
// path per call (with trailing newline). Used after a successful
// per-entry merge so an interrupted run can be resumed.
func AppendResume(path, entry string) error {
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open resume file %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := fmt.Fprintln(f, entry); err != nil {
		return fmt.Errorf("write resume file %q: %w", path, err)
	}
	return nil
}
