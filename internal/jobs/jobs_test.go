package jobs

import "testing"

func TestClampCPU(t *testing.T) {
	cases := []struct {
		numCPU int
		want   int
	}{
		// Floor: even on a 1-core host we run at least 1 worker
		// (anything less means no work happens).
		{1, 1},

		// Below the cap: NumCPU - 1.
		{2, 1},
		{4, 3},
		{8, 7},
		{9, 8},

		// At/above the cap: 8.
		{10, 8},
		{16, 8},
		{128, 8},

		// Defensive: clampCPU should never go negative even if
		// runtime.NumCPU() returned something nonsensical.
		{0, 1},
		{-3, 1},
	}
	for _, tc := range cases {
		if got := clampCPU(tc.numCPU); got != tc.want {
			t.Errorf("clampCPU(%d) = %d, want %d", tc.numCPU, got, tc.want)
		}
	}
}

func TestMemoryBudgetFromBytes(t *testing.T) {
	// perWorkerMemBytes = 256 MiB, expressed in bytes.
	mb := func(n int64) int64 { return n * 1024 * 1024 }

	cases := []struct {
		name      string
		availMiB  int64
		want      int
	}{
		// Unknown / nonsensical → 0 (caller skips memory cap).
		{"zero (unknown)", 0, 0},
		{"negative", -1, 0},

		// Even tiny memory still allows one worker.
		{"100 MiB", 100, 1},

		// Exactly one budget unit.
		{"256 MiB", 256, 1},

		// Two units.
		{"512 MiB", 512, 2},

		// Typical desktop available figures.
		{"2 GiB", 2 * 1024, 8},
		{"8 GiB", 8 * 1024, 32},

		// At/above gives integer divide.
		{"1 GiB", 1024, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := memoryBudgetFromBytes(mb(tc.availMiB))
			if got != tc.want {
				t.Errorf("memoryBudgetFromBytes(%d MiB) = %d, want %d",
					tc.availMiB, got, tc.want)
			}
		})
	}
}

func TestDefaultFloors(t *testing.T) {
	// We can't pin runtime.NumCPU() or /proc/meminfo in a portable
	// test, but we can verify the public Default() never returns < 1
	// — the load-bearing invariant for callers that pass it into a
	// worker channel.
	for _, ignoreMem := range []bool{false, true} {
		if got := Default(ignoreMem); got < 1 {
			t.Errorf("Default(%v) = %d, want >= 1", ignoreMem, got)
		}
	}
}

func TestDefaultIgnoringMemoryAtLeastSmartDefault(t *testing.T) {
	// Invariant: ignoring the memory cap can only raise the worker
	// count, never lower it. (CPU cap is always applied; memory cap
	// only further restricts.)
	smart := Default(false)
	uncapped := Default(true)
	if uncapped < smart {
		t.Errorf("Default(ignore=true)=%d should be >= Default(ignore=false)=%d",
			uncapped, smart)
	}
}
