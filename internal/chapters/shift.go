package chapters

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ShiftSpec is the parsed form of the --shift flag: a millisecond
// offset and an optional list of chapter indexes to apply it to.
//
//	--shift 3000                   -> Offset=3s,  Indexes=nil (all)
//	--shift 3000:0,1,4,5           -> Offset=3s,  Indexes=[0,1,4,5]
//	--shift -2500:-1               -> Offset=-2.5s, Indexes=[-1]
type ShiftSpec struct {
	Offset  time.Duration
	Indexes []int
}

// ParseShiftSpec parses the --shift argument. A zero-millisecond offset
// returns (_, false, nil) so callers can tell "flag not set" from
// "flag set to 0" if they care. An error is returned only on malformed
// syntax.
func ParseShiftSpec(s string) (ShiftSpec, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return ShiftSpec{}, false, nil
	}
	offStr, idxStr, hasColon := strings.Cut(s, ":")
	ms, err := strconv.Atoi(strings.TrimSpace(offStr))
	if err != nil {
		return ShiftSpec{}, false, fmt.Errorf("shift: bad millisecond offset %q", offStr)
	}
	spec := ShiftSpec{Offset: time.Duration(ms) * time.Millisecond}
	if hasColon {
		for _, p := range strings.Split(idxStr, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			n, err := strconv.Atoi(p)
			if err != nil {
				return ShiftSpec{}, false, fmt.Errorf("shift: bad index %q", p)
			}
			spec.Indexes = append(spec.Indexes, n)
		}
	}
	return spec, true, nil
}
