package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/exec"
)

// ffmetadataHeader is the magic first line ffmpeg expects. Writers always
// emit it; readers accept its absence for permissive interop with
// sidecar files users may hand-edit.
const ffmetadataHeader = ";FFMETADATA1"

// ParseFFMetadata parses an FFMETADATA1 document from r into a *audio.Tag.
// Unknown keys in the global section are preserved in Tag.Extra.
//
// The grammar (see spec/metadata-mapping.md) supports:
//   - `;` and `#` comment lines (both at start of line)
//   - `key=value` entries with escapes `\=`, `\;`, `\#`, `\\`, `\n`
//   - multi-line values via trailing backslash
//   - `[CHAPTER]` sections with TIMEBASE / START / END / title
//   - other bracketed sections (e.g. `[STREAM]`) which are skipped
func ParseFFMetadata(r io.Reader) (*audio.Tag, error) {
	tag := &audio.Tag{}
	scanner := bufio.NewScanner(r)
	// Descriptions may be very long; raise the default 64KB cap.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var (
		section    = "" // "" = global, "CHAPTER" etc.
		curChapter *partialChapter
	)

	flushChapter := func() error {
		if curChapter == nil {
			return nil
		}
		ch, err := curChapter.build()
		if err != nil {
			return err
		}
		tag.Chapters = append(tag.Chapters, ch)
		curChapter = nil
		return nil
	}

	for {
		line, ok, err := readLogicalLine(scanner)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}

		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "[") {
			if err := flushChapter(); err != nil {
				return nil, err
			}
			end := strings.Index(trimmed, "]")
			if end < 0 {
				return nil, fmt.Errorf("ffmetadata: unterminated section %q", trimmed)
			}
			section = strings.TrimSpace(trimmed[1:end])
			if strings.EqualFold(section, "CHAPTER") {
				curChapter = &partialChapter{Timebase: "1/1000"}
			}
			continue
		}

		rawKey, rawValue, ok := splitEscapedKV(line)
		if !ok {
			return nil, fmt.Errorf("ffmetadata: malformed line %q (no '=')", line)
		}
		key := strings.TrimSpace(unescape(rawKey))
		value := unescape(rawValue)

		switch {
		case curChapter != nil:
			curChapter.set(key, value)
		case section == "" || strings.EqualFold(section, "STREAM"):
			// Accept both global and [STREAM] keys into the Tag. ffmpeg
			// emits some audio tags inside [STREAM] blocks depending on
			// container — treating them as tag input is pragmatic.
			if err := assignGlobalKey(tag, key, value); err != nil {
				return nil, err
			}
		}
	}
	if err := flushChapter(); err != nil {
		return nil, err
	}

	return tag, nil
}

// WriteFFMetadata writes tag as FFMETADATA1 text to w. The output is
// deterministic for identical inputs: known keys appear in a fixed order
// and Extra keys are emitted alphabetically, so parse → write → parse is
// idempotent.
func WriteFFMetadata(w io.Writer, tag *audio.Tag) error {
	bw := bufio.NewWriter(w)
	if _, err := bw.WriteString(ffmetadataHeader + "\n"); err != nil {
		return err
	}

	writeKV := func(k, v string) error {
		if v == "" {
			return nil
		}
		_, err := fmt.Fprintf(bw, "%s=%s\n", escapeKey(k), escape(v))
		return err
	}
	writeIntKV := func(k string, v int) error {
		if v == 0 {
			return nil
		}
		_, err := fmt.Fprintf(bw, "%s=%d\n", escapeKey(k), v)
		return err
	}

	// Emit known fields in mapping-table order.
	pairs := []struct {
		key string
		val string
	}{
		{"title", tag.Title},
		{"album", tag.Album},
		{"artist", tag.Artist},
		{"album_artist", tag.AlbumArtist},
		{"composer", tag.Writer},
		{"genre", tag.Genre},
		{"description", tag.Description},
		{"longdesc", tag.LongDescription},
		{"comment", tag.Comment},
		{"copyright", tag.Copyright},
		{"publisher", tag.Publisher},
		{"encoder", tag.Encoder},
		{"encoded_by", tag.EncodedBy},
		{"grouping", tag.Grouping},
		{"title-sort", tag.SortTitle},
		{"album-sort", tag.SortAlbum},
		{"artist-sort", tag.SortArtist},
		{"TSO2", tag.SortAlbumArtist},
		{"TSOC", tag.SortWriter},
		{"performer", tag.Performer},
		{"language", tag.Language},
		{"lyrics", tag.Lyrics},
	}
	for _, p := range pairs {
		if err := writeKV(p.key, p.val); err != nil {
			return err
		}
	}

	ints := []struct {
		key string
		val int
	}{
		{"date", tag.Year},
		{"track", tag.Track},
		{"disc", tag.Disk},
	}
	for _, p := range ints {
		if err := writeIntKV(p.key, p.val); err != nil {
			return err
		}
	}

	if !tag.PurchaseDate.IsZero() {
		if _, err := fmt.Fprintf(bw, "purchase_date=%s\n", tag.PurchaseDate.UTC().Format("2006-01-02")); err != nil {
			return err
		}
	}

	// Extra keys in sorted order for determinism.
	keys := make([]string, 0, len(tag.Extra))
	for k := range tag.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := writeKV(k, tag.Extra[k]); err != nil {
			return err
		}
	}

	// Chapters.
	for _, ch := range tag.Chapters {
		if _, err := bw.WriteString("[CHAPTER]\nTIMEBASE=1/1000\n"); err != nil {
			return err
		}
		startMs := durationToMs(ch.Start)
		endMs := durationToMs(ch.End())
		if _, err := fmt.Fprintf(bw, "START=%d\nEND=%d\n", startMs, endMs); err != nil {
			return err
		}
		if err := writeKV("title", ch.Name); err != nil {
			return err
		}
	}

	return bw.Flush()
}

// ReadFFMetadata invokes `ffmpeg -i path -f ffmetadata -` and parses the
// resulting document into a Tag.
func (c *Client) ReadFFMetadata(ctx context.Context, path string) (*audio.Tag, error) {
	res, err := exec.Run(ctx, exec.Cmd{
		Name: c.Bin,
		Args: []string{
			"-hide_banner",
			"-i", path,
			"-f", "ffmetadata",
			"-",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ffmetadata read: %w", err)
	}
	return ParseFFMetadata(strings.NewReader(string(res.Stdout)))
}

// --- helpers ---

type partialChapter struct {
	Timebase string
	Start    int64 // in Timebase units
	End      int64
	Title    string
	hasStart bool
	hasEnd   bool
}

func (p *partialChapter) set(key, value string) {
	switch strings.ToUpper(key) {
	case "TIMEBASE":
		p.Timebase = value
	case "START":
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			p.Start = n
			p.hasStart = true
		}
	case "END":
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			p.End = n
			p.hasEnd = true
		}
	default:
		if strings.EqualFold(key, "title") {
			p.Title = value
		}
	}
}

func (p *partialChapter) build() (audio.Chapter, error) {
	if !p.hasStart || !p.hasEnd {
		return audio.Chapter{}, fmt.Errorf("ffmetadata: chapter missing START or END")
	}
	num, den, err := parseTimebase(p.Timebase)
	if err != nil {
		return audio.Chapter{}, err
	}
	start := timebaseToDuration(p.Start, num, den)
	end := timebaseToDuration(p.End, num, den)
	if end < start {
		return audio.Chapter{}, fmt.Errorf("ffmetadata: chapter END < START (%d < %d)", p.End, p.Start)
	}
	return audio.Chapter{
		Start:  start,
		Length: end - start,
		Name:   p.Title,
	}, nil
}

func parseTimebase(s string) (int64, int64, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("ffmetadata: bad TIMEBASE %q", s)
	}
	num, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || num <= 0 {
		return 0, 0, fmt.Errorf("ffmetadata: bad TIMEBASE numerator %q", s)
	}
	den, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil || den <= 0 {
		return 0, 0, fmt.Errorf("ffmetadata: bad TIMEBASE denominator %q", s)
	}
	return num, den, nil
}

// timebaseToDuration converts an integer count in TIMEBASE units to a
// time.Duration. TIMEBASE=num/den means one unit equals num/den seconds.
//
// We use math/big for the multiply because the naive int64 form
// `count * 1e9 * num / den` overflows on fine timebases at long
// offsets — a real audiobook in the wild had a chapter with
// TIMEBASE=1/10000000 and START=410137300000 (~11h23m), and the
// intermediate count*1e9 (4.1e20) wraps int64 silently and produces
// a nonsense duration. big.Int is overkill performance-wise but
// trivial to reason about.
func timebaseToDuration(count, num, den int64) time.Duration {
	b := new(big.Int).SetInt64(count)
	b.Mul(b, big.NewInt(num))
	b.Mul(b, big.NewInt(int64(time.Second)))
	b.Quo(b, big.NewInt(den))
	return time.Duration(b.Int64())
}

func durationToMs(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}

// readLogicalLine returns a single logical line, joining physical lines
// that end with an odd number of trailing backslashes. Each continuation
// contributes a literal '\n' to the returned string where the backslash
// was stripped. Returns ok=false at EOF.
func readLogicalLine(s *bufio.Scanner) (string, bool, error) {
	if !s.Scan() {
		return "", false, s.Err()
	}
	line := s.Text()
	for trailingBackslashes(line)%2 == 1 {
		line = line[:len(line)-1] + "\n"
		if !s.Scan() {
			if err := s.Err(); err != nil {
				return "", false, err
			}
			break
		}
		line += s.Text()
	}
	return line, true, nil
}

func trailingBackslashes(s string) int {
	n := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n
}

// splitEscapedKV splits line on the first unescaped '='.
func splitEscapedKV(line string) (key, value string, ok bool) {
	escaped := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '=' {
			return line[:i], line[i+1:], true
		}
	}
	return "", "", false
}

func unescape(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		next := s[i+1]
		switch next {
		case '=', ';', '#', '\\':
			b.WriteByte(next)
		case 'n':
			b.WriteByte('\n')
		default:
			// Unknown escape: preserve literally so we don't silently lose data.
			b.WriteByte(c)
			b.WriteByte(next)
		}
		i++
	}
	return b.String()
}

func escape(s string) string {
	needs := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '=', ';', '#', '\n':
			needs = true
		}
		if needs {
			break
		}
	}
	if !needs {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			b.WriteString(`\\`)
		case '=':
			b.WriteString(`\=`)
		case ';':
			b.WriteString(`\;`)
		case '#':
			b.WriteString(`\#`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func escapeKey(s string) string {
	// Keys take the same escape set as values. In practice ffmetadata
	// keys are ASCII identifiers so this is a no-op for the common case.
	return escape(s)
}

// assignGlobalKey populates the appropriate Tag field from a parsed
// key/value, falling back to Tag.Extra for unrecognized keys.
// See spec/metadata-mapping.md §Core mapping table.
func assignGlobalKey(tag *audio.Tag, key, value string) error {
	switch strings.ToLower(key) {
	case "title":
		tag.Title = value
	case "album":
		tag.Album = value
	case "artist":
		tag.Artist = value
	case "album_artist":
		tag.AlbumArtist = value
	case "composer":
		tag.Writer = value
	case "genre":
		tag.Genre = value
	case "description":
		tag.Description = value
	case "longdesc":
		tag.LongDescription = value
	case "comment":
		tag.Comment = value
	case "copyright":
		tag.Copyright = value
	case "publisher":
		tag.Publisher = value
	case "encoder":
		tag.Encoder = value
	case "encoded_by":
		tag.EncodedBy = value
	case "grouping":
		tag.Grouping = value
	case "title-sort":
		tag.SortTitle = value
	case "album-sort":
		tag.SortAlbum = value
	case "artist-sort":
		tag.SortArtist = value
	case "performer":
		tag.Performer = value
	case "language":
		tag.Language = value
	case "lyrics":
		tag.Lyrics = value
	case "date":
		return setIntField(&tag.Year, value, key)
	case "track":
		return setIntField(&tag.Track, value, key)
	case "tracks":
		return setIntField(&tag.Tracks, value, key)
	case "disc":
		return setIntField(&tag.Disk, value, key)
	case "discs":
		return setIntField(&tag.Disks, value, key)
	case "purchase_date", "purd":
		t, err := parsePurchaseDate(value)
		if err != nil {
			return fmt.Errorf("ffmetadata: parsing %s: %w", key, err)
		}
		tag.PurchaseDate = t
	default:
		// Case-sensitive second check for ID3 frame names (TSO2, TSOC)
		// which the mapping table keeps in upper-case.
		switch key {
		case "TSO2":
			tag.SortAlbumArtist = value
		case "TSOC":
			tag.SortWriter = value
		default:
			if tag.Extra == nil {
				tag.Extra = map[string]string{}
			}
			tag.Extra[key] = value
		}
	}
	return nil
}

func setIntField(dst *int, value, key string) error {
	// Tolerate "N/M" style (ffmpeg sometimes emits track as "1/12").
	if idx := strings.Index(value, "/"); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("ffmetadata: %s=%q is not an integer", key, value)
	}
	*dst = n
	return nil
}

func parsePurchaseDate(s string) (time.Time, error) {
	// ffmetadata stores purchase date as ISO date or timestamp.
	layouts := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	s = strings.TrimSpace(s)
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", s)
}
