package tag

import (
	"context"
	"fmt"
	"strings"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

// Composite chains a list of Importers and runs them in order, passing
// each the evolving Tag from the previous one. Per-importer failures
// abort the chain.
//
// Enable and Disable implement the --enable-improvers /
// --disable-improvers CLI flags. Name matching is case-insensitive.
//   - Enable, when non-empty, restricts execution to the listed names.
//   - Disable unconditionally skips the listed names.
//   - Disable takes precedence when both sets contain the same name.
type Composite struct {
	Importers []Importer
	Enable    map[string]bool
	Disable   map[string]bool
}

// Improve runs each importer in order. The input Tag is not mutated;
// the returned Tag incorporates every importer's contribution.
func (c *Composite) Improve(ctx context.Context, tag audio.Tag, workDir string) (audio.Tag, error) {
	out := tag
	for _, imp := range c.Importers {
		if !c.shouldRun(imp.Name()) {
			continue
		}
		next, err := imp.Improve(ctx, out, workDir)
		if err != nil {
			return out, fmt.Errorf("importer %s: %w", imp.Name(), err)
		}
		out = next
	}
	return out, nil
}

func (c *Composite) shouldRun(name string) bool {
	n := strings.ToLower(name)
	if c.Disable[n] {
		return false
	}
	if len(c.Enable) > 0 && !c.Enable[n] {
		return false
	}
	return true
}

// NamesSet converts a slice of importer names (whatever form the CLI
// accepted them in) into a normalized set suitable for Composite.Enable
// or Composite.Disable. Whitespace is trimmed; names are lowercased.
func NamesSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.TrimSpace(strings.ToLower(n))
		if n != "" {
			out[n] = true
		}
	}
	return out
}
