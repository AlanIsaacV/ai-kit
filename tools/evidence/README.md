# evidence

Facts on what one change introduces, for the person who has to decide on it.

Given an export of the code at the head of a change, an export at its base, and the
diff between them, `evidence` prints one JSON report on three checks: secrets in
added lines, new dependency advisories, and insecure workflow lines. It runs without
an LLM, gives no verdict and has no opinion on what any of it means.

`evidence` knows nothing about skills, plans or review. It takes three paths and
prints a report. That boundary is deliberate: it is what lets this tool be lifted
into its own repository later without rewriting a line of it.

```bash
evidence --head DIR --base DIR --diff FILE
```

It exits 0 whenever it printed a report, whatever the report says, and non-zero on a
usage or an input error, or when a signal stopped the run, which prints no report. A
file that is not a git diff is an input error rather than an empty change, because an
unreadable diff would otherwise read as no findings.

## Why it is shaped like this

**What is new, not what is there.** A report of the whole repository reads the same
on every branch, so nobody reads it. A finding already at the base is excluded from
`new` and counted in `inherited`, and that count is all that is said about it.

**A scanner that did not run never reads as clean.** Absence is an ordinary state:
scanners are installed per account and this tool installs none. A scanner that is
missing, crashes, runs out of time or prints something unreadable makes its check
`not-run` with a reason. It lists no finding and carries no `inherited`, so there is
no zero to misread. A scan of the base that fails makes the target `not-run` too,
since nothing could then tell new from inherited.

`gitleaks` is the one scanner the others stand on, because what they find is swept
through it. Without it nothing is scanned at all: no go tool, no `osv-scanner`, no
workflow check. `secrets` says `gitleaks` is not installed, `dependencies`, `workflows`
and `sweep` carry one fixed reason, and the report lists no finding and no target.

**A different version runs and says so.** A scanner other than the pinned version
runs anyway, and the report carries both versions and `at_pin: false`. Refusing would
turn an upgrade into a lost check.

**The reviewed repository does not configure its own review.** Scanners run from a
directory created for the run, so a `.gitleaks.toml` or `.gitleaksignore` in the
change is not picked up. `gitleaks:allow` is ignored, and `osv-scanner` is handed an
empty config of the run's own, because an `osv-scanner.toml` in the repository hides
advisories and leaves the exit code at 0.

**Nothing from the reviewed code runs.** No build script, no `go generate`, no test.
`GOTOOLCHAIN=local` keeps a scan from downloading a toolchain, and `osv-scanner` runs
with call analysis off, because call analysis for Rust runs `build.rs`. Symlinks in an
export are never followed: a tracked link can point anywhere.

**A secret value stays out of what the run prints and leaves, as far as `gitleaks`
recognises it.** That is the whole of the promise, and the paragraphs below say where
it stops.

Added lines reach `gitleaks` over stdin. This pass runs without `--redact`, because a
value found here has to be recognised again wherever else it turns up. Its report sits
in the run's own directory, mode 0700, and is deleted as soon as it is read. Only the
rule and the location go into `secrets`. The value stays in the memory of the process
for the steps below and is never written anywhere else. A `credentials_json` finding
prints no value either, since what follows that key on the line can be the credential.

What a scanner prints on stderr is never repeated. A compiler quotes the line it failed
on and a parser quotes the token it could not read, so a `reason` is built from fixed
text: the scanner, the target, the exit code and the kind of failure. It says that
packages could not be loaded, not which line stopped them. Seeing that line takes
running the scanner by hand.

The other fields are text from the reviewed repository: a path, a version, a function,
an action or image name. So the report is swept. Once it is built and before anything
is printed, every string in it goes through `gitleaks` again, and each value found is
replaced by `REDACTED` wherever it occurs in the report, along with every value the
added lines gave. That is how `src/ghp_.../app.go` comes to read `src/REDACTED/app.go`.
A value `gitleaks` found only by the keyword beside it on an added line, as in
`api_key = "..."`, reads `REDACTED` in a path or a version too. So does whatever else
`gitleaks` took for a secret on an added line, so a false positive of its can blank
a name. A report with nothing recognised in it comes out unchanged.

The value in a string is not always where a rule can see it. 151 of the 222 default
rules want a token followed by a quote, a space, a `;` or the end of the text, so
`sk_live_...` followed by `/`, `@` or `.` is no match. So `gitleaks` is given each string
whole, then each of its words cut at every character that is not a letter or a digit,
then the parts between `/`, `@` and `:`. A cut keeps everything before it, so a value
with a `-` of its own is still whole in one of them. What is found is then replaced by
its exact value in the original string, so the cuts never reach the report. Strings go
in decoded and are replaced before the report is encoded, so an escape can neither hide
a value from the scan nor be cut by the replacement. This run has to go without
`--redact` too, on the same terms as the first.

`sweep` in the report says how that went. `ran` carries a reason that says what it
means: it covers what `gitleaks` recognises, and it is no statement that nothing is
there. When the added lines could not be read, the reason says that as well. When
`gitleaks` crashed, ran out of its own thirty seconds, printed something unreadable,
named a value that cannot be placed in the report, was given a string it cannot read
whole or was gone by then, nothing unswept is printed. Each check that ran or partly ran
becomes `not-run` with a reason saying so, and the report holds no finding, no target
and no scanner. A check that was already `not-run` or `not-applicable` keeps its reason,
which is fixed text.

`gitleaks` always gets its input in batches of 16 KiB, each line whole, with as many
calls as that takes. Over stdin it cuts a stream at its first read plus 25000 bytes, and
a value across that cut is found by neither side. That read is whatever the pipe held,
usually 65536 bytes and sometimes far fewer, so only a batch under 25000 bytes is read
in one piece however the pipe behaves. Batches of added lines overlap by 4 KiB, because
a private key runs across lines. An added line longer than a batch is read in windows
overlapping by 4 KiB. A string in the report or a line in `go.mod` that long is refused
instead: the sweep fails, or the go tool does not run. Each batch opens with a line of
512 spaces, because `gitleaks` skips a stream that starts like a binary file. A first
line reading `%PDF-1.7` hid every added line after it. A call takes about a quarter of
a second, which comes to some twenty seconds for each MiB of added lines.

`go.mod` and `go.sum` are read before the go tool, because resolving a module writes
its path and version into file names under `$HOME/go/pkg/mod/cache/download`, even with
`GOPROXY=off`. `gitleaks` reads both, at head and at base, whichever is being scanned:
all of `go.mod`, and the module and version columns of `go.sum`, cut the same way as the
report, so that `example.com/sk_live_.../sub` does not get through. A finding makes the
target `not-run` and the go tool does not run on it. So does a value the added lines
gave, a `gitleaks` that fails here, and a `go.sum` that is a link, since that file
cannot be read without following it. They go over stdin because `gitleaks dir` skips
both names through its default allowlist. The hash column stays out because
`generic-api-key` reads `oauth2 v0.18.0 h1:...` as a key: read whole, 227 of 400
`go.sum` files in a module cache came back with a finding. With the column out and the
cuts in it was none of 922, and none of 1439 `go.mod` files. No hash reaches a file
name.

Every scanner runs in its own process group, with `TMPDIR` and `GOTMPDIR` inside the
run's directory, and `GOCACHE` points there too, because `go list` writes the
cgo-processed source of the reviewed code in both. That directory is removed on every
exit path. The group is killed when the budget runs out, on a signal, and when the
scanner exits, so a `go list` left underneath it cannot write again. SIGINT, SIGTERM,
SIGHUP, SIGQUIT and SIGABRT stop the run, clean up and print no report.

Where this stops:

- **`sweep: ran` is not an all-clear.** It says the strings went through `gitleaks`. A
  value `gitleaks` does not recognise stays as it was, and the bullets below are the
  ways that happens.
- **A value found only by its keyword is known only from the added lines.** Alone in a
  path, a name or a `go.mod` it has no keyword beside it. If it is not on an added line
  in the form `gitleaks` reads, or that pass failed, the sweep does not recognise it
  and it is printed, and the check on `go.mod` does not recognise it and it can reach
  the module cache. A secret that is only a name, with no shape, is not recognised at
  all.
- **The edges of a value still count.** The cuts give a rule the end it wants. They give
  it no start: where a rule wants a word boundary, `Fbadsk_live_...` and
  `x_sk_live_...` are not recognised, and neither is a token followed by more letters
  or digits, as in `npm_...x`, which is then simply a longer word.
- **`secrets` lists what `gitleaks` sees on the added line as written.** The cuts are
  for the report and the manifests. `uses: actions/sk_live_.../x@v1` is no finding in
  `secrets`, although `detail` reads `actions/REDACTED/x@v1`.
- **An added line longer than a batch can hide a long value.** A value of more than
  4 KiB that crosses a window in such a line is missed.
- **A change too large for the budget does not finish.** Past some twenty-five MiB of
  added lines `secrets` runs out of the nine minutes and reads `not-run`. A report too
  large for the thirty seconds of the sweep is withheld.
- **The check on `go.mod` reads the module's own two files.** What a dependency's
  `go.mod` names is read by the go tool alone.
- **SIGKILL cannot be handled.** It leaves the run's directory behind, and with it the
  cgo-processed source and, in the moment between a call to `gitleaks` and the read of
  its report, the values in that report. Scanners already running keep running.
- **Neither can a few signals nothing sends by accident.** SIGSEGV, SIGBUS, SIGFPE,
  SIGILL, SIGTRAP, SIGSYS and SIGSTKFLT sent from outside end the process the way a
  crash does, with the same result as SIGKILL.

**Two runs print the same bytes.** Output is sorted and holds no timestamp of the run.
That holds for the same inputs, scanner versions and database dates, and the report
states those because the third moves on its own.

## The checks

| Check | Source | Over |
|---|---|---|
| `secrets` | `gitleaks stdin` | added lines only, so there is no `inherited` |
| `dependencies` | `govulncheck`, `osv-scanner` | every manifest found walking the tree |
| `workflows` | native | `.github/workflows/*` and `.github/actions/**`, YAML files |

**Dependencies.** The walk skips dot-directories, `vendor`, `vendors`, `node_modules`,
`bower_components`, `third_party`, `third-party`, `testdata` and `__pycache__`.

| Found | What happens |
|---|---|
| `go.mod` | `govulncheck -scan symbol`; only advisories whose function is called count |
| `Cargo.lock` | `osv-scanner`, with the offline database |
| `Cargo.toml` with no lockfile at or above it | `not-run` |
| root `uv.lock`, root `Dockerfile` runs `uv sync` | `osv-scanner`, with the offline database |
| any other Python manifest or lockfile | `not-run`, saying what the root `Dockerfile` does |
| `package.json` | `not-run`: JavaScript is outside this cut |
| none of these | the check is `not-applicable` |

The offline database means private package names never reach an external API.
`--download-offline-databases` keeps it current under the scanner's own cache.

**Workflows.** `unpinned-action` is a `uses:` not pinned to a full SHA, or an image not
pinned to a digest. Local `./` references are excluded. `credentials-json` and
`pull-request-target` are those keys on a line. A finding on an added line that the
base already had in that file, as when a line moves, counts as inherited.

`detail` on an `unpinned-action` is rebuilt from the `uses:` value and never copied:
the action or image name, and its version only when it reads as a version number,
`latest`, `main` or `master`. Credentials in a URL or an image reference are dropped.
A value that is neither an action nor an image has no `detail`, and the finding still
names its `path` and `line`. What is left is swept like any other field, so a token
used as a name reads `actions/REDACTED@v1`.

## The report

```json
{
  "secrets": {
    "status": "ran",
    "scanners": [
      {
        "name": "gitleaks",
        "version": "8.30.1",
        "pinned": "8.30.1",
        "at_pin": true,
        "database": "rules built into the scanner",
        "database_date": "not-applicable"
      }
    ],
    "new": [{ "rule": "github-pat", "path": "settings.txt", "line": 1 }]
  },
  "dependencies": {
    "status": "partial",
    "reason": "1 of 2 dependency targets were not scanned",
    "scanners": [],
    "targets": [
      {
        "ecosystem": "go",
        "path": "go.mod",
        "status": "ran",
        "toolchain": "go1.27.3",
        "go_mod": "1.26"
      },
      {
        "ecosystem": "javascript",
        "path": "web/package.json",
        "status": "not-run",
        "reason": "advisories for JavaScript are outside this cut"
      }
    ],
    "new": [
      {
        "rule": "GO-2026-0001",
        "path": "go.mod",
        "package": "example.com/dep",
        "version": "v1.0.0",
        "functions": ["example.com/dep/pkg.Bad"]
      }
    ],
    "inherited": 2
  },
  "workflows": { "status": "not-applicable", "reason": "no workflow file at head", "scanners": [], "new": [] },
  "sweep": {
    "status": "ran",
    "reason": "every string went through gitleaks and what it recognised is replaced; this covers what gitleaks recognises, and a value it does not recognise stays as it was"
  }
}
```

`status` is `ran`, `partial` when some dependency targets ran and others did not,
`not-run`, or `not-applicable`. The scanners have a budget of nine minutes, so a host
that kills a long command still gets a report: whatever did not finish is `not-run`.
The sweep runs after that, on thirty seconds of its own, so a report that ran out of
time is swept too. `go_mod` is read from a top-level `go` line alone, never from inside
a block, and only when it reads as a Go version; otherwise the field is not there.

## Scanners

| Scanner | Pinned | Reads |
|---|---|---|
| `gitleaks` | 8.30.1 | stdin: added lines, `go.mod` and `go.sum`, the report. Without it nothing else runs |
| `osv-scanner` | 2.7.0 | one lockfile at a time |
| `govulncheck` | v1.8.0 | one module at a time, through `go` |

Each is looked up on PATH, then in `~/.local/bin` and `~/go/bin`, because neither
directory is reliably on PATH. `go` itself has to be on PATH.

## Tests

```bash
go test ./...
```

They run in this module and not in `make check`, which works with no Go toolchain.
`TestSecretValueIsNeverPrinted` runs the real `gitleaks`, and the real `govulncheck`
over a local database, in a child process, and reads what that process printed and
what it left in `HOME` and `TMPDIR`, file names as well as contents. It seeds a token
in each place a report or a scanner can carry one, in the shapes that need a boundary
and followed by the characters that deny it, a keyword value on an added line that also
turns up in a path and a version, and a report and a change large enough to cross any
read of the stream. It kills a run on cgo code with a signal and with an exhausted
budget. A stand-in gives the scanner output where a route needs an advisory no local
database holds. Each part skips when what it needs is absent.
