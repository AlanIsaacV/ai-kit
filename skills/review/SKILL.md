---
name: review
description: "Put one change in front of the person who has to decide on it, as rules instead of lines: at most ten rules the change assumes, highest criticality first, each with its level, the rule in one sentence, a `path:line` that exists at the reviewed ref and where to read, on one screen, naming every changed file left unread. On a branch or a range it puts a short block of automatic evidence between the rules and that list: secrets in added lines, new dependency advisories and insecure workflow lines, what is inherited as a count, and every check that did not run with its reason. Takes a local repository and a branch, a commit range or a PR number; a bare branch resolves against its merge-base with the default branch. It launches the read-only `rules` agent, which reads the diff and what the changed code depends on, because an inherited rule lives outside the changed lines. The criticality is inferred and the report says so. Read-only over the reviewed repository: nothing reaches its working tree, its `.git` or its memory log, and the report lives in the conversation. Use when somebody has to decide on a branch or a PR without reading the whole diff. Triggers on: 'revisá esta rama', 'revisá este PR', 'revisá este rango', 'qué reglas asume este cambio', 'qué asume esta rama', 'qué reglas asume este PR', 'quiero decidir sobre este PR sin leer el diff', 'review this branch', 'review this PR', 'review this range', 'what rules does this change assume', 'what does this branch assume', 'help me decide on this PR without reading the diff'. It criticises no code, which is `debrief`'s job over code the person wrote, it does not explain how a repository works, which is `ask`'s, and it never edits, never commits and never invokes `plan`, `work` or `debug`."
---

# review

Read [the host runtime](references/runtime.md) first for tool invocation and
delegation. Its memory preflight does not apply here: this skill runs no `memory`
command.

Put one change in front of the person who has to decide on it, as the rules it
assumes rather than the lines it touches. What escapes a review is rarely a line.
It is something the change takes for granted about the data or the business, often
inherited from code the change never mentions, so a diff read carefully still does
not show it.

This skill is the door and the orchestrator. It settles the repository, resolves
the input to a base and a head, puts the code and the diff in a temporary
directory, launches `rules`, checks what comes back, runs `evidence`, cleans up and
prints one screen. The `rules` agent reads and returns, and it never sees the
evidence.

It shares a name with the `review` agent and is not that agent. `work` step 7
launches `review` over code an agent wrote, against an objective, and it returns
findings on the code. This skill needs no objective and reports no code criticism.

## Hard rules

1. **Read-only over the reviewed repository.** Nothing reaches its working tree,
   its `.git` or its memory log. The git commands below are all this skill runs
   there, and each of them only reads. No `git fetch`, no `git worktree add`, no
   `git checkout`, and no `git status`, which refreshes the index behind your back.
2. **No `memory` command, against any repository.** Any `memory` read against a
   target creates `<target>/.ai-kit/` while it answers, so a single query would
   break rule 1 without a word.
3. **A stop launches nothing.** A ref that does not exist, a range with no changes
   and a PR that cannot be read each end the run with a message naming what
   failed, before any agent starts.
4. **`rules` reads only the temporary directory.** It gets no path to the reviewed
   repository, which is the one place this differs from what the runtime says about
   `repo_root`: an agent with no path to the target cannot write to it.
5. **The report lives in the conversation.** No file, no record, no copy kept.
6. **Nothing unread passes as read.** Every changed file `rules` did not read is
   named in the report, however long that list gets. Coverage is computed by
   subtraction, so a file nobody mentioned counts as unread.
7. **The criticality is inferred, and the report says so.** Nobody declared it for
   this repository.
8. **It reports and stops.** No verdict, no recommendation to merge or not, no code
   criticism, no fix. The decision is the person's.
9. **Conversation in the user's language; everything written to disk in English.**
   The agent answers in English and you put the report in their language.
10. **A check that did not run never reads as clean.** It appears with its reason
    and with no count, and a missing `evidence` is reported the same way.

## Commands

Every value crosses from one command to the next by being read and pasted. **No
block here captures one command's output for another**, and none assigns a
variable: nothing says which shell this turn got, and a shell that refuses one of
those shapes refuses the whole block before any of its lines has run.

`<target-repo>` is the path step 1 prints, substituted literally into every command
and left unquoted, so that a command that forgot it cannot run at all. Every other
placeholder is quoted.

## Steps

### 1 — Settle the repository

```bash
git -C "<the path the user named, or the session's cwd>" rev-parse --show-toplevel
```

**Empty output stops the skill**: the path is missing or is not inside a git
repository.

> No puedo resolver el repo desde `<the path>`: o no existe, o no está dentro de un
> repositorio git. Decime la ruta absoluta y sigo desde ahí.

### 2 — Resolve the input to a base and a head

Read the input by its shape, in this order:

- **A PR**: `123`, `#123` or a URL ending in `/pull/123`. Go to **A PR** below.
- **A range**: anything with `..` in it. `A...B` reads from their merge-base to `B`;
  `A..B` reads from `A` to `B`, exactly as written. An empty side is `HEAD`.
- **A ref**: anything else, usually a branch, and also a tag or a commit. This is
  `<ref>` below.
- **Nothing named**, as in "esta rama": the current branch, which is `<ref>`.

  ```bash
  git -C <target-repo> branch --show-current
  ```

  Empty output is a detached head, and there you ask what to review.

**Every ref resolves the same way**, one call per ref so that an empty output says
which one failed:

```bash
git -C <target-repo> rev-parse --verify --quiet "<ref>^{commit}"
```

It prints the commit. When it prints nothing, try the same command once more with
`refs/remotes/origin/<ref>` as the ref, since nothing here fetches and a branch may
only exist as a remote-tracking ref. **Empty both times stops the run**, and no
agent is launched:

> No existe `<ref>` en `<target-repo>`, ni local ni en `origin`. No hago fetch y no
> lancé ningún agente. Decime otra rama, un rango o un PR.

On a range, both ends go through that, and for `A...B` the base is what this prints:

```bash
git -C <target-repo> merge-base "<A>" "<B>"
```

On `<ref>`, the head is that commit and the base is its merge-base with the
repository's default branch, which needs finding, because nothing local says it
for certain. Candidates, in this order: what the first command prints, then
`origin/main`, `origin/master`, `main` and `master`, keeping those the second one
lists.

```bash
git -C <target-repo> symbolic-ref --quiet --short refs/remotes/origin/HEAD
git -C <target-repo> for-each-ref --format='%(refname:short)' refs/remotes/origin/main refs/remotes/origin/master refs/heads/main refs/heads/master
```

Then take the candidates in order, one call each, and **the first that prints a
commit is the base**:

```bash
git -C <target-repo> merge-base "<candidate>" "<head>"
```

`origin/HEAD` comes first and is still only a candidate, because it is written at
clone time and nothing keeps it current. It can name a branch abandoned long ago
that shares no history with what is under review, and then `merge-base` prints
nothing and exits non-zero, which is what rejects it. Pasting an empty base into a
diff would compare the head against the working tree instead, and at exit 0. When
no candidate prints a commit:

> No encontré la rama por defecto de `<target-repo>` contra `<ref>`. Pasame un rango
> `base..head` y sigo.

**With a base and a head, check that there is something to read:**

```bash
git -C <target-repo> diff --no-ext-diff --no-textconv --shortstat "<base>" "<head>"
```

**Empty output stops the run**, and no agent is launched. A branch that is the
default branch itself lands here too, since its merge-base is its own tip.

> No hay cambios entre `<base>` y `<head>`: no hay nada que revisar y no lancé ningún
> agente.

Otherwise say in one line what you are about to read, then go on. It is a readback
and not a question, and it is where a wrong base shows up, as a file count nobody
expected:

> Reviso `<ref o rango>`: base `<base corta>` (`<de dónde salió>`), head
> `<head corta>`, `<N>` archivos.

#### A PR

Resolve the GitHub repository from the target, unless the URL already names it:

```bash
git -C <target-repo> remote get-url origin
```

Read `<owner>/<name>` off what it prints. No `origin`, or a remote that is not on
GitHub, stops the run and asks for the PR's URL. `--repo` goes on every `gh` call,
so nothing depends on where this shell is standing.

```bash
gh pr view "<number>" --repo "<owner>/<name>" --json number,title,baseRefName,baseRefOid,headRefName,headRefOid,changedFiles
```

**A failure stops the run**, and no agent is launched: a PR that does not exist, a
`gh` with no access, a missing `gh`. Name the PR and repeat what `gh` said.

> No pude leer el PR `<number>` de `<owner>/<name>`: `<lo que dijo gh>`. No lancé
> ningún agente.

`changedFiles` at 0 stops the same way, with the no-changes message. The base is
`baseRefOid` and the head is `headRefOid`. Say the same readback line, then go on
to step 3.

### 3 — Put the change in a temporary directory

The temporary directory is the only place this skill writes.

```bash
mktemp -d
```

`<work-dir>` is the path it printed, pasted literally from here on. If that path
sits under `<target-repo>`, stop: the temporary directory has to be outside the
reviewed repository.

For a branch or a range:

```bash
mkdir "<work-dir>/code" &&
git -c filter.lfs.smudge= -c filter.lfs.process= -c filter.lfs.required=false -C <target-repo> archive --format=tar --output "<work-dir>/head.tar" "<head>" &&
tar -xf "<work-dir>/head.tar" -C "<work-dir>/code" &&
rm "<work-dir>/head.tar" &&
git -c core.quotePath=false -C <target-repo> diff --no-color --no-ext-diff --no-textconv "<base>" "<head>" > "<work-dir>/change.diff" &&
git -c core.quotePath=false -C <target-repo> diff --name-only "<base>" "<head>" > "<work-dir>/changed.txt"
```

`git archive` gives the code at the ref without touching `.git`, which
`git worktree add` would. The `filter.lfs` settings are there because `archive`
runs smudge filters, and an LFS smudge downloads into `.git`. The `--no-ext-diff`
and `--no-textconv` keep the repository's own diff configuration out of the run.

The cost is that the code has no gitignored files, so a repository that keeps its
docs out of git has no docs here.

For a PR, the diff comes from GitHub, and the code comes over `gh api` as a tarball
of the exact head commit, because a `git fetch` would write to `.git`:

```bash
mkdir "<work-dir>/code" &&
gh pr diff "<number>" --repo "<owner>/<name>" > "<work-dir>/change.diff" &&
gh pr diff "<number>" --repo "<owner>/<name>" --name-only > "<work-dir>/changed.txt" &&
gh api "repos/<owner>/<name>/tarball/<headRefOid>" > "<work-dir>/head.tar.gz" &&
tar -xzf "<work-dir>/head.tar.gz" --strip-components=1 -C "<work-dir>/code" &&
rm "<work-dir>/head.tar.gz"
```

**When either block fails, go to step 7's cleanup and stop**, repeating the error,
with no agent launched. GitHub refuses the diff of a very large PR; when that is
what happened, say so and offer the range, if the branch is local.

### 4 — Launch `rules`

One subagent, with these inputs and nothing else:

```
label: <the ref, the range or the PR, as the readback named it>
code_dir: <work-dir>/code
diff_path: <work-dir>/change.diff
changed_files_path: <work-dir>/changed.txt
```

When it fails or returns nothing usable, go to step 7's cleanup and say so. A report
built without it does not exist.

### 5 — Check what came back

The agent's own account is not the report. Two things in it are checkable, and
both are checked here against the files.

**Every anchor has to exist at the reviewed ref.** Paste one pair of lines per rule
into one block, with `<path>` and `<line>` taken from its `anchor`:

```bash
awk 'NR==<line>{print FILENAME":"FNR": "$0}' "<work-dir>/code/<path>"
grep -Fxc "<path>" "<work-dir>/changed.txt"
```

- `awk` printing nothing, or a line with nothing on it, means the anchor does not
  exist. That rule is dropped and the report says how many were.
- `grep` printing `0` means the file was not touched: the rule is inherited. Any
  other number means it was. Take this over the agent's `inherited`.

Put the rules in order: `security` and `money` together, then `business`,
`experience`, `internal`. Keep the first ten.

**Unread is computed, not told.** Paste `files_read` as the arguments here, one per
path:

```bash
printf '%s\n' "<path>" "<path>" > "<work-dir>/read.txt"
```

```bash
grep -Fxv -f "<work-dir>/read.txt" "<work-dir>/changed.txt" > "<work-dir>/unread.txt"
wc -l "<work-dir>/changed.txt" "<work-dir>/unread.txt"
```

Read `<work-dir>/unread.txt` whole, past any limit a tool puts on it: on a large
range it is the longest thing in this run and the one thing here that may not be
shortened.

### 6 — Gather the evidence

**On a PR there is nothing to run**: no base is exported on that path, so go to
step 7, and the report says so.

On a branch or a range, the base goes beside the code here rather than in step 3,
so that it does not exist while `rules` is reading. `evidence` gets only paths
inside the temporary directory, never `<target-repo>`.

```bash
mkdir "<work-dir>/base" &&
git -c filter.lfs.smudge= -c filter.lfs.process= -c filter.lfs.required=false -C <target-repo> archive --format=tar --output "<work-dir>/base.tar" "<base>" &&
tar -xf "<work-dir>/base.tar" -C "<work-dir>/base" &&
rm "<work-dir>/base.tar" &&
evidence --head "<work-dir>/code" --base "<work-dir>/base" --diff "<work-dir>/change.diff"
```

Give this call a ten-minute timeout: a scan can take that long, and `evidence` stops
its own scanners before then and still prints a report. Its standard output is one
JSON report. Hold it in the conversation: it goes in no file and to no agent, and
step 8 is built from it.

Each of `secrets`, `dependencies` and `workflows` carries a `status`, which is one
of `ran`, `partial`, `not-run` or `not-applicable`, with a `reason` on all but the
first. Its `new` lists what this change introduced, `inherited` counts what was
already at the base, and `scanners` names what ran. In `dependencies`, `targets`
says manifest by manifest what was scanned and what was not.

**A failure here stops nothing.** The rules are still the report. If the block
fails, or what it prints is not that JSON, go on to step 7 and step 8 says evidence
did not run, repeating the error.

### 7 — Clean up

```bash
rm -rf "<work-dir>"
```

This runs on every path that reached step 3, and it runs before the report goes out,
so the report is the last thing the person sees. If it fails, say so and name the
directory.

### 8 — Print the report

One screen, in the user's language, and then stop. The template is the shape:

```
Reglas que asume `<ref, rango o PR>`, `<base corta>..<head corta>`. Leí <leídos> de <total> archivos cambiados.
Criticidad inferida: la deduje del código, nadie la declaró para este repositorio.

1. [<nivel>] <la regla, en una oración> (heredada)
   `<path>:<line>` · leer: <dónde leer>
2. [<nivel>] <la regla, en una oración>
   `<path>:<line>` · leer: <dónde leer>

Quedaron <n> reglas fuera del corte de 10.
Descarté <n> reglas con un ancla que no existe en este ref.

Evidencia sobre lo que introduce este cambio:
Secretos: <n> nuevos
  `<path>:<line>` <regla>
Dependencias: <n> nuevas, <n> heredadas
  <advisory> en <paquete> <versión>, `<path>`
Workflows: <n> nuevos, <n> heredados
  `<path>:<line>` <regla> <detalle>
No corrió: <check o manifiesto>: <razón>
Escáneres: <nombre> <versión> (datos del <fecha>); Go <toolchain>, go.mod <versión>

Sin leer (<n>):
<path>
<path>
```

- The levels read `seguridad`, `dinero o posiciones`, `negocio o regulatorio`,
  `experiencia` and `interno`, or the same in the user's language.
- `(heredada)` goes on a rule whose anchor is in a file the change did not touch.
  When no rule has it, say so in one line instead: `Ninguna regla heredada: todas
  caen en archivos del cambio.` Never invent one.
- Each rule is two lines and no more. A rule that does not fit gets shortened, not
  wrapped onto a third.
- The line about rules past the cut and the line about dropped rules appear only
  when they are not zero.
- **The evidence block goes between the rules and `Sin leer`** and holds facts from
  the JSON and nothing else. It says no `limpio`, no `seguro`, no `sin riesgo`: no
  word that grades the change.
- A finding is one line, every one in `new` is listed, and each line is built only
  from the fields the JSON gives. `<detalle>` appears when the finding has one.
- A check with status `ran` shows its counts, including a zero. `secrets` has no
  `inherited` and shows none. `partial` shows its counts too, with a `No corrió` line
  for every target whose status is `not-run`.
- A check with status `not-run` shows no count: `Secretos: no corrió: <razón>`.
  `not-applicable` reads `Secretos: no aplica: <razón>`. `dependencies` gets a
  `No corrió` line per target here as well.
- `Escáneres` names each scanner in `scanners`, its version, and its `database_date`
  unless that is `not-applicable`. One with `at_pin` false shows both versions:
  `gitleaks 8.31.0 (fijada 8.30.1)`. For Go, `toolchain` and `go_mod` go side by
  side, because they can differ and the reader decides what that means.
- On a PR the whole block is one line: `Evidencia: no corrió: en un PR no se exporta
  la base.` When `evidence` itself failed it is one line too: `Evidencia: no corrió:
  <el error>.`
- `Sin leer` lists every path in `unread.txt`, one per line, and goes last so that
  a long list does not push a rule off the screen. When nothing was left unread, the
  section is not there and the count in the first line says it.
- **No rules is a real report.** Say that none came up and say how much was read.

Add nothing after it: no summary of the change, no opinion on merging, no next step
offered.

## Key principles

- **The rule is the unit.** Lines are what the diff already shows.
- **An inherited rule is the reason this exists**, and it cannot be seen from the
  changed lines, which is why `rules` gets the code as well as the diff.
- **Evidence is what this change introduced.** A report of the whole repository
  would read the same on every branch, so what is inherited is a count and nothing
  more.
- **What can be checked is checked here.** An anchor is read off the code and
  coverage is subtracted from the changed files, because a report that is trusted
  without the diff has nothing else standing behind it.
- **The reviewed repository is read and never touched**, and that includes the
  index, the refs and the log.
