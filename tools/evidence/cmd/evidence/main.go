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
	"time"

	"github.com/alanv/ai-kit/tools/evidence/internal/evidence"
)

const usage = `evidence — facts on what one change introduces, or on what one tree holds

Usage:
  evidence --head DIR --base DIR --diff FILE
  evidence --tree DIR

  --head DIR    an export of the code at the head of the change
  --base DIR    an export of the code at its base
  --diff FILE   the unified git diff from base to head
  --tree DIR    an export of the code at one commit, scanned whole; it cannot be
                combined with --head, --base or --diff

With --head, --base and --diff, prints one JSON report marked "mode": "change" on
three checks, restricted to what the change introduces: secrets in added lines,
new dependency advisories, and insecure workflow lines. A finding already at the
base is counted as inherited and not listed.

With --tree, prints one JSON report marked "mode": "tree" on the same three
checks over the whole tree: secrets in every file, every dependency advisory and
every insecure workflow line, each listed under "found". Git history is not read.

Each check says whether it ran, and why not when it did not.

Exit 0 whenever a report was printed, whatever it says. Non-zero on a usage or an
input error, or when a signal stopped the run. The report holds no verdict.
`

var budget time.Duration

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("evidence", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	tree := flags.String("tree", "", "")
	head := flags.String("head", "", "")
	base := flags.String("base", "", "")
	diff := flags.String("diff", "", "")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "%s\n\n%s", err, usage)
		return 2
	}
	change := *head != "" || *base != "" || *diff != ""
	if *tree != "" && change {
		fmt.Fprintf(stderr, "--tree cannot be combined with --head, --base or --diff\n\n%s", usage)
		return 2
	}
	if (*tree == "" && (*head == "" || *base == "" || *diff == "")) || flags.NArg() > 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGABRT)
	defer stop()

	report, err := evidence.Run(ctx, evidence.Options{Tree: *tree, Head: *head, Base: *base, Diff: *diff, Budget: budget})
	if err != nil {
		fmt.Fprintln(stderr, "evidence: "+strings.TrimSpace(err.Error()))
		return 1
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "evidence: interrupted; no report")
		return 1
	}
	if err := evidence.Encode(stdout, report); err != nil {
		fmt.Fprintln(stderr, "evidence: "+err.Error())
		return 1
	}
	return 0
}
