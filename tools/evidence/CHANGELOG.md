# Changelog

## Unreleased

### `evidence` reports what one change introduces

`evidence --head DIR --base DIR --diff FILE` prints one JSON report on secrets in
added lines, new dependency advisories and insecure workflow lines. It exists so that
`review` can put facts beside the rules a change assumes without anybody running a
scanner by hand.

- **Scoped to the change.** What the base already had is a count, in dependencies and
  in workflows. Secrets are read from added lines, so they have no such count.
- **A check that did not run says so.** Absent, crashed, timed out or unreadable all
  give `not-run` with a reason and no finding. A scanner at another version runs and
  the report names both.
- **Go through `govulncheck` directly**, called advisories only.
  **Rust and Python through `osv-scanner`**, offline database, no call analysis.
  REJECTED: `osv-scanner` for Go too, because `govulncheck` is what was run by hand on
  a real repository with private modules. REJECTED: online queries, which send package
  names to an external API.
- **Python only when the image says so.** `uv.lock` is read when the root `Dockerfile`
  runs `uv sync`. Otherwise `not-run`, because a lockfile beside a `pip install` does
  not state what is deployed.
- **JavaScript reports `not-run`.** Outside the first cut.
- **No scanner error text is printed.** `reason` is fixed text with the scanner, the
  target and the exit code, and a workflow `detail` is rebuilt from the parts of a
  `uses:` value that can be shown. REJECTED: filtering that text, because a compile
  error quotes whatever the line held and no filter knows what a secret looks like.
- **The report is swept before it is printed.** Every string in it goes through
  `gitleaks`, and each value found becomes `REDACTED` wherever it occurs, because a
  path, a version, a function or an action name is text from the reviewed repository.
  `sweep` says whether that happened, and on `ran` its reason says it covers what
  `gitleaks` recognises, because `ran` alone reads as an all-clear. When it fails,
  findings, targets and scanners are withheld and each check says so. REJECTED:
  printing the unswept report beside a warning, because the warning does not take the
  value back. REJECTED: scanning the encoded JSON, because an escape in front of a
  value hides its boundary from the rules and a replacement can cut the escape in half.
- **The sweep gives each string whole, cut and in parts.** Each word is cut at every
  character that is not a letter or a digit, and split on `/`, `@` and `:`, because 151
  of the 222 default rules need a token to end in a boundary `/`, `@` or `.` do not
  satisfy, so Stripe, GCP and npm shapes passed the sweep in every path and name while
  it read `ran`. Replacement is by the exact value in the original string. REJECTED:
  splitting on every separator, which breaks a value that holds one of them.
- **Values found on added lines are known to the sweep and the guard.** That pass no
  longer uses `--redact`, so a value found only by its keyword reads `REDACTED` in a
  path or a version, and stops the go tool when `go.mod` or `go.sum` names it. Values
  live in the memory of the process and, between a call and its read, in the run's own
  directory, and each report is deleted as soon as it is read.
- **`gitleaks` reads batches of 16 KiB, with as many calls as that takes.** Over stdin
  it cuts a stream at its first read plus 25000 bytes and misses a value across the
  cut. Added lines overlap so a private key survives a batch edge, and an added line
  longer than a batch is read in overlapping windows, because refusing it would lose
  the check on any change holding one minified file. A report string or `go.mod` line
  that long fails instead. Each batch opens with 512 spaces, because `gitleaks` skips a
  stream that starts like a binary file and `%PDF-1.7` on the first added line hid the
  whole change.
- **Without `gitleaks` nothing is scanned.** The go tool, `osv-scanner` and the workflow
  check do not run, and the report lists no finding and no target. REJECTED: running
  them and printing an unswept report, because a path and a `detail` then carry whatever
  the tree holds, and the go tool writes what `go.mod` names into file names.
- **`go.mod` and `go.sum` are read before the go tool.** A secret in either makes the
  target `not-run`, at head and at base, and the go tool does not run on it, because
  resolving a module writes its path and version into file names in the module cache.
  They are cut the same way as the report, so a value in the middle of a module path is
  seen. They go over stdin. REJECTED: `gitleaks dir`, whose default allowlist skips
  both. REJECTED: reading the hash column of `go.sum`, because `generic-api-key` takes
  `oauth2 v0.18.0 h1:...` for a key on most real `go.sum` files and a hash reaches no
  file name.
- **Scanners get the run's own temp directory and a process group.** `TMPDIR` and
  `GOTMPDIR` sit in the run's directory, the group is killed on the budget, on a signal
  and when the scanner exits, and SIGINT, SIGTERM, SIGHUP, SIGQUIT and SIGABRT clean up
  before the process ends. A `go list` left under a killed `govulncheck` kept writing
  cgo-processed source to the host's temp directory.
- **`go_mod` is read from a top-level line.** The earlier pattern crossed newlines and
  matched inside blocks, so it reported the wrong thing whenever a block held a line
  starting with `go`.
- **The README states where this stops.** A value `gitleaks` does not recognise stays
  as it was, which includes a keyword value that is on no added line and a token glued
  to what comes before it, and SIGKILL leaves the run's directory behind, so "no secret
  value leaves" would be more than the code delivers.
