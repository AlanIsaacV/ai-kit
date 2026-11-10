---
name: audit
description: "Rate one repository at one ref against ai-kit's safe-to-expose standard and say whether it is fit to expose to the internet. Takes a local repository and a ref, any commit-ish, `HEAD` by default. It exports the commit with `git archive` to a temporary directory without touching the repository, detects the stacks from the manifests, runs `evidence --tree` over the export, and launches four read-only `audit-lens` agents in parallel, one per group of axes (1 who gets in | 2 untrusted input reaching something powerful | 3-4 secrets and where it runs | 5-6 supply chain and resources, with the scanner evidence), each given the standard cut before its Calibration section. It checks every returned `path:line` against the export, merges and de-duplicates findings by anchor, decides the six mandatory controls and the verdict by the standard's bar, and prints in the conversation: a verdict line, `fit to expose` or `not fit to expose` with every confirmed Critical and High and every absent control; the mandatory controls; confirmed findings ranked CRITICAL > HIGH > MEDIUM and then by checklist item, each with `path:line`, the exposure path, the missing control and the exploit; suspected findings with the fact that would confirm each; the scanner evidence check by check; the readings chosen where the standard is ambiguous; and everything not read, git history included. Read-only over the audited repository: nothing reaches its working tree, its `.git` or its memory log, nothing from its code runs, and no live system of the app is contacted. Use when somebody asks whether an app can be put on the internet. Triggers on: 'auditá este repo', 'auditá esta app', 'está lista para exponer', 'se puede exponer esta app', 'es segura para exponer', 'audit this repo', 'audit this app', 'is this app safe to expose', 'is this fit to expose', 'can this go on the internet'. It writes no fix and reviews no change, which is `review`'s job; it never edits, never commits and never invokes `plan`, `work` or `debug`."
---

# audit

Read [the host runtime](references/runtime.md) first for tool invocation and
delegation. Two things here differ from what it says: its memory preflight does
not apply, because this skill runs no `memory` command, and the lenses get no
path to the audited repository (rule 4), not its `repo_root`.

Say whether one app is fit to expose to the internet, by the bar
`standards/safe-to-expose.md` sets, and show what that rests on: every finding
with `path:line`, ranked, confirmed apart from suspected, the scanner evidence,
and what nobody read. The serious gaps are logic flaws a scanner does not see, so
the read is the audit and the scanners are evidence beside it.

This skill is the door and the orchestrator. It settles the repository and the
commit, exports the commit to a temporary directory, detects the stacks, cuts the
standard, runs `evidence --tree`, launches four `audit-lens` readers in parallel,
checks and merges what they return, decides the mandatory controls and the
verdict, cleans up and prints the report. The lenses read and return; none of
them decides the verdict.

## Hard rules

1. **Read-only over the audited repository.** The git commands below are all this
   skill runs there, and each only reads: `rev-parse` and `archive`, the latter
   with LFS smudge disabled. No `git fetch`, no `git worktree add`, no
   `git checkout`, and no `git status`, which refreshes the index behind your
   back. No `memory` command either: its log is not read, and a read would create
   `.ai-kit/` in it.
2. **No live system of the app is contacted**, by this skill or by a lens: no
   request to any route it serves, any URL it calls, any database or service it
   names. The one network use is `evidence`'s scanners fetching their public
   advisory databases, which is not contact with the app.
3. **Nothing from the audited code runs.** No build, no tests, no install, no
   script it ships.
4. **The lenses read only the temporary directory.** Every path they get is inside
   `<work-dir>`, and the standard they get is the copy cut before `## Calibration`:
   that section names the answers for a known repository, and a reader who has
   seen them is no longer testing the checklist.
5. **The report lives in the conversation.** Nothing is written into the audited
   repository, no file and no record is kept, and `<work-dir>` is removed.
6. **Nothing unread passes as read**, and a scanner that did not run is not a
   clean result. Git history is never scanned, and the report says so.
7. **Conversation in the user's language; everything written to disk in English.**
   The verdict phrases, severities, item numbers and control ids stay as the
   standard spells them.

## Commands

Every value crosses from one command to the next by being read and pasted. No
block captures one command's output for another, and none assigns a variable.
`<target-repo>` is the path step 1 prints, substituted literally and left
unquoted; every other placeholder is quoted. `<ai-kit clone>` is the clone root
the runtime reference resolves: two directories above this skill's directory,
symlinks followed.

## Steps

### 1 — Settle the repository and the commit

```bash
git -C "<the path the user named, or the session's cwd>" rev-parse --show-toplevel
git -C <target-repo> rev-parse --verify --quiet "<ref, or HEAD when none was named>^{commit}"
```

`<sha>` is the full hash the second line prints, and it is what gets exported. Say
one line: `Audito <target-repo> en <ref> (<sha>).`

### 2 — Export the commit

```bash
mktemp -d
```

`<work-dir>` is the path it printed. If it sits under `<target-repo>`, stop: the
temporary directory has to be outside the audited repository. It is the only place
this skill writes.

```bash
mkdir "<work-dir>/code" &&
git -c filter.lfs.smudge= -c filter.lfs.process= -c filter.lfs.required=false -C <target-repo> archive --format=tar --output "<work-dir>/head.tar" "<sha>" &&
tar -xf "<work-dir>/head.tar" -C "<work-dir>/code" &&
rm "<work-dir>/head.tar"
```

`git archive` gives the code at the commit without touching `.git`, which
`git worktree add` would. The `filter.lfs` settings are there because `archive`
runs smudge filters, and an LFS smudge downloads into `.git`. The cost is that
the export has no ignored files.

### 3 — Detect the stacks

```bash
find "<work-dir>/code" \( -name node_modules -o -name vendor -o -name .venv \) -prune -o -type f \( -name package.json -o -name package-lock.json -o -name npm-shrinkwrap.json -o -name yarn.lock -o -name pnpm-lock.yaml -o -name bun.lock -o -name bun.lockb -o -name 'requirements*.txt' -o -name pyproject.toml -o -name uv.lock -o -name poetry.lock -o -name Pipfile -o -name Pipfile.lock -o -name setup.py -o -name go.mod -o -name Cargo.toml -o -name Gemfile -o -name pom.xml -o -name build.gradle -o -name build.gradle.kts -o -name composer.json -o -name '*.csproj' -o -name mix.exs \) -print
```

Read each manifest's path relative to `<work-dir>/code`:

- `package.json` and the JavaScript lockfiles are **Node**;
- `requirements*.txt`, `pyproject.toml`, `uv.lock`, `poetry.lock`, `Pipfile`,
  `Pipfile.lock` and `setup.py` are **Python**;
- `go.mod` is **Go**;
- anything else is a stack the standard does not cover, named by its manifest,
  and is not assessable.

`<stacks>` is one line in that shape, each stack with its manifests:
`Node: motor/package.json, motor/package-lock.json; Python: tools/requirements.txt; not covered: Rust (Cargo.toml)`.

### 4 — Cut the standard

```bash
awk '/^## Calibration/{exit} {print}' "<ai-kit clone>/standards/safe-to-expose.md" > "<work-dir>/standard.md" &&
grep -n '^## ' "<work-dir>/standard.md"
```

The headings it prints end at `## Checklist`, with no `## Calibration` among them.
`<work-dir>/standard.md` is the only standard any lens is given; the whole file
and its `## Calibration` section never reach one.

### 5 — Run the scanners

```bash
evidence --tree "<work-dir>/code" > "<work-dir>/evidence.json" &&
cat "<work-dir>/evidence.json"
```

Give this call a ten-minute timeout: a scan can take that long, and `evidence`
stops its own scanners before then and still prints a report. It exits 0 whenever
it printed one. The report is marked `"mode": "tree"`; each of `secrets`,
`dependencies` and `workflows` carries a `status` (`ran`, `partial`, `not-run` or
`not-applicable`), a `reason` on all but `ran`, a `scope`, its `scanners`, the
per-manifest `targets` of dependencies, and what it found under `found`; `sweep`
says whether the scanners' own leftovers were cleaned. Hold it in the
conversation for step 9; the `3-4` and `5-6` lenses read it from the file.

### 6 — Launch the four lenses

Four `audit-lens` subagents, launched together in one message so that they read in
parallel. On Claude Code the role is `ai-kit:audit-lens`. Where the host cannot
select a role by name, the runtime's fallback holds: the body of
`<ai-kit clone>/agents/audit-lens.md` goes to a fresh subagent with these inputs.

Each gets these fields and nothing else, `focus` set to `1`, `2`, `3-4` and `5-6`
in turn, and `evidence_path` only for `3-4` and `5-6`. No note, caveat or
instruction rides beside them:

```
focus: <1 | 2 | 3-4 | 5-6>
label: <last component of <target-repo>> at <ref> (<sha>)
code_dir: <work-dir>/code
standard_path: <work-dir>/standard.md
stacks: <stacks>
evidence_path: <work-dir>/evidence.json
```

**Wait for all four before step 7.** Step 8 deletes the directory they read.

### 7 — Check, merge and decide

An agent's own account is not the report. Three things are done here, in order.

**Every anchor has to exist at the commit.** That is each entry of every
finding's `anchors` and every control's `anchors`. Paste one line per anchor into
one block, `<first>` and `<last>` both the line for a single `path:line`:

```bash
awk -v a=<first> -v b=<last> 'FNR==a||FNR==b{print FILENAME":"FNR": "(NF?"present":"blank")}' "<work-dir>/code/<path>"
```

It prints whether the line holds anything and never what it holds, so no committed
secret reaches the conversation. An anchor exists when its first line prints
`present` and its last line prints at all. Any other anchor is dropped: no
output, `blank`, an error, or a path outside `code`. A finding or a control keeps
the anchors that exist; a finding left with none is dropped. Every dropped anchor
is kept for the report with its lens and what it was attached to.

**Merge.** Two findings are one when they share at least one item and an anchor of
one overlaps an anchor of the other: same path, line ranges that meet. The merged
finding keeps the fields of the higher severity, by CRITICAL > HIGH > MEDIUM >
NEEDS REVIEW, the union of both `items` and `anchors`, and every lens that
reported it. A finding marked `confirmed` with no exposure path, no missing control
or no exploit is `suspected` and NEEDS REVIEW, with the missing fact as
`would_confirm`.

**Decide the mandatory controls**, each by its items: M1 1.1, M2 1.2, M3 3.1, M4
4.1, M5 2.5 and 2.6, M6 4.2. A control is **absent** when any lens returned it
`absent`, when a confirmed finding sits on one of its items, or when no lens
returned it `present` with an anchor that survived. Otherwise it is **present**.
Not found is absent.

**Decide the verdict.** **fit to expose** only when no confirmed CRITICAL or HIGH
finding is left and all six controls are present. Otherwise **not fit to expose**.
Suspected findings never block.

### 8 — Clean up

```bash
rm -rf "<work-dir>"
```

It runs once every lens has returned and step 7 is done, before the report goes
out. If it fails, say so and name the directory.

### 9 — Print the report

In the user's language, every section in this order, and then stop. The template
is the shape:

```
Audit of `<target-repo>` at `<ref>` (`<sha>`)
Stacks: <stacks>

VERDICT: <fit to expose | not fit to expose>
- [CRITICAL] <item> <title> `<path>:<line>`
- [HIGH] <item> <title> `<path>:<line>`
- M<n> absent: <the control's name>

## Mandatory controls
| Control | Status | Anchor | Lens |
|---|---|---|---|
| M1 Login fails closed | <present | absent> | `<path>:<line>` | <focus> |

## Confirmed findings
1. [<severity>] <items> <title>
   `<path>:<line>`, `<path>:<first>-<last>`
   Exposure: <exposure_path>
   Missing control: <missing_control>
   Exploit: <exploit>
   Lens: <focus>, <focus>

## Suspected findings (NEEDS REVIEW)
- <items> <title> `<path>:<line>`
  Would confirm: <would_confirm>
  Lens: <focus>

## Scanner evidence (evidence --tree, mode: tree)
Secrets: <status>[: <reason>]. Scope: <scope>
  `<path>:<line>` <rule> <detail>
Dependencies: <status>[: <reason>]
  <ecosystem> `<path>`: <status>[: <reason>]
  <rule or aliases> in <package> <version>, `<path>`
Workflows: <status>[: <reason>]
  `<path>:<line>` <rule> <detail>
Sweep: <status>[: <reason>]
Scanners: <name> <version> (data of <database_date>)

## Readings chosen
- <item> (lens <focus>): <question> Reading: <reading>

## Not read
- Git history: not scanned, so item 3.1 covers the tree at <sha> only.
- Ignored files: the export holds only what git tracks at <sha>.
- <stack not covered> (`<manifest>`): not assessable against the standard.
- Item <item> (lens <focus>): not assessable: <reason>
- `<path>` (lens <focus>): <reason>
- Dropped anchor `<anchor>` (lens <focus>, <finding title or control>): not at <sha>.
```

- **The verdict line names everything that decides it**: after `not fit to
  expose`, every confirmed CRITICAL and HIGH and every absent control, one line
  each. After `fit to expose`, nothing.
- **The controls table has all six rows**, M1 to M6, each with the anchor that
  survived and the lenses that returned it; an absent control nobody anchored
  reads `not found` in the anchor column.
- **Confirmed findings are ranked** CRITICAL, then HIGH, then MEDIUM, and within a
  level by the lowest checklist item they touch, compared as numbers so that 2.10
  follows 2.9. Suspected findings are ordered by item the same way. Each finding
  keeps every anchor that survived step 7.
- **The scanner section holds facts from the JSON and nothing else**: each check's
  status, its reason when it has one, its scope, every entry of `found` one per
  line built only from its fields, and for dependencies every target with its
  status. A check that is not `ran` shows no count. It says no `clean`, no `safe`:
  no word that grades the app.
- **Not read** lists everything the four `files_not_read` named, every item a lens
  returned `not-assessable`, every stack not covered and every dropped anchor,
  however long it gets. The git history and ignored-files lines are always there.
- **An empty section is a real result.** No suspected findings, no readings
  chosen: say that none came up, in one line, and never pad.

Add nothing after it: no fix, no next step offered.

## Key principles

- **The bar is the standard's, and so is every rating.** The lenses rate by its
  rubric and Iron Law; this skill only merges, checks anchors and applies the bar.
- **A lens that has seen the answers is not a reader.** Calibration stays out of
  every copy a lens reads.
- **What can be checked is checked here.** Every anchor is read off the export, and
  an anchor that is not there is dropped and named.
- **The audited repository is read and never touched**, and that includes its
  index, its refs, its log and its live systems.
