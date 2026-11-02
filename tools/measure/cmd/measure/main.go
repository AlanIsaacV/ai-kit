package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/alanv/ai-kit/tools/measure/internal/measure"
)

const usage = `measure — a change's base and head under the same local recipe

Usage:
  measure --repo DIR --base REF --head REF [--recipe DIR] [--runs N] [--json]

  --repo DIR     the git repository holding both refs; nothing is written to it
  --base REF     the ref the change starts from
  --head REF     the ref the change ends at
  --recipe DIR   the recipe directory, relative to the repository root and taken
                 from the head (default "bench")
  --runs N       runs of each side, interleaved base then head (default 5)
  --json         print the report as JSON instead of text

Exports both refs with git archive, copies the recipe out of the head export, and
runs <recipe>/recipe --code <export> --out <dir> for each side, N times each. The
recipe prints one JSON metric per line on stdout. For every probe and metric it
prints median, min and max per side, the delta of medians, and whether the two
ranges overlap. The report holds no verdict.

Exit 0 whenever a report was printed, whatever it says, failed runs included.
Non-zero on a usage or an input error, or when a signal stopped the run.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("measure", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", "", "")
	base := flags.String("base", "", "")
	head := flags.String("head", "", "")
	recipe := flags.String("recipe", "bench", "")
	runs := flags.Int("runs", 5, "")
	asJSON := flags.Bool("json", false, "")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "%s\n\n%s", err, usage)
		return 2
	}
	if *repo == "" || *base == "" || *head == "" || *recipe == "" || *runs < 1 || flags.NArg() > 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer stop()

	report, err := measure.Run(ctx, measure.Options{
		Repo:     *repo,
		Base:     *base,
		Head:     *head,
		Recipe:   *recipe,
		Runs:     *runs,
		Progress: stderr,
	})
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "measure: interrupted; no report")
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "measure: "+strings.TrimSpace(err.Error()))
		return 1
	}
	if *asJSON {
		err = measure.EncodeJSON(stdout, report)
	} else {
		err = measure.EncodeText(stdout, report)
	}
	if err != nil {
		fmt.Fprintln(stderr, "measure: "+err.Error())
		return 1
	}
	return 0
}
