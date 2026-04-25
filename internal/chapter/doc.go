// Package chapter implements the pure chapter-manipulation algorithms
// used by the merge, split, and chapters commands.
//
// All functions here are pure: no subprocess, no I/O, no goroutines.
// They take chapter and/or silence slices and return new slices without
// mutating inputs. See spec/chapter-algorithms.md for the canonical
// description of each algorithm.
package chapter
