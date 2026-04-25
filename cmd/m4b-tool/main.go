package main

import (
	"context"
	"flag"
	"fmt"
	"io"
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
		printHelp(os.Stdout)
		return
	}
	switch os.Args[1] {
	case "help", "--help", "-h":
		printHelp(os.Stdout)
		return
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
		fmt.Fprintf(os.Stderr, "unknown command %q\nrun \"m4b-tool --help\" for usage\n", os.Args[1])
		os.Exit(2)
	}
}

// printHelp writes the top-level usage to w. Sub-command flag details
// live with each sub-command — invoke `m4b-tool <command> --help` to
// see them. The Go stdlib's flag.FlagSet renders that automatically
// via the per-command fs.Usage hooks.
func printHelp(w io.Writer) {
	fmt.Fprint(w, `m4b-tool — manage audiobook .m4b files

Usage:
  m4b-tool <command> [flags]

Commands:
  merge      combine input files into a single tagged .m4b
  split      extract chapters from a file into per-chapter outputs
  chapters   read, write, shift, or repair chapter markers
             ('chapters export <input> [output]' dumps a sidecar)
  doctor     report which external tools are installed and runnable
  probe      print the duration of an audio file (debug helper)
  version    print build version info
  help       print this message

Run "m4b-tool <command> --help" for command-specific flags.

Project: https://github.com/warricksothr/m4b-tool
`)
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
