package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/warricksothr/m4b-tool/internal/audio"
	"github.com/warricksothr/m4b-tool/internal/doctor"
	"github.com/warricksothr/m4b-tool/internal/ffmpeg"
)

// Version metadata is filled in by goreleaser via -ldflags at release
// time; defaults are used for local builds.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("ok")
		return
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("m4b-tool %s (%s, built %s)\n", version, commit, date)
		return
	case "probe":
		os.Exit(runProbe(os.Args[2:]))
	case "chapters":
		os.Exit(runChaptersCmd(os.Args[2:]))
	case "merge":
		os.Exit(runMergeCmd(os.Args[2:]))
	case "split":
		os.Exit(runSplitCmd(os.Args[2:]))
	case "doctor":
		os.Exit(doctor.Run(context.Background()))
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}

// runProbe is the Milestone 1 throwaway entry point. It prints the
// duration of a file using the ffmpeg probe, exercising the exec wrapper
// and the ffmpeg Client end-to-end. Real subcommand plumbing arrives with
// the chapters/merge/split commands in later milestones.
func runProbe(args []string) int {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	exact := fs.Bool("exact", false, "run the exact (full-decode) probe instead of the fast header probe")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: m4b-tool probe [--exact] <file>")
		return 2
	}
	path := fs.Arg(0)

	client, err := ffmpeg.NewClient("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx := context.Background()
	var (
		d    time.Duration
		err2 error
	)
	if *exact {
		d, err2 = client.ProbeDurationExact(ctx, path)
	} else {
		d, err2 = client.ProbeDuration(ctx, path)
	}
	if err2 != nil {
		fmt.Fprintln(os.Stderr, err2)
		return 1
	}
	fmt.Println(audio.FormatHMS(d))
	return 0
}
