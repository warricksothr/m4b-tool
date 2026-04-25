package tag

import (
	"fmt"
	"sort"
	"strings"
)

// KnownImporters is the set of canonical importer names the tool
// recognizes. Used to validate the user's --enable-improvers /
// --disable-improvers arguments at flag-parse time so typos produce a
// useful error instead of a silent skip.
//
// The actual importer wiring for each command lives in the orchestrators
// (merge.Run, split.Run, chapters.Run); this set is purely the naming
// catalog.
var KnownImporters = map[string]struct{}{
	"ffmetadata":                {},
	"chapters-txt":              {},
	"description":               {},
	"cover":                     {},
	"cuesheet":                  {},
	"chapters-from-file-tracks": {},
	"guess-chapters-by-silence": {},
	"equate":                    {},
}

// ValidateNames returns an error listing any names that are not
// recognized importers. An empty input yields nil.
func ValidateNames(names []string) error {
	var bad []string
	for _, n := range names {
		n = strings.TrimSpace(strings.ToLower(n))
		if n == "" {
			continue
		}
		if _, ok := KnownImporters[n]; !ok {
			bad = append(bad, n)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf("unknown importer(s): %s (known: %s)",
		strings.Join(bad, ", "), knownImporterList())
}

func knownImporterList() string {
	names := make([]string, 0, len(KnownImporters))
	for n := range KnownImporters {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
