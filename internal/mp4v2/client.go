package mp4v2

import (
	"context"
	"fmt"
	stdexec "os/exec"
	"strings"
	"time"

	"github.com/warricksothr/m4b-tool/internal/exec"
)

// Client bundles resolved paths for the four mp4v2 utility binaries and
// the feature flags detected from mp4tags -help output. Construct with
// [NewClient]; the zero value is not usable.
type Client struct {
	MP4Chaps string
	MP4Tags  string
	MP4Art   string
	MP4Info  string

	// SupportsSortNames is true when the mp4tags binary accepts the
	// -sortname / -sortalbum / -sortartist / -sortalbumartist /
	// -sortcomposer flags. Older builds omit these.
	SupportsSortNames bool

	// SupportsPurchaseDate is true when the mp4tags binary accepts the
	// -purchasedate flag. Also version-dependent.
	SupportsPurchaseDate bool
}

// NewClient resolves each of the four mp4v2 utility binaries on PATH
// and probes mp4tags for optional-feature support. Returns an error the
// moment any required binary is missing so callers can surface the
// install hint at startup rather than at first use.
func NewClient() (*Client, error) {
	c := &Client{}
	paths := []struct {
		name string
		dst  *string
	}{
		{"mp4chaps", &c.MP4Chaps},
		{"mp4tags", &c.MP4Tags},
		{"mp4art", &c.MP4Art},
		{"mp4info", &c.MP4Info},
	}
	for _, p := range paths {
		resolved, err := stdexec.LookPath(p.name)
		if err != nil {
			return nil, fmt.Errorf("mp4v2: %q not found on PATH — install the mp4v2 utility suite", p.name)
		}
		*p.dst = resolved
	}

	// Feature-detect once. A failure here is non-fatal: we leave the
	// bools false so callers simply skip the affected fields on write.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	help, _ := readMp4TagsHelp(ctx, c.MP4Tags)
	c.SupportsSortNames = strings.Contains(help, "sortname")
	c.SupportsPurchaseDate = strings.Contains(help, "purchasedate")
	return c, nil
}

// readMp4TagsHelp captures the combined stdout+stderr from mp4tags -help.
// mp4tags writes its usage to stdout in builds we tested but older
// forks may print to stderr; merging is the robust option.
func readMp4TagsHelp(ctx context.Context, bin string) (string, error) {
	res, err := exec.Run(ctx, exec.Cmd{Name: bin, Args: []string{"-help"}})
	// mp4tags -help historically exits 0; -h may exit non-zero on some
	// forks. Either way we just want the captured text.
	return string(res.Stdout) + string(res.Stderr), err
}
