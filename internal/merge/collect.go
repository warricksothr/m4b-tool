package merge

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultExtensions is the audio file extension allowlist from
// spec/cli-surface.md §File collection (minus the leading dot).
var DefaultExtensions = []string{"aac", "alac", "flac", "m4a", "m4b", "mp3", "oga", "ogg", "wav", "wma", "mp4"}

// collectInputs resolves the user's positional inputs into a flat,
// ordered list of audio file paths.
//
// For each input arg, in the order the user supplied:
//   - a directory is walked recursively; matching files are sorted
//     alphabetically before being appended.
//   - a file is included unconditionally (no extension filter), letting
//     the user opt a specific file in even if its extension is unusual.
//
// Returns an error if any input arg fails to stat.
func collectInputs(args []string, extensions []string) ([]string, error) {
	extSet := make(map[string]bool, len(extensions))
	for _, e := range extensions {
		extSet["."+strings.ToLower(strings.TrimPrefix(e, "."))] = true
	}

	var out []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("input %q: %w", arg, err)
		}
		if !info.IsDir() {
			out = append(out, arg)
			continue
		}
		var dirFiles []string
		walkErr := filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if extSet[strings.ToLower(filepath.Ext(path))] {
				dirFiles = append(dirFiles, path)
			}
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk %q: %w", arg, walkErr)
		}
		sort.Strings(dirFiles)
		out = append(out, dirFiles...)
	}
	return out, nil
}
