---
name: bootstrap
description: "Give a repository the two files a newcomer — human or agent — needs to orient themselves: a `README.md` and a `CLAUDE.md`, both produced in one invocation and neither written without the user's agreement. What the repository declares about itself is gathered by an ordered reading procedure over its manifests, task runners, entrypoints and example env files rather than by inference or by a project-type classifier, so a repo that is a service *and* a dashboard gets both usage sections instead of being filed as one type. Nothing goes into either file that could have been obtained by asking the code — that is what `ask` is for, and prose restating mechanism is the prose that rots. Where the code misleads a reader who only reads it becomes an anchored `gotcha` in that repository's own memory log, which the `CLAUDE.md` points at instead of copying. Every command the README cites is checked back against the inventory before anything is written. On a repository that already has a README it regenerates the one section that enumerates that repo's surfaces, whatever that section is called, and changes nothing else in that file without your agreement. Read-only over the target repository apart from those two files and the records it appends, and it can be pointed at a repository other than the one it runs in. Triggers on: 'este repo no tiene readme', 'armá el readme de este repo', 'necesito un readme acá', 'generá el CLAUDE.md', 'falta el CLAUDE.md', 'documentá este repo', 'dejalo listo para que un agente lo entienda', 'onboardeá este repo', 'arrancá la documentación de este proyecto', 'actualizá la lista de partes del readme', 'el readme no lista todo lo que hay', 'bootstrap this repo', 'write a README for this repository', 'set up a CLAUDE.md', 'document this repo for agents', 'onboard this repository', 'the README's structure section is out of date'. It writes those two files and stops: it never authors an objective and never invokes `plan`, `work` or `debug`, and no step of those three ever invokes it."
---

# bootstrap

Two files, one invocation, for a repository this loop has never seen: a
`README.md` for whoever arrives at it, and a `CLAUDE.md` for whatever agent is
put to work in it.

One rule decides every sentence in both of them. **A sentence is written only if
it answers a question no other surface can be asked.** Where a function is
called, what order the middleware runs in, which struct carries the token —
those are questions the code answers at the moment of asking, and `ask` is the
surface that asks them, with a `path:line` on every claim. Written into a file
instead, the same sentence is true on the day it is written, unchecked for
however long the file lives, and indistinguishable from the sentences around it
that are still true. That is not a stylistic preference about documentation. It
is the failure that produced the previous system's rotted prose, and the
substitution test is the only defence that scales: before writing a sentence,
ask which surface could have been asked for it, and if one could, do not write
it.

What survives the test is small and it is exactly what nothing else holds. Why
this repository exists and who it is for. The command somebody actually runs,
which lives in a manifest or in nobody's head. The invariant that no file states.
The place where reading the code carefully leads you to the wrong conclusion.

So this skill spends most of its effort not on prose but on **finding out what
the repository declares about itself** — reading manifests, task runners,
entrypoint conventions and example environment files, and recording what each one
literally says, with the path it said it at. That inventory is what the README's
commands are checked against, and it is what decides which sections the README
gets: sections follow the surfaces the repository actually has, never a type it
was assigned. A repository with a Go service and a TypeScript dashboard is both
of those things, and a procedure that resolves it to one is a procedure that
silently drops half the repo.

Nothing here runs on its own. No step of `plan`, `work` or `debug` calls this
skill; a human invokes it, points it at a repository, and it produces two files
and stops.

## What it takes

```
repo_root: <absolute path of the repository to bootstrap, or the path the user named in words>
```

That is the only input. Everything else — what the repository is, what it
declares, whether it already has a README — is read in step 1 and step 2, and the
four things that cannot be read are asked once in step 3.

## Hard rules

1. **Read-only over the target repository except for exactly three writes.**
   `README.md`, `CLAUDE.md`, and records appended through the `memory` CLI. That
   is the whole list. No new directories beyond the `.ai-kit/` that the first
   `memory` command creates, no `docs/`, no scratch file "just to hold the
   inventory",
   no reformatting a file you happened to read, no fixing a typo you noticed.
   Anything you need to write for yourself goes to scratch outside the target
   repo.
2. **Neither file is written until the user agrees, in that conversation, having
   seen the content.** Not a summary of it — the content. A file written on
   inferred consent is a file in somebody's working tree that they did not ask
   for and now have to review after the fact.
3. **The substitution test governs every sentence in both files.** A sentence
   that answers a question `ask` could have answered off the code does not go in,
   however well it reads. The test is applied per sentence and not per section.
4. **The inventory reports what a file declares and never infers.** `package.json`
   declaring `"dev": "next dev"` is a fact about that file. "The dev server runs
   on port 3000" is not, unless something declared it. The distinction is the
   whole reason the inventory is worth building: a declaration can be pointed at,
   and an inference cannot be checked by anybody who comes later.
5. **No project-type classifier.** The repository is not resolved to one of
   service / CLI / library / frontend, and no section matrix keys off such an
   answer. Sections are chosen per surface the inventory found. A repository with
   two surfaces gets two usage sections.
6. **Every `gotcha` this skill writes carries at least one `--anchor`, and
   `--repo-root` alongside it.** `add` refuses an unanchored `gotcha` outright —
   `"at least one anchor is required: pass --anchor path:line or --anchor
   path:first-last"` — and it refuses one whose file or line range does not exist.
   The requirement does not relax because the target is somebody else's checkout;
   what changes there is that `--repo-root` supplies the tree the lines are read
   out of.
7. **No diagram is ever written to the memory log**, and the one diagram this
   skill may write goes in the `README.md` alone — never in `CLAUDE.md`, never in
   a record body.
8. **A README the skill did not write is not audited, retrofitted or tidied.**
   Two changes to an existing README are permitted and no others: the content of
   the single section that enumerates that repository's surfaces, by step 6; and,
   when no section enumerates them at all, **one new section added on the user's
   explicit agreement** — a new section in somebody's file needs the same
   agreement a new file does. Everything else in that file is somebody else's
   writing and stays exactly as it is, including the parts you can see are stale,
   and including the commands in it, which step 8 does not check.
9. **Nothing is asserted about the repository's own history, quality or
   roadmap.** Not "recently refactored", not "well tested", not "planned for the
   next release". None of it is derivable and none of it survives contact with the
   next month.
10. **A version, a framework's shape or a tool's flag is resolved, never
    recalled.** The manifest says which version is pinned; read it. When the
    README needs a claim about how a tool behaves, resolve it — `context7` if
    available, otherwise the pinned source in the dependency tree — or leave the
    claim out.
11. **This skill writes two files and stops.** No objective, no criteria, no fix,
    no invocation of `plan`, `work` or `debug`. When bootstrapping turns up work
    worth doing — a command in the CI that no manifest declares, a surface with no
    entrypoint at all — name it in one line and leave the next move to the user.
12. **Conversation in the user's language; everything written to disk in
    English.**

## The memory log

State lives in an append-only log, read and written only through the `memory`
CLI. Never edit the `.jsonl` by hand.

```bash
MEMORY="${CLAUDE_PLUGIN_ROOT}/bin/memory"
[ -x "$MEMORY" ] || ( cd "${CLAUDE_PLUGIN_ROOT}/tools/memory" && go build -o "$MEMORY" ./cmd/memory )
```

### The one directory that decides everything: the target repository

This skill is pointed at a repository, usually one other than the one it is
running in, and **the log it reads and writes is the target repository's log** —
not the log of the repo this session happened to start in. A constraint about how
some code misleads belongs next to that code; filed anywhere else it is a fact
nobody will ever query for, and the `CLAUDE.md` this skill writes would be
pointing at an empty log.

So settle the target once, at step 1, as an absolute path:

```bash
git -C "<the path the user named, or the session's cwd>" rev-parse --show-toplevel
```

It is written bare, with no `TARGET=$(…)` around it, and that is not an oversight:
**nothing in this turn says which shell it got**, and an assignment of a command
substitution is one of the shapes a shell may refuse outright — at parse time, so
that a block carrying it runs none of its lines. Nothing is lost by leaving it off
— a variable would not survive to the next command either, and what this line is
for is putting the path on stdout where you can read it and copy it.

**Empty output means the derivation failed, and a failed derivation stops the
skill.** `rev-parse` prints nothing on stdout when the path is missing or is not
inside a git repository. Say so and ask, rather than proceeding:

> No puedo resolver el repo desde `<the path>` — o no existe, o no está dentro de
> un repositorio git. Decime la ruta absoluta del repo que querés documentar y
> sigo desde ahí.

Then take the answer, confirm the path printed, and use it. **What must never
happen is a `memory` call with `--dir` empty**, because an empty `--dir` is not a
refusal — it resolves the log from the current directory instead, which on step 4
means `add` creating or appending to `.ai-kit/memory.jsonl` in a different
repository, at exit 0, with nothing in the output saying which one. And not only on
step 4: every command creates the location it resolves, so even step 1's reads
leave an `.ai-kit/` in whatever repository the flag pointed at. The refusal
below covers a path that *is* passed and is not a repo; it never fires on the
empty string, so this stop is the only thing standing between a mistyped path and
another repository's tree gaining a directory it never asked for.

`<target-repo>` in the commands below is a placeholder for the path this
derivation prints — the same notation as `<area>` and `<path-prefix>`, and not a
variable anything sets. **Substitute that absolute path literally into every
command**: each command runs in its own shell, so nothing carries the value
forward for you. The same goes for every file you read and both files you write —
`<target-repo>/README.md`, not `README.md`, because a relative path here writes
into whatever directory the session is standing in.

The notation is what makes forgetting survivable. `--dir "$TARGET"` left
unsubstituted is `--dir ""`, which is the silent fallback above — exit 0, in
whatever repository cwd happens to sit in. `--dir <target-repo>` left
unsubstituted is not a command at all: the shell reads `<` as a redirection, the
command fails with a non-zero exit, and `memory` never runs. Which failure you
get differs by shell and by where the placeholder falls in the line; the measured
exits and messages are in the log. A path you can read in the command is a path
you can see is wrong. Leave the brackets unquoted: `--dir "<target-repo>"`
parses, so the command runs unsubstituted and the brackets stop nothing, and
step 1's `[ -f <target-repo>/README.md ]` quoted and unsubstituted answers false
without a word. The quotes go in at substitution time instead, when the absolute
path you paste contains a space: unquoted, that test errors instead of answering,
and an error prints no `readme-exists` — indistinguishable from a README that is
not there, which sends the run to step 5 to author a new one.

Two flags carry the target, and both appear on every command that accepts them:

- **`--dir <target-repo>`** resolves *which log* — `<repo>/.ai-kit/memory.jsonl`.
  Accepted by `query`, `search`, `anchors`, `add`, `link`, `graph`.
- **`--repo-root <target-repo>`** resolves *which working tree* the anchors are
  relative to and read out of. Accepted by `anchors` and `add`, and by nothing
  else — do not put it on `query` or `link`, which refuse an undefined flag.

`--repo-root` defaults to the repo the log lives in, so on the same-repo case the
two coincide and writing both looks redundant. Write both anyway. The pair then
reads identically whether the target is this repo or somebody else's checkout,
which is the case this skill exists for, and neither value is ever left to be
resolved from wherever the shell happens to be standing.

Two measured behaviours, so neither surprises you:

- **A directory that is not inside a git repository is refused**: exit 1 with
  `code: bad_scope`, "not inside a git repository". That is a stop, and the honest
  response is to say the path is not a repo and ask for the right one.
- **A git repository with no log yet returns `count: 0` at exit 0, and creates the
  log while answering** — `<repo>/.ai-kit/`, an empty `memory.jsonl` and a
  `.gitignore` containing `*` that keeps the whole directory out of `git status`.
  Any command that resolves a location creates it, a read as much as a write, so
  neither the directory nor the file says anything about whether anything was ever
  written there. `count` does, and it is what step 1 reads: the number of records
  the CLI can parse out of the log, which is 0 for a log that does not exist yet and
  0 for one that exists holding nothing.

### What this skill writes, and under what

```
--plan repo-bootstrap   --kind gotcha        --anchor <path>:<first>-<last>
--plan repo-bootstrap   --kind open-question
```

`--plan repo-bootstrap` is **fixed**, and it is not whatever objective tree is in
flight in the session that invoked this skill. A constraint about how a
repository's code misleads belongs to no objective and outlives every one of
them; filed under a tree's slug it would disappear the moment that tree was
finished. On a fresh repository there is no tree to file it under at all.

What that costs is worth knowing rather than discovering: `work`'s start-up read
bounds its `gotcha` query by `--plan <tree-slug>`, so a constraint written here is
outside that bound and reaches a later session by the `--about` tags and by the
`CLAUDE.md` pointer, not by the tree filter. Tag accordingly — step 4 says how.

`--kind gotcha` is the kind for a constraint the code obeys without stating it,
and it is the right kind for exactly the interview answer that produced it:
something a reader of the code would get wrong, which nothing regenerates. It is
the one kind the CLI refuses without an anchor.

`--kind open-question` carries an interview probe that came back unanswered —
see step 3. It is not a placeholder for a `gotcha` you could not anchor; it is the
record that says nobody in the room knew, so the next session does not read the
silence as "there is nothing here".

`$MEMORY kinds` is the authority on which kinds and edge types exist; `$MEMORY
--help` on which flags do. A command this file does not show is one of those two
reads away.

## Steps

### 1 — Settle the target, and find out what is already there

Derive the target path first, by the section above, and keep it where you can read
it — it is the one value everything downstream depends on and this is the cheapest
moment to get it right.

Then read four things about it, in this order, because each one changes what the
rest of the run does:

```bash
ls -a <target-repo>
[ -f <target-repo>/README.md ] && echo readme-exists
[ -f <target-repo>/CLAUDE.md ] && echo claude-exists
$MEMORY query --dir <target-repo> --limit 1 \
  | python3 -c 'import sys,json; print("log-has-records" if json.load(sys.stdin)["count"] else "log-is-empty")'
```

**`README.md` present** routes the README half of this run through step 6 instead
of step 5: you are regenerating one section of somebody's file, not authoring a
new one.

**`CLAUDE.md` present** is a stop for that half until the user says otherwise.
Ask, in one line, whether to leave it, add to it, or replace it — and if the
answer is to add, treat its existing content the way hard rule 8 treats an
existing README: the only lines you touch are the ones you are adding.

**`log-is-empty`** is not an error and needs no `init`. Any `memory` command that
resolves a location creates it — including this very query and the three reads a few
lines below — so `.ai-kit/` and an empty `memory.jsonl` exist from that moment
whether or not anything is ever written to them.

That is why the count and not a test on the file. Its *existence* stopped being
evidence of anything the moment a read started creating it, and its *size* is only a
proxy: a log holding one blank line, or one line some older writer left broken, has
a size and no records. `query`'s `count` is the number of records the CLI can
actually read out of it, which is the question being asked — and `--limit 1` keeps
the answer from carrying any record bodies, since the count is all that is wanted
here.

`log-is-empty` means step 4's first `add` will be the first record this repository's
log has ever held, and **that gets announced when it happens** — not asked about,
announced. The consent for the write is the invocation; what the user is owed is
knowing that a log now exists in their tree with content in it and what that content
is. Nothing about this depends on running before the three queries below: a read
creates the file and writes no records into it, so the count still answers the same
question afterwards.

Then read what the log already holds, because on a repository the loop has seen
before this is where half the `CLAUDE.md` comes from and none of it should be
re-asked:

```bash
$MEMORY query   --dir <target-repo> --kind gotcha --current
$MEMORY anchors --dir <target-repo> --repo-root <target-repo> --kind gotcha --current
$MEMORY query   --dir <target-repo> --kind adr,decision --current
```

No `--plan` on any of the three: a constraint or a decision about this repository
may have been written under any tree, and bounding by one would hide exactly the
record that matters. The first two lines carry the same filter as each other, and
that identity is the point — a query narrower than the check verdicts records
nobody loaded, and a check narrower than the query loads records nobody verified.

**Read `truncated` before you say anything about what came back.** These queries
are deliberately unbounded by tree, which is exactly the shape that breaches the
withholding threshold — `$MEMORY kinds` reports it as `25`. Above it, `query`
returns `count: N`, `returned: 0`, `truncated: true` at exit 0, which is a log
full of records that this round has not read and which reads, at a glance, like an
answer. On a repository the loop has worked in for a while that is the likely
outcome, not the exotic one.

When it breaches, **narrow it — do not raise the limit and do not raise the
threshold**. Add `--about` with the path prefix of the surface you are about to
document (a tag ending in `/` matches by path prefix) and run the pair again, or
take one kind at a time, or reach for `$MEMORY search --dir <target-repo> "<a
distinctive phrase>"`. Narrow **both** the `query` and the `anchors` line
together, by the identity rule above. Raising `--limit` instead hands back the N
most recent and reports only how many older matches it dropped, and an `adr` is
exactly the record whose age is no argument against it. A wide `--threshold` is
the same trade paid in context: every body loaded into this thread so that one of
them can be cited in a `CLAUDE.md` pointer.

What comes back does three things. A `gotcha` already on disk is one you do not
re-interview and do not re-write. A `gotcha` whose anchor came back `changed` or
`could_not_check` is named to the user in this same round, with its title and its
`path:line`: nobody can tell from a hash whether the constraint still binds, and
this is the one moment somebody who knows the repo is in the conversation. An
`adr` or a `decision` on this repository is a candidate invariant for the
`CLAUDE.md` — as a pointer to the record, by step 7, never as a copy of its text.

Say in one line what these returned, including nothing. On a repository the loop
has never seen, nothing is the ordinary answer, and a filter that ran and found
nothing is different from a step that was skipped. **A `count: 0` is the only
thing that means nothing came back**: a `truncated: true` means the opposite and
must never be reported as an empty log, and `count: 0` itself cannot distinguish
*nobody wrote anything here* from *there is no log here* — what you know is that
the query came back empty, and that is what to say.

Finally, say back what you are about to do — which repository, which of the two
files are being created and which regenerated — and go. This is a readback, not
an approval gate; the approval gate is step 9, where there is something to
approve.

### 2 — Build the declaration inventory, in this order

This is the step everything else rests on. It is a **reading procedure**, not a
scanner: you read files, in the order below, and record what each one literally
declares along with the path it declared it at. There is no code to run and no
per-language parser to maintain, which is the trade — the inventory is not
byte-identical between two runs, and in exchange nothing has to be taught a fifth
language before it can read one.

Record every row as three things: **what you read**, **what it declares, in its
own words**, and **`path:line`**. A row with no path is not a row.

**a. The root, first.** `ls -a` output from step 1, plus `.gitignore`, plus the
top-level directory names. This is the surface list: every top-level directory
that holds code or configuration is a candidate surface, and the ones the
inventory ends up naming are what step 5's layout section enumerates and what any
diagram's nodes are drawn from.

**b. Every manifest, and every manifest — not the first one that matches.** A
repository may declare more than one ecosystem and the common case here is that it
does. Read all of them; nothing is skipped because something else was found
first.

- **Go** — `go.mod`: the module path and the `go` directive. `go.work` if
  present: every module in the workspace is a surface. Every directory containing
  a `package main` file is an entrypoint, `cmd/*` conventionally. A
  `.golangci.yml`, `.golangci.yaml` or `Makefile` lint target is what says how it
  is linted; the absence of one declares nothing.
- **Python** — `pyproject.toml`: `[project] name`, `version`,
  `requires-python`, `dependencies`, `[project.scripts]` (each entry is a
  console entrypoint), `[project.optional-dependencies]`, and whatever
  `[tool.*]` tables are present — `[tool.poetry]`, `[tool.uv]`, `[tool.ruff]`,
  `[tool.pytest.ini_options]` each declare a tool that is actually configured.
  Then `setup.py`/`setup.cfg`, `requirements*.txt`, `tox.ini`, `noxfile.py`,
  `Pipfile`, and a `__main__.py` anywhere as an entrypoint convention.
- **TypeScript / JavaScript** — `package.json`: `name`, `private`, every key of
  `scripts` verbatim, `bin`, `main`/`module`/`exports`, `engines`,
  `packageManager`, and `workspaces` — a `workspaces` array means every entry is
  its own surface with its own `package.json` to read. Then the lockfile, which
  is the only honest statement of which package manager is in use
  (`package-lock.json` → npm, `yarn.lock` → yarn, `pnpm-lock.yaml` → pnpm,
  `bun.lockb` → bun), and `tsconfig.json`, plus any framework config that
  declares an entrypoint by convention (`next.config.*`, `vite.config.*`,
  `astro.config.*`).
- **Rust** — `Cargo.toml`: `[package] name`, `edition`, `rust-version`,
  `[[bin]]` entries and `src/main.rs` as the default one, `[lib]`,
  `[features]`, and `[workspace] members` — again, every member is a surface.
  Then `rust-toolchain.toml` for the pinned toolchain.

**c. Task runners, whatever the language.** These are frequently more accurate
than the manifests, because they are what somebody types: `Makefile` targets (the
target name and its recipe), `justfile`, `Taskfile.yml`, `Rakefile`, `mise.toml`,
`docker-compose.yml` services and their `command`s, `Dockerfile` `CMD` and
`ENTRYPOINT`, `Procfile`.

**d. CI, which is the most honest statement of how the project is built and
tested.** `.github/workflows/*.yml`, `.gitlab-ci.yml`, `.circleci/config.yml`,
`Jenkinsfile`. Take the actual commands the steps run. A command that appears in
CI and in no manifest is a real command and belongs in the inventory with the
workflow file as its path — this is the row that most often rescues a quickstart.

**e. Environment and configuration, as declared.** `.env.example`,
`example.env`, `.env.sample`, `.env.template`, `config/*.example.*`, a
`values.yaml`, a settings schema. Every key in them, verbatim.

Then the part the declarations cannot give you, stated as its own thing so
nobody mistakes it for inventory: **a repository with no example file is not a
repository with no configuration.** When there is no example file, or when the
example file is visibly shorter than the surface it configures, read the source
for the variables it consumes — `os.Getenv` and `os.LookupEnv` in Go,
`os.environ` and `os.getenv` in Python, `process.env` in
TypeScript/JavaScript, `std::env::var` in Rust — and record each one with the
`path:line` it was read at. That is a *read*, not a declaration, and it is
recorded as one. It is also the single most common gap in this whole procedure,
because an uncommitted `.env` leaves no trace at all.

**f. Anything else that declares itself.** The four languages above are the ones
this procedure names, and they are not the world. A `pom.xml`, `build.gradle`,
`Gemfile`, `composer.json`, `CMakeLists.txt`, `mix.exs`, `pubspec.yaml` or
`*.csproj` is read the same way: find what it declares as its name, its
entrypoint and its runnable tasks, and record those with paths. **An
unrecognised manifest is reported by name and read literally, never skipped** —
skipping it is how the previous system's detector produced a README for half a
repository without ever saying which half.

**When the repository declares none of this** — no manifest, no runner, no CI, no
example file — say exactly that. Do not assemble a plausible quickstart out of
what repositories like this one usually do; a fabricated command is worse than an
absent one, because the reader will run it. What the inventory holds in that case
is the surface list from (a), and the quickstart is asked for in step 3 as the one
thing only the user knows. If they do not know either, the README says the build
command is not declared anywhere in the repository, which is a true and useful
sentence.

**On a monorepo**, the inventory will find several manifests and possibly several
READMEs. The procedure reads all of them and the surface list carries every
workspace member. What this skill does *not* do is decide which of several
READMEs is "the" README: say what you found and ask which file this run is about.

Close the step by putting the inventory in front of the user as a short list —
surfaces found, commands declared, configuration keys, and the gaps. It is
cheaper to be corrected here than anywhere later, and the person reading it can
usually spot a missing surface in one glance.

### 3 — Ask the four things the code cannot be asked

Everything else in this run comes off the repository. These four do not exist
anywhere in it, and the highest-value one is the second.

**One round trip, not four turns.** One `AskUserQuestion` carrying all four:

1. **What is this repository for, in one line, and who uses it?** The name of a
   module is not a purpose and a directory listing is not an audience. This is the
   first line of the README and nothing in the tree contains it.
2. **Where does the code mislead a reader who only reads it?** Where would a
   careful reader draw a conclusion that is wrong — a name that means something
   else, a value that looks configurable and is not, an order that looks
   incidental and is load-bearing, a retry count fixed by somebody else's rate
   limit. This is the question that extracts what reading the code would get
   *wrong* rather than what it says, and each answer becomes an anchored `gotcha`
   in step 4.
3. **What must an agent working here never do, or always do?** The invariant that
   no file states — a migration that is never edited after it ships, a generated
   file that is never hand-edited, a directory whose contents are somebody else's
   contract. These are the `CLAUDE.md`'s critical rules and there is no other
   source for them.
4. **Which command do you actually run to build, test and run this locally, and
   what does the inventory have wrong?** Ask this holding step 2's list, so the
   question is a correction rather than an interrogation. What comes back here is
   what fills a quickstart the declarations could not.

Do not ask anything the inventory answered. A question whose answer is in a file
you just read costs the user a turn to tell you what the repository already told
you, and it teaches them that answering carefully is optional.

**When the person in the room does not know this repository** — they inherited it,
they are bootstrapping somebody else's checkout — questions 2 and 3 have no honest
answer, and pressing produces invented constraints, which is the worst possible
content for a `CLAUDE.md`. Take "I don't know" as a complete answer and record the
probe rather than filling it:

```bash
$MEMORY add --dir <target-repo> --plan repo-bootstrap --kind open-question \
  --title "Where does this repository's code mislead a reader who only reads it? — unanswered at bootstrap" \
  --body "Asked during bootstrap; nobody in the conversation had worked in this repository.
The CLAUDE.md points at the gotcha log, which is empty on this surface for that reason and not because the code holds no traps." \
  --about <area>
```

Then the `CLAUDE.md` carries the commands and the pointer and no invariants, and
it says that the traps have not been collected yet. An empty section that says why
it is empty is worth having; a filled one that somebody guessed is not.

### 4 — Write the interview's constraints to that repository's log, anchored

Each answer to question 2 becomes **one `gotcha`, one record**, written now,
before either file is composed. One record per constraint and never one record per
interview: the anchor check reports per record, so three constraints bundled into
one body get one verdict between them, and a rewrite touching the first condemns
the other two.

**These records are written here and they stand whatever the user decides at step
9.** The log is append-only, so this is not a write that a later "no, don't write
the files" can take back, and it is not meant to be: what the user just told you
about how their code misleads is true independently of whether they want a README
today, and it is the one thing in this whole run that nothing regenerates. Step
9's gate is over the two files, and only over them. That asymmetry is deliberate,
and because it is the kind of thing a user discovers afterwards, it is stated to
them at the moment it happens rather than left in this file — see the announcement
at the end of this step, which says it in words whether or not this run is what put
the first records in the log.

The anchor is the misleading code itself. Ask which lines the answer is about if
that is not already clear — that is not ceremony, it is what makes the record
checkable at all, and the person who just described the trap is the only one who
can point at it:

```bash
$MEMORY add --dir <target-repo> --repo-root <target-repo> --plan repo-bootstrap --kind gotcha \
  --title "<the constraint, stated so a later session can check it>" \
  --body "<why the code is shaped this way, and what a reader of it would conclude instead>" \
  --about <area>,<path-prefix>/ \
  --anchor <repo-relative path>:<line> \
  --anchor <repo-relative path>:<first>-<last>
```

**The code it constrains goes in `--anchor`, not in the body.** `add` refuses a
`gotcha` carrying no anchor — `"at least one anchor is required: pass --anchor
path:line or --anchor path:first-last"`, exit 1 — and it refuses one whose file or
line range does not exist, so a citation you fumbled fails here rather than
surfacing months later as an unresolvable claim. `--repo-root <target-repo>` is what
lets it read those lines out of the target's working tree. The flag repeats; cite
every place the constraint reaches. Lines are 1-based and the range covers both
ends.

Draw the range **tight** — the lines the constraint actually binds, not the whole
function. Everything inside it is what a later rewrite is measured against: too
wide and every unrelated edit reports `changed`, and a record that cries wolf gets
ignored, which is worse than not having written it.

`--about` gets both the area and the path prefix, ending the prefix in `/`. That
is what a later session's filter reaches these by, and it matters more here than
anywhere else in this skill: these records are filed under `repo-bootstrap`, so a
`--plan`-bounded query in some other tree will not see them, and the tags are what
is left.

**An answer you cannot anchor is not written as a `gotcha`.** Two honest homes for
it: if it is about specific code whose location nobody can pin right now, it is an
`open-question` carrying what was said; if it is a repository-wide rule rather
than a claim about lines — "never hand-edit the generated client" — it was an
answer to question 3 and it belongs in the `CLAUDE.md`'s rules, not in the log.
What must not happen is an anchor drawn at a plausible-looking file to satisfy the
flag.

Then confirm what landed, and **announce the log if this run is what put anything
in it**:

```bash
$MEMORY anchors --dir <target-repo> --repo-root <target-repo> --plan repo-bootstrap --kind gotcha --current
```

`checked` equal to `matched` says every record under the filter is anchored;
`counts.still_holds` equal to `checked` says every anchor resolves against the
code right now, which for records written seconds ago is the only acceptable
answer. Anything else means a range was drawn at the wrong place — say so and
write the corrected record, since nothing already written is ever edited.

Then say what landed, in words, here and not at step 9 — including that it stands
whatever step 9 decides, because that is the part the user cannot infer and the
part an append-only log will not let them undo:

> Escribí `<n>` gotchas en el log de memoria de este repo. Quedan ahí pase lo que
> pase con los dos archivos: el log es append-only y esto no se deshace. Lo que
> vas a aprobar o rechazar al final son el `README.md` y el `CLAUDE.md`, no esto.

And if step 1 printed `log-is-empty`, these are the first records this repository's
log has ever held, so name the path in the same breath:

> Este repo no tenía nada guardado: estos `<n>` gotchas son los primeros de su log
> de memoria — `<target-repo>/.ai-kit/memory.jsonl`. Es un archivo JSONL append-only, y
> queda fuera de git por su propio `.gitignore` adentro de `.ai-kit/`, sin tocar el
> `.gitignore` del repo. El `CLAUDE.md` va a apuntar ahí.

### 5 — Compose the README, section by section

Only for a repository with no README; one that has one goes to step 6.

The sections below are the specification. **Four are always written** and the
rest are written when the inventory found the thing they are about — a
configuration section for a repository with no configuration is a heading with an
apology under it.

**Name and one-line purpose — always.** The name, and question 1's answer as one
sentence. Nothing else above the first heading.

**Quickstart — always.** The shortest path from a fresh clone to something
running or a test passing: prerequisites with their declared versions (from the
`go` directive, `requires-python`, `engines`, `rust-version`), then the commands.
Every command here traces to an inventory row or to question 4's answer, and step
8 checks that.

**Layout of top-level surfaces — always.** One line per surface from inventory
row (a): the path and what it is. Not a file listing and not a description of
what is inside each one — the path is what lets the reader go look, and the
looking is `ask`'s job.

**Pointers — always.** Where the reader goes next: `CLAUDE.md` for agent-facing
rules, the memory log for this repository's recorded constraints, and whatever
external surface the inventory turned up (a CI dashboard, a published package, a
deployment target). Pointers, not summaries.

**Usage — one section per surface, chosen by what the inventory found.** A
repository with an HTTP service and a CLI gets a usage section for each; a
repository with a service and a web dashboard gets both, and this is the case a
project-type classifier resolves to one and loses. What goes in each is what the
surface declares about being used: the command and its declared flags for a CLI,
the declared port and the entrypoint for a service, the install line and the
public entrypoint for a library, the dev command for a frontend. There is no
matrix and no priority order — the sections are the surfaces.

**Configuration — when the inventory found configuration.** The variables and
what each is for, from the example file's own keys or from the `path:line` where
the code reads them. A variable whose purpose nothing declares is listed with
what it is called and nothing invented about it.

**Development — when the inventory found a way to develop.** Test, lint and build
commands as declared, and the declared toolchain versions.

**Overview diagram — conditional, and this is the one place a picture earns its
keep.** It is written **only when the inventory found more than one piece that
hands off to another**. A single-module library gets no diagram, and neither does
a repository whose surfaces do not talk to each other. When it is written, it is
mermaid, boxes and arrows, at the level of what exists and who hands off to
whom:

- **Every node is a surface the inventory found.** No node for something you
  reasoned must exist.
- **Every edge is a hand-off you read** — an import across surfaces, a declared
  service dependency in `docker-compose.yml`, a client the manifest depends on.
  An edge you cannot point at is left out rather than drawn faintly; a diagram
  with a guessed edge in it is read with the same confidence as one without.
- **No internal call paths, no per-request sequence, no layers within a
  surface.** That is the densest possible restatement of what the code already
  carries, and it is what step 5's exclusion list refuses below.

#### What a README never receives

The substitution test decides every sentence, and these six are where it fails
most often and most expensively:

- **Per-feature prose.** How a feature works is a question the code answers, and
  the answer changes on the next commit while the paragraph does not.
- **Architecture deep-dives.** Layer-by-layer explanations of the internals. The
  overview diagram is the whole allowance for structure, and it stops at who hands
  off to whom.
- **Sequence diagrams and any per-request call-path rendering.** The densest
  restatement of the code there is, and the fastest to go wrong.
- **An API reference dump.** Endpoints, parameters, response shapes. Generated
  from the code or read from the code, never transcribed into prose.
- **A copy of the changelog or release notes.** They have a file; point at it.
- **A roadmap.** What is intended is not what exists, and a README that mixes
  them makes the reader unable to tell which sentences describe the repository in
  front of them.

State the reason rather than the list, because a list invites its own edge cases:
each of these fails the substitution test — every one of them answers a question
`ask` could answer off the code at the moment of asking, or a question that
belongs to a surface that already exists.

### 6 — When a README already exists: regenerate the one section that enumerates surfaces

The section that lists a repository's surfaces is the section that goes stale the
moment a surface is added, and nothing automates it. That single section is what
this step regenerates, and — apart from the one case below, where a README
enumerates no surfaces at all and the user agrees to a new section — it is the
only thing in an existing README this skill touches.

**Find it by matching against the inventory, never by heading name.** Read the
existing README's sections and score each one by how many of the inventory's
surface rows its lines name. The section that names most of them is the one — a
section naming three or more of them, where no other section names as many, is
the ordinary signal. It may be called `## Parts`, `## Layout`, `## Structure`,
`## Repository layout`, `## Packages`, `## What is in here`, or nothing at all —
it may be a fenced block under a heading about something else. **The procedure
does not require the heading to be called anything in particular**, because a
fixed heading name is exactly how a tool ends up regenerating nothing on a
repository that named its sections differently.

Two outcomes that are not "the section":

- **No section names the surfaces.** Then this README does not enumerate them and
  there is nothing to regenerate. Say so, offer to add one as a new section, and
  write it only if the user agrees — that is a new section in their file and it
  needs the same agreement as a new file.
- **Two sections score alike.** Say which two and ask. Guessing here rewrites the
  wrong half of somebody's README.

Then regenerate **that section's content and nothing else**:

- The heading stays exactly as it is, spelling and level included.
- The prose before and after the section stays exactly as it is, including the
  paragraphs that reference what the section used to say. If the regenerated
  content genuinely makes one of those paragraphs false, name it to the user in
  one line rather than editing it — hard rule 8 holds, and a prose paragraph
  somebody wrote is theirs to change.
- The section's existing format is preserved: a fenced two-column block stays a
  fenced two-column block, a bullet list stays a bullet list, a table stays a
  table. You are replacing rows, not restyling.
- Rows already there that still match the inventory keep their existing wording.
  Rewording a line that was already right produces a diff nobody can review.

Show the replacement as a diff of that section — before and after — in step 9,
and write it only on agreement. A section regenerated inside a file the skill did
not author is the change with the highest chance of destroying something, and the
user seeing the two versions side by side is the only real check on it.

### 7 — Compose the CLAUDE.md, as a router and nothing more

The `CLAUDE.md` is read at the start of every session in that repository, by
something that can read the entire repository itself. So it carries **only what
the code cannot be asked for**, and there are exactly three things:

1. **The build, test and run commands that are not derivable from the tree.** The
   incantation with the flag that has to be there, the environment variable that
   has to be set first, the service that has to be up. A command that is one
   `package.json` read away does not go here.
2. **The invariants no other surface states** — question 3's answers. Stated as
   rules, in the imperative, one line each. "Never hand-edit `client/generated/`;
   regenerate it with `make client`." An invariant with a reason is worth two
   without one.
3. **A pointer to where that repository's gotchas live in its memory log** — the
   command, not the content:

   ```bash
   memory query --kind gotcha --current
   memory anchors --kind gotcha --current
   ```

   The pointer and not a copy, for a reason that is the whole design: a copy in
   this file is unanchored and uncheckable, and the log's copy is anchored and
   `memory anchors` says whether it still describes the code. Copying it here
   would create the one thing this system exists to eliminate — a claim about code
   in a file, with nothing able to tell anybody whether it is still true.

   **This pointer is written even when there are no gotchas yet**, which on a
   fresh repository is the ordinary case. It is what gives them a home for when
   they appear, and the sentence is honest as written: this is where the
   constraints for this repository live.

**What the `CLAUDE.md` does not contain**, stated as flatly as the README's list:
**no architecture summary and no flow description.** No module map, no "the
request goes through X then Y", no directory inventory, no layer diagram, no
mermaid at all, no restatement of the README. Every one of those is a question
`ask` answers off the code with a `path:line`, and every one of them is wrong
within a month of being written.

Keep it short enough to be a router — a page, not a document. The length is not
an arbitrary ceiling: everything past the first page is content that failed the
substitution test and got written anyway, because the three things above do not
fill more than that. When it runs long, the fix is deleting what the code could
have been asked, never trimming the pointer.

### 8 — Run the commands check

Every command the produced README cites is compared against the declaration
inventory, before anything is written, as a step of this invocation.

Take each command string in the composed README — quickstart, usage, development,
every fenced block — and find the inventory row it came from. Report the result as
three lists:

**On the step 6 path the check covers the regenerated section and nothing else.**
The rest of that README is somebody else's writing, hard rule 8 says it is not
audited, and a command sitting in a quickstart this run never touched is outside
what this run is answering for. Checking it anyway would make this step exactly
the automatic doc-vs-code audit the last paragraph of this step says was
rejected — the only difference being that a human happened to start it. If one of
those unchecked commands is visibly wrong, hard rule 11 applies: name it in one
line and leave the next move to the user.

- **Declared** — the command appears in an inventory row, with that row's
  `path:line`. This is the ordinary case and it is what makes the README's
  commands checkable at all.
- **From the user** — the command came from question 4, and the row is the
  conversation. Named separately because it is trustworthy for a different reason
  and nothing in the repository backs it.
- **Unverifiable** — the command appears in **neither**. Report it with exactly
  that word:

  > `<command>` no aparece en ninguna declaración del repo — **no verificable**.
  > Puede ser correcto y no hay nada acá que lo respalde.

**Unverifiable is not wrong, and the wording matters.** The declared set is
incomplete by construction: an uncommitted `.env`, a command that lives in
somebody's shell history, a step in a deployment nobody wrote down. Reporting such
a command as an error would train the user to ignore the check, which costs more
than the check buys. What the report has to do is separate the commands with a
path behind them from the commands without one, and let the person who knows
decide.

An unverifiable command has two honest endings: the user confirms it, and it
becomes a "from the user" row; or nobody can confirm it, and it comes out of the
README. A command in a quickstart that nobody could confirm is the single worst
line in a README, because it is the first thing a newcomer runs.

**This check runs here and never automatically.** No hook invokes it, no step of
`plan`, `work` or `debug` calls it, and it does not become a background audit of
READMEs this skill did not write. The previous system's version of this was an
automatic audit of documented claims against machine-derived facts, and it was
rejected on purpose: an automatic check over prose nobody asked about produces
findings nobody acts on, every run, forever.

### 9 — Put it in front of the user, write on agreement, and stop

Show the whole thing, in the chat, before anything reaches the disk:

- The **`README.md`** in full — the composed file for a new one, or the
  before-and-after of the single regenerated section for an existing one.
- The **`CLAUDE.md`** in full.
- The **commands check**, with its three lists.
- **The gaps**: what the inventory could not establish, the questions that came
  back unanswered, the surfaces with no declared entrypoint. This is not an
  apology paragraph — it is the part the user would otherwise discover by acting
  on a hole.

Then ask, once, whether to write. **Silence is not agreement and neither is
enthusiasm about the content.** An edit they hand back is the decision; take it
literally and do not re-litigate it.

On agreement, write exactly the files that were shown, at absolute paths under
`<target-repo>`, and nothing else — no directory, no `docs/`, no scratch file left
behind. Then say what landed: the two paths, and the log path if step 4 wrote the
first records into it.

**On a refusal, neither file is written and the run ends there** — no partial
write, no "I'll leave the CLAUDE.md at least", no file in scratch for later. Say
what that leaves behind, because it is not nothing: step 4's `gotcha` records are
on disk and stay there, by the rule stated in that step, and the composed files
exist only in this conversation. If they want the prose without the files, it is
already above in the chat.

Then stop. If the run turned up work worth doing — a command in CI that no
manifest declares, a surface with no entrypoint, a `gotcha` whose anchor came back
`changed` in step 1 and that nobody settled — name it in one line and leave the
next move to the user. Turning it into an objective is `plan`'s job, invoked by
them.

## Key principles

- **A sentence is written only if it answers a question no other surface can be
  asked.** That is the whole content rule, applied per sentence, and it is what
  keeps both files small enough to stay true.
- **Declarations are read; nothing is inferred.** What a manifest, a runner or a
  CI step literally says can be pointed at by whoever reads the README next. What
  a repository "probably" does cannot be checked by anybody, ever.
- **Sections follow the surfaces, never a type.** A repository is allowed to be
  three things at once, and a procedure that resolves it to one silently documents
  a third of it.
- **The traps go in the log, anchored; the files point at them.** A constraint
  copied into `CLAUDE.md` is uncheckable the moment the code moves. The same
  constraint in the log with an anchor tells a later session, mechanically,
  whether it still describes the code.
- **A command with no path behind it says so.** Unverifiable, not wrong — and
  never quietly promoted to verified by being printed in a fenced block.
- **One picture, one level, one place.** An overview of who hands off to whom, in
  the README, only when there is more than one piece. Everything below that level
  is the code restating itself.
- **The target repository is read, not touched.** Two files it asked for and its
  own log. Not a directory, not a `docs/`, not the typo you noticed on the way
  past.
- **It bootstraps and stops.** A human invokes it, it produces two files, and
  what happens next is theirs to decide.
