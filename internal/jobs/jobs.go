// Package jobs computes the default number of concurrent workers for
// commands that fan out ffmpeg subprocesses (split, merge).
package jobs

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// jobsCap is the hard upper bound on auto-picked worker count. Past
// ~8 concurrent ffmpeg processes on typical audiobook content the
// bottleneck shifts from CPU to disk I/O / shared decoder state, and
// extra workers stop helping.
const jobsCap = 8

// perWorkerMemBytes is our conservative estimate of an ffmpeg
// AAC→MP3 worker's resident set: ffmpeg itself, the decoder state,
// the encoder state, and I/O buffers. Real-world RSS lands in the
// 100–250 MB range; rounding up to 256 MB keeps the memory-aware
// cap on the safe side (better to underutilize than to push the
// host into swap or trigger an OOM kill).
const perWorkerMemBytes int64 = 256 * 1024 * 1024

// Default returns the number of workers to use when the user has not
// set --jobs. The result is the smaller of:
//
//   - cpuCap = min(jobsCap, NumCPU-1) — leaves one core for the OS
//     so a long run doesn't starve interactive work
//   - memCap = max(1, MemAvailable / perWorkerMemBytes) — keeps
//     fan-out within the host's memory budget
//
// On platforms where MemAvailable can't be probed (anything not
// Linux right now), only the CPU cap applies.
//
// When ignoreMemoryCap is true the memory budget is skipped and only
// the CPU cap applies. This is the escape hatch for users who know
// their host can absorb the parallelism (or who hit a noisy
// MemAvailable reading — WSL2's mirrored memory accounting in
// particular has been observed to underreport headroom).
//
// Always >= 1, even on a single-core or near-OOM host.
func Default(ignoreMemoryCap bool) int {
	cpu := clampCPU(runtime.NumCPU())
	if ignoreMemoryCap {
		return cpu
	}
	mem := memoryBudget()
	if mem > 0 && mem < cpu {
		return mem
	}
	return cpu
}

// clampCPU encodes the CPU rule independently of runtime.NumCPU() so
// tests can pin a specific machine size.
func clampCPU(numCPU int) int {
	n := numCPU - 1
	if n < 1 {
		n = 1
	}
	if n > jobsCap {
		n = jobsCap
	}
	return n
}

// memoryBudget returns the worker count the host can fit per its
// available memory, or 0 when the budget can't be determined (caller
// then skips the memory cap).
func memoryBudget() int {
	avail, ok := availableMemory()
	if !ok {
		return 0
	}
	return memoryBudgetFromBytes(avail)
}

// memoryBudgetFromBytes converts an available-memory figure into a
// worker count. Floors at 1 if there's any memory at all — better to
// run something than nothing — and at 0 only when availBytes is
// nonsensical.
func memoryBudgetFromBytes(availBytes int64) int {
	if availBytes <= 0 {
		return 0
	}
	n := int(availBytes / perWorkerMemBytes)
	if n < 1 {
		n = 1
	}
	return n
}

// availableMemory reads /proc/meminfo's MemAvailable line and returns
// it in bytes. Returns (0, false) when /proc/meminfo can't be read,
// which is the case on every non-Linux platform; callers treat that
// as "skip the memory cap" rather than as an error.
//
// MemAvailable is preferred over MemFree because it accounts for
// reclaimable cache — i.e. it's the kernel's own estimate of how
// much memory could be allocated to a new process without paging.
func availableMemory() (int64, bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "MemAvailable:") {
			continue
		}
		// "MemAvailable:    1234567 kB"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}
