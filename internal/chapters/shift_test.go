package chapters

import (
	"reflect"
	"testing"
	"time"
)

func TestParseShiftSpec(t *testing.T) {
	tests := []struct {
		in       string
		wantOK   bool
		wantSpec ShiftSpec
		wantErr  bool
	}{
		{"", false, ShiftSpec{}, false},
		{"3000", true, ShiftSpec{Offset: 3 * time.Second}, false},
		{"3000:0,1,4,5", true, ShiftSpec{Offset: 3 * time.Second, Indexes: []int{0, 1, 4, 5}}, false},
		{"-2500:-1", true, ShiftSpec{Offset: -2500 * time.Millisecond, Indexes: []int{-1}}, false},
		{"100: 2 , 3 ", true, ShiftSpec{Offset: 100 * time.Millisecond, Indexes: []int{2, 3}}, false},
		{"500:", true, ShiftSpec{Offset: 500 * time.Millisecond}, false},

		{"abc", false, ShiftSpec{}, true},
		{"500:x", false, ShiftSpec{}, true},
	}
	for _, tc := range tests {
		spec, ok, err := ParseShiftSpec(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseShiftSpec(%q) = (%+v, %v, nil), want error", tc.in, spec, ok)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseShiftSpec(%q) unexpected err: %v", tc.in, err)
			continue
		}
		if ok != tc.wantOK {
			t.Errorf("ParseShiftSpec(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
		}
		if !reflect.DeepEqual(spec, tc.wantSpec) {
			t.Errorf("ParseShiftSpec(%q) = %+v, want %+v", tc.in, spec, tc.wantSpec)
		}
	}
}
