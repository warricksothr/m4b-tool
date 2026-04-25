package merge

import (
	"reflect"
	"testing"
)

func TestInterleaveWithSilence(t *testing.T) {
	tests := []struct {
		name    string
		parts   []string
		silence string
		want    []string
	}{
		{
			name:    "three parts gets two silences",
			parts:   []string{"a", "b", "c"},
			silence: "S",
			want:    []string{"a", "S", "b", "S", "c"},
		},
		{
			name:    "two parts gets one silence",
			parts:   []string{"a", "b"},
			silence: "S",
			want:    []string{"a", "S", "b"},
		},
		{
			name:    "single part returns unchanged",
			parts:   []string{"a"},
			silence: "S",
			want:    []string{"a"},
		},
		{
			name:    "empty silence returns parts as-is",
			parts:   []string{"a", "b"},
			silence: "",
			want:    []string{"a", "b"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := interleaveWithSilence(tc.parts, tc.silence)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
