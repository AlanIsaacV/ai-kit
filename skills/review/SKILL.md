---
name: review
description: "Put one change in front of the person who has to decide on it, in one report they can decide from without asking anything afterwards: what the change is, how it works with `path:line`, the business rules it implements, its impact inside the repository, what it touches outside the repository (each service, table, cache, queue, incident or ticket it reaches, checked with the host's read-only tools and the trace each returned, and what could not be checked and why), what was tested and how (each test with what it stands on, the changed paths no test exercises, and the PR's CI), what is missing and what could be better against the intent its PR or its commits declare, at most ten rules it assumes, highest criticality first, each with what its anchored code is for and who uses it, what can go wrong, a short block of automatic evidence on a branch or a range (secrets in added lines, new dependency advisories, insecure workflow lines, every check that did not run with its reason), and every changed file left unread. Takes a local repository and a branch, a commit range or a PR number; a bare branch resolves against its merge-base with the default branch. A branch, a range or a PR whose commits are local, when its commits fall into two or more topics, is first shown as a map of those topics, each with its commits and its files of the final diff counted as added, modified, deleted and shared: one topic per conventional-commit scope, read tolerantly so `Type (scope):` counts, and one per unscoped commit, or, when no commit has a scope, one per commit's dominant directory, plus a last `sin-tema` topic holding the files only merges changed. Nothing is launched until topics are named as `review <range> tema <name>`, or several as `tema a,b`, which are reviewed as one union: the final diff cut down to their files, judged against their own commits, with an unread list of their files only; a name not on the map stops the run and lists the valid names. It launches four read-only agents in parallel over an export of the reviewed ref: `explore` explains the change, `rules` finds what it assumes, including rules inherited from code the change never mentions, `evaluate` judges what is missing and what could be better, and `outside` checks what the change reaches at runtime with whatever read-only tools the host exposes, discovered at run time, calling only those whose description says they read (on Codex `outside` does not run, because Codex ignores its `disallowedTools` and its read-only sandbox does not cover MCP tool calls, and the outside section reads `No corrió outside`); and it reads the why from the reviewed repository's memory log the way `ask` does. The criticality is inferred and the report says so. Read-only over the reviewed repository: nothing reaches its working tree or its `.git`, the log read only leaves its `.ai-kit/` behind, nothing from the reviewed code runs, `outside` writes nothing in any system it reads, and the report lives in the conversation. Use when somebody has to decide on a branch or a PR without reading the whole diff. Triggers on: 'revisá esta rama', 'revisá este PR', 'revisá este rango', 'qué reglas asume este cambio', 'qué asume esta rama', 'qué reglas asume este PR', 'qué le falta a este PR', 'quiero decidir sobre este PR sin leer el diff', 'review this branch', 'review this PR', 'review this range', 'what rules does this change assume', 'what does this branch assume', 'what is this PR missing', 'help me decide on this PR without reading the diff'. It gives no verdict and no merge recommendation and writes no fix; it never edits, never commits and never invokes `plan`, `work` or `debug`, and `ask` stays the door for follow-up questions about the repository."
---

# review

Read [the host runtime](references/runtime.md) first for tool invocation,
delegation and the memory prerequisite. Two things here differ from what it says:
the readers get no path to the reviewed repository (rule 4), and the log is read
with `--dir` rather than from inside the target.

Put one change in front of the person who has to decide on it, in one report they
can decide from without asking anything afterwards: what the change is and how it
works, what it reaches outside its repository, what it was tested with, what it
left undone, the rules it assumes and what can go wrong. What escapes a review is
rarely a line. It is something the change takes for granted about the data or the
business, often inherited from code the change never mentions, so a diff read
carefully still does not show it. And a rule
read alone does not show it either: without what the change is for and how it
works, the reader knows what to look at and not whether it matters.

This skill is the door and the orchestrator. It settles the repository, resolves
the input to a base and a head, sorts the commits into topics and stops on a map
of them when there are two or more and none was named, puts the code and the diff
in a temporary directory, reads what the change says it is for, launches
`explore`, `rules`, `evaluate` and `outside` in parallel over that directory,
reads the why from the reviewed repository's log and the CI from GitHub, checks
what came back, runs `evidence`, cleans up and prints the report. The four agents read and return, and
none of them sees the evidence, the log or the CI.

It shares a name with the `review` agent and is not that agent. `work` step 7
launches `review` over code an agent wrote, against an objective, and it returns
findings on the code. This skill needs no objective; what it reports as missing or
improvable is `evaluate`'s reading against what the change itself declares.

## Hard rules

1. **Read-only over the reviewed repository.** Nothing reaches its working tree or
   its `.git`, with the one exception rule 2 names. The git commands below are all
   this skill runs there, `git log` and `git cat-file` among them, and each of them
   only reads. No `git fetch`, no `git worktree add`, no `git checkout`, and no
   `git status`, which refreshes the index behind your back.
2. **The log is read and never written, and only the reviewed repository's.**
   `memory query` and `memory search` against `<target-repo>`, and no other
   `memory` subcommand. Any `memory` read creates `<target-repo>/.ai-kit/` while it
   answers, with a `.gitignore` inside holding `*` that keeps it out of
   `git status`. That directory is the one thing this skill leaves in the reviewed
   repository, and it is accepted. The export is never a `--dir`: it is not a git
   repository, `memory` refuses it, and the log belongs to the repository anyway.
3. **A stop launches nothing.** A ref that does not exist, a range with no changes,
   a topic that is not on the map or has no files, and a PR that cannot be read
   each end the run with a message naming what failed, before any agent starts.
   So does a map of topics waiting for the person to name one.
4. **The agents read only the temporary directory, with one exception.**
   `explore`, `rules`, `evaluate` and `outside` get no path to the reviewed
   repository, which is the one place this differs from what the runtime says
   about `repo_root`: an agent with no path to the target cannot write to it.
   `explore` is told not to use the code graph, whose index describes a checkout
   and not the reviewed commit. The log and the CI are read by this skill, never
   by an agent. The exception is `outside`, the only agent that reads beyond the
   temporary directory, and only through the read tools the host exposes, each
   called only when its own description says it reads and writes nothing
   anywhere. It runs no shell, so no command line reaches anything from it.
5. **Nothing from the reviewed code runs.** No build, no tests, no install, no
   script it ships, here or in any agent. What was tested is read off the test
   files and the CI, never learned by running them.
6. **The report lives in the conversation.** No file, no record, no copy kept.
7. **Nothing unread passes as read.** Every changed file that none of the four
   agents read is named in the report, however long that list gets. On a topic,
   the changed files are the topic's. Coverage is
   computed by subtraction, so a file nobody mentioned counts as unread.
8. **The criticality is inferred, and the report says so.** Nobody declared it for
   this repository.
9. **It reports and stops.** No verdict, no recommendation to merge or not, no fix
   and no replacement code. What is missing and what could be better are
   `evaluate`'s findings against the declared intent, each with its reasoning, for
   the person to weigh; they are not a grade. The decision is the person's.
10. **Conversation in the user's language; everything written to disk in English.**
    The agents answer in English and you put the report in their language.
11. **A check that did not run never reads as clean.** An agent that failed, a log
    that was not read, a CI that was not read, a missing `evidence` and a thing
    outside the repository that was not checked each appear with their reason and
    with no count.

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

An input that ends in `tema <names>` names a topic, which **Topics** below defines,
or several separated by commas, as in `tema aum,valuation`, which are reviewed
together as one. Take `tema` and everything after it off, keep `<names>` exactly
as written, commas and all, and read what is left.
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

On a branch, a range, or a PR whose commits are local, the topics below come
first, and the readback is said once they are known: with a topic named and two
or more topics, it is the topic's readback there instead of this one.

#### Topics

On a branch, a range, or a PR whose commits are local, as **A PR** below says,
the commits classify the change into topics, and that is all they do: what gets
reviewed is always the final diff `<base>..<head>`, cut down to the files of the
topics named, never the state of the code at any commit in between.

A topic is the scope of a conventional commit, `type(scope): ...` or
`type(scope)!: ...`, read loosely: a space may stand before the parenthesis and
the type may take any case, so `Feat (split): ...` is the topic `split`. The scope
is lowercased, so `CI` and `ci` are one topic, and scopes that differ by a plural
or a spelling stay two. When some non-merge commit of the change has a scope,
every non-merge commit without one is its own topic, named by its short hash.
When none has a scope, each commit goes under the directory where its own diff
changed the most lines, added plus deleted: the first two segments of a path with
three or more, the first segment of a path with two, and `raiz` for a file at the
root, the alphabetically first winning a tie.

A topic's files are the files of the final diff that its commits touched, and a
file that several topics touched belongs to each of them and is shared. A file
that no non-merge commit touched, which only a merge brought in, belongs to the
topic `sin-tema`, whose commits are the change's merge commits. `sin-tema` is the
last row, is there only when it has files, shares none of them, and does not count
toward the two topics a map needs. A renamed file counts as a deletion and an
addition, so the total here can run one above the readback's count for each
rename.

The map reads git's output from the temporary directory step 3 fills, so that
directory is made here, before the map, and step 3 uses it:

```bash
mktemp -d
```

`<work-dir>` is the path it printed, pasted literally from here on. If that path
sits under `<target-repo>`, stop: the temporary directory has to be outside the
reviewed repository. It is the only place this skill writes, apart from the
`.ai-kit/` that step 6's log read leaves in the reviewed repository under rule 2,
and every run that made it removes it at step 10, a run that stops before step 3
included.

The map is computed here on every run, from git and nothing else. Git writes what
the map reads to three files in `<work-dir>`:

```bash
git -C <target-repo> -c core.quotePath=false diff --no-ext-diff --no-textconv --no-renames --name-status "<base>" "<head>" > "<work-dir>/map-status.txt" &&
git -C <target-repo> -c core.quotePath=false log --no-merges --no-renames --numstat --format=%x01%h%x09%s "<base>..<head>" > "<work-dir>/map-log.txt" &&
git -C <target-repo> log --merges --format=%h "<base>..<head>" > "<work-dir>/map-merges.txt"
```

**When this block fails, go to step 10's cleanup and stop**, repeating the error,
with no agent launched.

The program below reads those three files and runs no command, and the map it
prints is never written anywhere. Pass it inline, as it stands; if it has to go in
a file, that file is `<work-dir>/map.awk` and nowhere else. With no topic named it
runs with `show="rows"` and an empty `topic`; with one named, with `show="pick"`
and `topic` set to `<names>` as written. `label`, `base`, `from` and `head` are
what the readback names and the program cannot work out: the ref, range or PR as
the person wrote it, the short base, where the base came from, and the short head.

```bash
awk -v dir="<work-dir>" -v show="<rows, or pick when a topic was named>" -v topic="<names, or nothing>" -v label="<ref, rango o PR>" -v base="<base corta>" -v from="<de dónde salió>" -v head="<head corta>" 'BEGIN {
  sfile = dir "/map-status.txt"
  while ((getline line < sfile) > 0) {
    tab = index(line, "\t")
    path = substr(line, tab + 1)
    status[path] = substr(line, 1, 1)
    order[++n] = path
  }
  close(sfile)
  lfile = dir "/map-log.txt"
  while ((getline line < lfile) > 0) {
    if (substr(line, 1, 1) == "\001") {
      tab = index(line, "\t")
      hash[++nc] = substr(line, 2, tab - 2)
      subject = substr(line, tab + 1)
      if (match(subject, /^[A-Za-z]+ ?\([^() ]+\)!?:/)) {
        s = substr(subject, index(subject, "(") + 1)
        scope[nc] = tolower(substr(s, 1, index(s, ")") - 1))
        scoped = 1
      }
      continue
    }
    if (line == "") continue
    split(line, f, "\t")
    path = substr(line, length(f[1]) + length(f[2]) + 3)
    k = split(path, seg, "/")
    key = (k >= 3) ? seg[1] "/" seg[2] : (k == 2) ? seg[1] : "raiz"
    if (!((nc, key) in weight)) keys[nc, ++nkeys[nc]] = key
    weight[nc, key] += (f[1] == "-" ? 0 : f[1]) + (f[2] == "-" ? 0 : f[2])
    if (path in status) touched[nc, ++ntouched[nc]] = path
  }
  close(lfile)
  for (c = 1; c <= nc; c++) {
    t = hash[c]
    if (c in scope) t = scope[c]
    else if (!scoped) {
      best = -1
      for (i = 1; i <= nkeys[c]; i++) {
        key = keys[c, i]
        if (weight[c, key] > best || (weight[c, key] == best && key < t)) { t = key; best = weight[c, key] }
      }
    }
    if (!(t in commits)) names[++m] = t
    sep = (t in commits) ? " " : ""
    commits[t] = commits[t] sep hash[c]
    ncommits[t]++
    for (i = 1; i <= ntouched[c]; i++) {
      line = touched[c, i]
      if ((t, line) in has) continue
      has[t, line] = 1
      files[t]++
      if (status[line] == "A") added[t]++
      else if (status[line] == "D") deleted[t]++
      else modified[t]++
      sep = (line in owners) ? " " : ""
      owners[line] = owners[line] sep t
      nowners[line]++
    }
  }
  for (i = 1; i <= n; i++) {
    p = order[i]
    if (nowners[p] < 2) continue
    k = split(owners[p], o, " ")
    for (j = 1; j <= k; j++) shared[o[j]]++
  }
  for (i = 2; i <= m; i++) {
    x = names[i]
    for (j = i - 1; j >= 1 && (files[names[j]] + 0 < files[x] + 0 || (files[names[j]] + 0 == files[x] + 0 && names[j] > x)); j--) names[j + 1] = names[j]
    names[j + 1] = x
  }
  rows = m
  for (i = 1; i <= n; i++) {
    p = order[i]
    if (p in owners) continue
    owners[p] = "sin-tema"
    has["sin-tema", p] = 1
    files["sin-tema"]++
    if (status[p] == "A") added["sin-tema"]++
    else if (status[p] == "D") deleted["sin-tema"]++
    else modified["sin-tema"]++
  }
  if (files["sin-tema"] > 0) {
    names[++rows] = "sin-tema"
    mfile = dir "/map-merges.txt"
    while ((getline line < mfile) > 0) {
      sep = ("sin-tema" in ncommits) ? " " : ""
      commits["sin-tema"] = commits["sin-tema"] sep line
      ncommits["sin-tema"]++
    }
    close(mfile)
  }
  for (i = 1; i <= rows; i++) if (files[names[i]] > 0) {
    valid[names[i]] = 1
    list = list (list == "" ? "" : ", ") "`" names[i] "`"
  }
  if (topic in valid) picked[++np] = topic
  else {
    k = split(topic, w, ",")
    for (i = 1; i <= k; i++) {
      x = w[i]
      gsub(/^ +| +$/, "", x)
      if (x != "" && !(x in named)) picked[++np] = x
      named[x] = 1
    }
  }
  delete named
  ok = (np > 0)
  for (i = 1; i <= np; i++) {
    named[picked[i]] = 1
    if (!(picked[i] in valid)) ok = 0
  }
  if (show != "export" && m < 2) print "fewer than two topics: the change is reviewed whole"
  if (show == "rows" && m >= 2) {
    bar = "\174"
    hdr = "@ Tema @ Commits @ Archivos @ Nuevos @ Modificados @ Borrados @ Compartidos @"
    rule = "@---@---@---@---@---@---@---@"
    fmt = "@ `%s` @ %d: %s @ %d @ %d @ %d @ %d @ %d @\n"
    gsub(/@/, bar, hdr)
    gsub(/@/, bar, rule)
    gsub(/@/, bar, fmt)
    print hdr
    print rule
    for (i = 1; i <= rows; i++) {
      t = names[i]
      c = commits[t]
      gsub(/[^ ]+/, "`&`", c)
      printf fmt, t, ncommits[t], c, files[t], added[t], modified[t], deleted[t], shared[t]
    }
  }
  if (show == "pick" && m >= 2 && !ok) print "no such topic; valid names: " list
  if (show == "pick" && m >= 2 && ok) {
    for (i = 1; i <= np; i++) {
      tl = tl (i > 1 ? ", " : "") "`" picked[i] "`"
      hl = hl (hl == "" || commits[picked[i]] == "" ? "" : " ") commits[picked[i]]
    }
    for (i = 1; i <= n; i++) {
      p = order[i]
      k = split(owners[p], o, " ")
      in_union = 0
      outside = 0
      for (j = 1; j <= k; j++) {
        if (o[j] in named) in_union = 1
        else outside = 1
      }
      if (in_union) { nfiles++; if (outside) nshared++ }
    }
    print "topics: " tl
    print "files: " nfiles + 0
    print "shared: " nshared + 0
    print "commits: " hl
    print "Reviso `" label "`, " (np > 1 ? "temas" : "tema") " " tl ": base `" base "` (`" from "`), head `" head "`, `" nfiles + 0 "` archivos del tema, `" nshared + 0 "` compartidos con otros temas."
  }
  if (show == "export") {
    printf "" > (dir "/topics.txt")
    for (i = 1; i <= n; i++) {
      p = order[i]
      k = split(owners[p], o, " ")
      others = ""
      in_union = 0
      for (j = 1; j <= k; j++) {
        if (o[j] in named) in_union = 1
        else others = others (others == "" ? "" : " ") o[j]
      }
      if (in_union) print p > (dir "/changed.txt")
      if (others != "") print p "\t" others > (dir "/topics.txt")
    }
  }
}'
```

With fewer than two topics it prints one line, `fewer than two topics: the change
is reviewed whole`, in either shape. With two or more and `show="rows"` it prints
the map itself, as the markdown table the person sees: a header, a separator and
one row per topic, largest first and `sin-tema` last, each with the topic, its
commits counted and listed by short hash, its files, how many of them were added,
modified and deleted, and how many are shared. With two or more and `show="pick"`
it prints what the topics named add up to and the topic's readback, or the line
that stops the run. **Every cell and every value is the program's.** The table goes to the person as it was
printed, pasted whole and verbatim: no row is retyped, reordered, recomputed or
rounded, no cell is filled from anything but that output, and no table is ever
written by hand from the git output. Step 3 runs the same program with
`show="export"`, over the same three files, and that is the only other place it
runs.

- **The one line** means a single topic, or none. The change is reviewed exactly
  as it is today: the readback above, then step 3, and a `tema` named with it
  changes nothing. On a PR that is the PR reviewed whole, as **A PR** says.
- **A table and no topic named**: say the readback above, then the map, then ask
  which topic to review, and stop, after step 10's cleanup has removed
  `<work-dir>`. Nothing has been exported and no agent has been launched. First
  this line:

  > Temas por scope de commit, o por directorio cuando ningún commit trae scope;
  > `sin-tema` junta los archivos que sólo trajo un merge. Cada uno se revisa
  > sobre el diff final `<base corta>..<head corta>` reducido a sus archivos; un
  > archivo compartido entra en cada tema que lo tocó.

  then the table, every line the program printed, as it printed it, and then the
  question:

  > ¿Qué tema reviso? Pedímelo como `review <ref, rango o PR> tema <nombre>`, o
  > como `tema <nombre>,<nombre>` para revisar varios juntos en un solo reporte.

  An answer that names one or several of them is that request: start again from
  step 2 with `tema <names>` added to the input, so the map is computed again.
- **A topic named** runs the program with `show="pick"`, and what it prints
  decides the path.

  Several names separated by commas are one topic: the union of their files and
  of their commits, reviewed in one report. A file that only topics named
  together touched is not shared on that report, since every topic that touched
  it is under review. `sin-tema` can be named alone or with others.

  **A line starting `no such topic; valid names:` stops the run** after step
  10's cleanup, and no agent is launched. It is what the program prints when a name is not on the map or its
  row has no files, and what follows the colon is every name with files, as the
  `Tema` column shows them. Paste that list, as printed, into the message:

  > No hay archivos del tema `<names>` en `<ref, rango o PR>`. Los temas que
  > puedo revisar son: <la lista que imprimió el programa, tal cual>. No lancé
  > ningún agente.

  **Otherwise it prints five lines**: `topics:` with the names as the readback and
  the report write them, `files:` with the files of the union, `shared:` with how
  many of them a topic outside the named ones also touched, `commits:` with the
  short hashes of the union's commits, which step 4 pastes, and last the topic's
  readback, the line starting `Reviso`. It is this one, with `tema` for one name
  and `temas` for several, `<topics>`, `<files>` and `<shared>` the values of the
  lines above it, and the rest what `label`, `base`, `from` and `head` carried:

  > Reviso `<ref, rango o PR>`, tema <topics>: base `<base corta>`
  > (`<de dónde salió>`), head `<head corta>`, `<files>` archivos del tema,
  > `<shared>` compartidos con otros temas.

  **That line is said to the person now, verbatim, as the program printed it, as
  a message of its own, and step 3 does not start until it has been said.**
  Nothing is exported and no agent is launched before it. It is never retyped
  from the template above or rebuilt from the other four lines; the template is
  what the program prints, not something to fill in.

Everything below that says "on a topic" holds only on this last path, for one
name or several, and `<names>` there is what followed `tema`, as written, which
the program reads, never retyped. On a PR whose commits are not local this
section does not run, and the PR is reviewed whole.

#### A PR

Resolve the GitHub repository from the target, unless the URL already names it:

```bash
git -C <target-repo> remote get-url origin
```

Read `<owner>/<name>` off what it prints. No `origin`, or a remote that is not on
GitHub, stops the run and asks for the PR's URL. `--repo` goes on every `gh` call,
so nothing depends on where this shell is standing.

```bash
gh pr view "<number>" --repo "<owner>/<name>" --json number,title,body,baseRefName,baseRefOid,headRefName,headRefOid,changedFiles
```

**A failure stops the run**, and no agent is launched: a PR that does not exist, a
`gh` with no access, a missing `gh`. Name the PR and repeat what `gh` said.

> No pude leer el PR `<number>` de `<owner>/<name>`: `<lo que dijo gh>`. No lancé
> ningún agente.

`changedFiles` at 0 stops the same way, with the no-changes message. Keep `title`
and `body`: they are what the change declares, and step 4 hands them on.

**A PR whose two commits are local goes through Topics.** Ask for each, one call
per commit so that a failure says which one is missing:

```bash
git -C <target-repo> cat-file -e "<baseRefOid>^{commit}"
git -C <target-repo> cat-file -e "<headRefOid>^{commit}"
```

Both exit 0 when both are local. Then the head is `headRefOid` and the base is
what this prints, the commit GitHub's own diff of the PR starts from:

```bash
git -C <target-repo> merge-base "<baseRefOid>" "<headRefOid>"
```

Run the diff check from step 2 with that base and head, and go to **Topics**.
On a map or a topic it goes the way a range does, and the PR number stays what the
readback and the report name. When Topics prints its one line, the PR is reviewed
whole: the readback Topics said, with this base, is the PR's only one and is not
said again, a `tema` named with it changes nothing, and step 3 reads it from
GitHub, whose diff starts from this same commit.

**Otherwise the PR is reviewed whole**, whether a commit is missing or
`merge-base` printed nothing: the base is `baseRefOid` and the head is
`headRefOid`, a `tema` named with it changes nothing, and step 3 reads it from
GitHub. Say the same readback line, then go on to step 3.

### 3 — Put the change in a temporary directory

`<work-dir>` is the temporary directory **Topics** made before the map, and it
holds nothing yet but what the map read. A PR that did not go through Topics
reaches here without one, and makes it now:

```bash
mktemp -d
```

`<work-dir>` is the path it printed, pasted literally from here on, and the rest
holds as Topics says: if that path sits under `<target-repo>`, stop, and it is the
only place this skill writes.

For a branch or a range, except on a topic:

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

**On a topic**, nothing here runs until the topic's readback, the line starting
`Reviso` that step 2's program printed, has been said to the person as printed.
Then, a PR's included, the code is the whole head, exported the same
way from the local commit, so that the readers can follow what the topic's files
call and what calls them. What is cut down is the diff and the list of changed
files. The map program writes the topic's files to `changed.txt`, the union's
when several topics were named, and to `topics.txt` every file of the final diff
that a topic outside the named ones touched, followed by a tab and those topics,
which step 8 and step 11 read; it reads the three files step 2 wrote, which are
still there. The diff is the final one with every file outside
the topic taken out:

```bash
mkdir "<work-dir>/code" &&
git -c filter.lfs.smudge= -c filter.lfs.process= -c filter.lfs.required=false -C <target-repo> archive --format=tar --output "<work-dir>/head.tar" "<head>" &&
tar -xf "<work-dir>/head.tar" -C "<work-dir>/code" &&
rm "<work-dir>/head.tar" &&
awk -v dir="<work-dir>" -v show="export" -v topic="<names>" '<the program of the map block in step 2, verbatim>' &&
git -c core.quotePath=false -C <target-repo> diff --no-color --no-ext-diff --no-textconv --no-renames "<base>" "<head>" > "<work-dir>/whole.diff" &&
awk 'NR == FNR { keep[$0] = 1; next } /^diff --git / { on = 0 } /^diff --git a\// { rest = substr($0, 14); on = (substr(rest, 1, (length(rest) - 3) / 2) in keep) } on' "<work-dir>/changed.txt" "<work-dir>/whole.diff" > "<work-dir>/change.diff" &&
rm "<work-dir>/whole.diff" &&
grep -c '^diff --git ' "<work-dir>/change.diff" &&
wc -l "<work-dir>/changed.txt"
```

The two counts at the end are the same number, the `files:` value step 2's
program printed for the topic. A file whose path git has to quote, because it holds a tab, a quote, a
backslash or a newline, falls out of the diff and makes the first count smaller;
it stays in `changed.txt`, so it is still listed as unread if nobody opens it.

For a PR reviewed whole, the diff comes from GitHub, and the code comes over
`gh api` as a tarball of the exact head commit, because a `git fetch` would write
to `.git`:

```bash
mkdir "<work-dir>/code" &&
gh pr diff "<number>" --repo "<owner>/<name>" > "<work-dir>/change.diff" &&
gh pr diff "<number>" --repo "<owner>/<name>" --name-only > "<work-dir>/changed.txt" &&
gh api "repos/<owner>/<name>/tarball/<headRefOid>" > "<work-dir>/head.tar.gz" &&
tar -xzf "<work-dir>/head.tar.gz" --strip-components=1 -C "<work-dir>/code" &&
rm "<work-dir>/head.tar.gz"
```

**When any of these blocks fails, go to step 10's cleanup and stop**, repeating
the error, with no agent launched. GitHub refuses the diff of a very large PR; when
that is what happened, say so and offer the range, if the branch is local.

### 4 — Read what the change says it is for

`evaluate` judges what is missing against an intent, and here the intent is what
whoever made the change wrote about it. Nobody in this conversation corrected it,
which is why it travels as `declared`.

**On a PR**, except on a topic, it is the `title` and the `body` step 2 read,
verbatim. An empty body leaves the title alone.

**On a branch or a range** it is the commit messages of the change:

```bash
git -C <target-repo> log --no-merges --format='%h %s%n%b' "<base>..<head>"
```

**On a topic**, a PR's included, it is the messages of the topic's own commits
and of no other: the short hashes on the `commits:` line step 2's program printed,
pasted one per argument. For several topics that line holds the commits of all of
them, and for `sin-tema` it holds the change's merge commits, whose messages are
then the whole of what that topic declares:

```bash
git -C <target-repo> log --no-walk --format='%h %s%n%b' "<hash>" "<hash>"
```

When that runs past about two hundred lines, run it once more with
`--format='%h %s'` and hand on the subjects alone, saying in the intent that they
are subjects only. Merge commits are left out of every intent but `sin-tema`'s,
because their message names a branch, not a purpose; `sin-tema` has nothing else
to be judged against.

**Nothing declared** — an empty output, or a PR with neither title nor body worth
the name — is not a stop. `evaluate` gets `intent_source: inferred` and the intent
`Nothing was declared; read it off the diff at diff_path.`, and the report says the
intent was inferred.

### 5 — Launch `explore`, `rules`, `evaluate` and `outside`

Four subagents, launched together in one turn so that they read in parallel, or
three where `outside`'s role is not installed, as its paragraph below says. Each
gets the inputs below and nothing else, and every path in them is inside
`<work-dir>`.

**Every input keeps its shape however long it is**, on a topic or not:

- **The intent goes verbatim** to `evaluate` and `outside`, the two readers whose
  contracts take one: every line step 4 handed on, in its order, never summarized,
  shortened, paraphrased or reworded, at any length. Step 4 is the only place an
  intent is ever cut, and what it handed on is what goes.
- **The changed files go in the shape each contract names.** `rules` and `outside`
  take `changed_files_path`, and `explore` takes `paths_hint`: each gets the path
  `<work-dir>/changed.txt`, never the paths it holds. `evaluate` takes
  `changed_files`, which its contract defines as the paths themselves: every line
  of `changed.txt`, one per line, as written there, at any length, and never the
  path to the file, a sample or a count. Read it whole for that, past any limit a
  tool puts on it:

  ```bash
  cat "<work-dir>/changed.txt"
  ```

- **No reader gets an input its contract does not define.** No note, caveat,
  summary or instruction rides beside the fields listed for each reader below.
  `explore`'s topic is the text below, with the lines for a topic when there is
  one, and nothing more.

**`explore`** explains the change:

```
topic: <the topic below, with its placeholders substituted>
repo_root: <work-dir>/code
scope: both
paths_hint: <work-dir>/change.diff, <work-dir>/changed.txt
budget_hint: <the number of lines in changed.txt, plus ten>
```

The topic, in English:

```
Explain the change under review, <the ref, the range or the PR, as the readback named it>.
Its diff is <work-dir>/change.diff and its changed files are listed in
<work-dir>/changed.txt. repo_root is an export of its head: no .git, no history,
none of the files the repository ignores. Read the diff first, then the code it
touches and what calls it. Return findings for six questions, each claim starting
with its tag:
- what: what the change is, at most five findings.
- how: how it works, at most ten steps in the order the code runs them.
- business: the business rules it implements, at most eight.
- impact: what inside this repository calls, reads or depends on what it changed,
  at most eight.
- test: each test the change added or touched, what it exercises and what it
  stands on: mocks, fake servers, an in-memory store, a real database or service,
  or nothing.
- untested: each changed path that no test exercises, with a line in the changed
  file as its evidence.
Cite every path relative to repo_root. Do not use the code graph: any index of
this repository describes a checkout, not this export. Run nothing from this code.
List in files_read every path under repo_root you opened, relative to repo_root.
```

On a topic, that text gets these lines after its first sentence, so that the files
the diff leaves out are not read as missing from the change. `<topics>` is the
`topics:` value step 2's program printed, after `topic` for one name and `topics`
for several:

```
This is the topic <topics> of a larger change: the diff and changed files hold only
the files its own commits touched, and the head under repo_root is the whole
change.
```

**`rules`** finds what the change assumes:

```
label: <the ref, the range or the PR, as the readback named it>
code_dir: <work-dir>/code
diff_path: <work-dir>/change.diff
changed_files_path: <work-dir>/changed.txt
```

**`evaluate`** judges what is missing and what could be better:

```
intent: <step 4's intent, verbatim>
intent_source: <declared, or inferred when step 4 found nothing declared>
manual_checks: <empty>
repo_root: <work-dir>/code
changed_files: <the paths in <work-dir>/changed.txt, as written there>
diff_path: <work-dir>/change.diff
constraints: <empty>
```

**`outside`** checks what the change touches outside the repository:

```
label: <the ref, the range or the PR, as the readback named it>
repo_name: <on a PR, the <name> of <owner>/<name>; otherwise the <name> read off origin below>
intent: <step 4's intent, verbatim>
code_dir: <work-dir>/code
diff_path: <work-dir>/change.diff
changed_files_path: <work-dir>/changed.txt
```

On a branch or a range, `repo_name` comes from the remote and not from the
directory, because a linked worktree or a clone under another name sits in a
folder that is not the repository's name, and `outside` tags its reads and
searches with it:

```bash
git -C <target-repo> remote get-url origin
```

Read `<owner>/<name>` off what it prints, as step 2 does for a PR, and pass
`<name>`. A remote that is not on GitHub is not a stop here: its last path
component, without `.git`, is the name. Only when there is no `origin` is
`repo_name` the last component of `<target-repo>`.

It gets no list of tools: what the host exposes is discovered by the agent at run
time, and a tool added to the host is used without this skill changing. A host
with no outside tools is not a stop: `outside` says so and its section reads as
one line.

**`outside` is launched only as its own installed role**, `ai-kit:outside` on
Claude Code, which carries the `disallowedTools` that keep the shell and every
editing tool away from it. It is not installed on Codex: Codex ignores
`disallowedTools` and its read-only sandbox does not cover MCP tool calls, so
nothing but the contract would stand between `outside` and a tool that writes.
On Codex, and on any host where the role cannot be selected by name, it is not
launched, not even through the runtime's fallback of handing the contract to a
fresh subagent, and its section is one line:

> No corrió outside: en este host no está instalado como rol propio, y sin él
> nada más que el contrato lo separa de una herramienta que escribe (en Codex,
> `disallowedTools` se ignora y el sandbox de solo lectura no cubre las
> llamadas a herramientas MCP).

**Wait for all four before step 8.** Steps 6 and 7 touch nothing in `<work-dir>`
and may run while they read, when the host lets agents run in the background.
Nothing from step 8 on runs while any of them is still reading, on any path: step
9 puts the base beside the code they are reading, and step 10 deletes the
directory from under them.

**An agent that fails, or returns nothing usable, costs its own sections and
nothing else.** Keep what went wrong in one line; step 11 prints it where those
sections would have been. `outside` returning `did_not_run` is the same case, with
its reason, and so is `outside` not launched on a host without its role. When none
of the four returns something usable there is no report: go to step 10's cleanup
and say which failed or did not run and why.

> Ninguno de los cuatro lectores devolvió algo usable: `<explore: razón>`,
> `<rules: razón>`, `<evaluate: razón>`, `<outside: razón>`. No hay reporte.

### 6 — Read the why from the log

The code says what the change does. Why the code around it is the way it is, what
was decided and what went wrong once, is a question the code cannot be asked, and
it is read from the reviewed repository's log the way `ask` reads rationale.
Rule 2 is the boundary: reads only, against `<target-repo>`.

```bash
command -v memory
memory --help
```

When `memory` is missing or broken, this section does not run and the report
says so. Nothing is installed from here.

Choose the areas from where the change lands: the path prefixes, ending in `/`,
of the two or three directories that hold most of `changed.txt`. Choose the kinds
for the change, never all of them at once:

- **`adr`**, **`decision`** and **`gotcha`** first, on every change: why it is
  built this way, what a choice displaced, a constraint the code obeys without
  saying so.
- **`wont-fix`**, **`postmortem`** and **`dead-end`** when the change handles
  something that once failed, or does something somebody tried and dropped.
- **`hypothesis`**, **`open-question`** and **`assumption`** when the change
  touches what is knowingly unsettled.

```bash
memory query --dir <target-repo> --kind "<the kinds you chose>" --about "<area>" --current
memory search --dir <target-repo> "<a distinctive phrase from step 4's intent>"
```

**A read past the count threshold comes back as `returned: 0`, `truncated: true`,
at exit 0, and the response is to narrow**: one kind at a time, a tighter area,
or the phrase through `search`. When narrowing would lose the answer, read the
catalogue, which names every match and reads none of them, then read whole by id
the few that bear on the change:

```bash
memory query --dir <target-repo> --kind "<the kinds you chose>" --about "<area>" --current --threshold -1 --fields id,kind,title,at --body-lines 0
memory query --dir <target-repo> --id "<id>,<id>" --fields id,kind,title,at,body
```

Keep each record's **id and `at` date** from the moment you read it. What bears on
the change enters "what it is" and "how it works" as a why, cited `<id> (<date>)`.
**Never fill a why from the code**: the code's current shape narrated as if
somebody had chosen it reads exactly like a recorded decision, and it is not one.
When nothing came back, the why is absent and the report says which kinds it
consulted over which areas. A `count: 0` cannot tell an empty log from a log with
nothing on this surface, so it is reported as nothing found, never as no log.

**A failure here stops nothing.** The section says the log was not read and why.

### 7 — Read the CI

Rule 11 holds here: CI that was not read is never reported as passing.

**On a PR**, with the number and repository step 2 used:

```bash
gh pr checks "<number>" --repo "<owner>/<name>" --json bucket,completedAt,description,event,link,name,startedAt,state,workflow
```

Exit 0 means every check finished. **Exit 8 means some are still pending**, and
what it printed is still the list. A PR with no checks may come back as an empty
list or as a non-zero exit saying none were reported; either reads as no checks
reported, quoting what `gh` said in the second case. Any other failure reads as CI
not read, with what `gh` said.

**On a branch**, the checks are those of its open PR, when it has one. `<ref>` is
a branch when this prints a line:

```bash
git -C <target-repo> for-each-ref --format='%(refname)' "refs/heads/<ref>" "refs/remotes/origin/<ref>"
```

Nothing printed means a tag or a commit, and CI is not read. Otherwise resolve
`<owner>/<name>` as step 2 does for a PR, with `git remote get-url origin`, and
look for the PR by the branch's name, without `origin/`:

```bash
gh pr list --repo "<owner>/<name>" --head "<branch>" --state open --json number,headRefOid,url
```

An empty list means the branch has no open PR, and the section says so. One PR
means `gh pr checks` on its number, as above. When its `headRefOid` is not the
reviewed head, the checks are of another commit and the section names it. More
than one: read the one whose `headRefOid` is the reviewed head, and when none is,
name them and read none. No `origin`, a remote that is not on GitHub, or a `gh`
that fails is CI not read, with the reason.

**On a range** CI is not read: a range has no PR to read it from, and the section
says so.

### 8 — Check what came back

An agent's own account is not the report. Three things in what the four returned
are checkable, and all three are checked here against the files.

**Every anchor has to exist at the reviewed ref.** That is each `anchor` in
`rules`' `rules` and `what_can_go_wrong`, each `evidence` entry of `explore`'s
findings, each `where` of `evaluate`'s `missed` and `better`, and each `where` of
`outside`'s `touches`. Paste one line per anchor into one block, with `<path>` and
`<line>` taken from it; for a range `<path>:<first>-<last>`, `<line>` is its first
line. Add the `grep` line for the anchors of `rules`' rules and for the evidence of
`explore`'s `untested:` findings, and for nothing else:

```bash
awk 'NR==<line>{print FILENAME":"FNR": "$0}' "<work-dir>/code/<path>"
grep -Fxc "<path>" "<work-dir>/changed.txt"
```

- `awk` printing nothing, or a line with nothing on it, means the anchor does not
  exist. A rule, a what-can-go-wrong entry, an `evaluate` finding or an `outside`
  thing with that anchor is dropped, the `outside` thing with every check on it.
  An `explore` finding keeps the evidence that exists and is dropped when none
  does. An evidence entry that is not a path under `code` is dropped the same way.
  The report counts the drops per agent.
- `grep` printing `0` means the file was not touched: the rule is inherited. Any
  other number means it was. Take this over the agent's `inherited`.
- **On a topic**, a file the topic did not touch may still be one another topic
  changed. For each rule whose `grep` printed `0`, look its path up in the map:

  ```bash
  awk -F '\t' '$1 == "<path>" { print $2 }' "<work-dir>/topics.txt"
  ```

  What it prints is the topics that changed that file, `sin-tema` when only a
  merge did, and the rule is labelled with them rather than inherited. Every file
  of the final diff outside the topics named is there, so nothing printed means
  the change did not touch it, and the rule is inherited.
- **An `untested:` finding is about a changed path or it is dropped.** Its evidence
  entries whose `grep` prints `0` are taken off it, and the finding is dropped when
  none is left, counted with `explore`'s other drops. A path the change did not
  touch is not one it left untested. What survives here is the whole of what step
  11 may print as `Sin test`.
- **`outside`'s trace ids are not checked.** They name a call in a system this skill
  cannot read, so they are cited as `outside` returned them, never completed and
  never dropped for being unverifiable. What a tool returned is `outside`'s account
  and is printed as such.

Put the rules in order: `security` and `money` together, then `business`,
`experience`, `internal`. Keep the first ten. Put `what_can_go_wrong` in the same
order and keep the first five. Order `missed` and `better` by severity, `high`
first, and keep five of each.

**Unread is computed, not told.** Paste the `files_read` of every agent that
returned, `outside`'s included, together, as the arguments here, one per path,
spelled as `changed.txt` spells them: relative to `<work-dir>/code`, with that
prefix taken off a path an agent wrote absolute. A path that appears twice costs
nothing.

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

**On a topic**, read it with the other topics that share each file instead, which
prints every unread path, followed by a tab and those topics when it is shared.
`topics.txt` already leaves out the topics named, so a file only they touched
prints bare:

```bash
awk -F '\t' 'FILENAME == ARGV[1] { others[$1] = $2; next } { print $0 (($0 in others) ? "\t" others[$0] : "") }' "<work-dir>/topics.txt" "<work-dir>/unread.txt"
```

### 9 — Gather the evidence

**On a PR reviewed whole there is nothing to run**: no base is exported on that
path, so go to step 10, and the report says so.

On a branch, a range or a topic, a PR's topic included, since its commits are
local, the base goes beside the code here rather than in step 3,
so that it does not exist while the agents are reading, and that is why this step
waits for all four. `evidence` gets only paths inside the temporary directory,
never `<target-repo>`.

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
step 11 is built from it.

Each of `secrets`, `dependencies` and `workflows` carries a `status`, which is one
of `ran`, `partial`, `not-run` or `not-applicable`, with a `reason` on all but the
first. Its `new` lists what this change introduced, `inherited` counts what was
already at the base, and `scanners` names what ran. In `dependencies`, `targets`
says manifest by manifest what was scanned and what was not.

**On a topic** the same block runs, and `change.diff` is the topic's, so
`secrets` and `workflows` read only the topic's files. `dependencies` does not
read the diff: it compares every manifest at the base with the one at the head,
so on a topic it is about the whole change, and step 11 says so.

**A failure here stops nothing.** If the block fails, or what it prints is not
that JSON, go on to step 10 and step 11 says evidence did not run, repeating the
error.

### 10 — Clean up

```bash
rm -rf "<work-dir>"
```

This runs on every path that made `<work-dir>`, the stops at the map and at
`no such topic` included, and only once every agent launched in step 5 has
returned. It runs before the report goes out, so the report is the last
thing the person sees. If it fails, say so and name the directory.

### 11 — Print the report

In the user's language, every section in this order, and then stop. A section
whose agent did not run is still there, with one line saying so. The template is
the shape:

```
Revisión de `<ref, rango o PR>`, `<base corta>..<head corta>`. Leí <leídos> de <total> archivos cambiados.

## Qué es
<a lo sumo cinco líneas>
Por qué, según el log: <lo que dice el registro> <id> (<fecha>)

## Cómo funciona
1. <paso> `<path>:<line>`
2. <paso> `<path>:<line>`
Log: consulté <tipos> sobre <áreas>: <n> registros citados.

## Reglas de negocio que implementa
- <regla> `<path>:<line>`

## Impacto dentro del repositorio
- <qué lo usa y qué cambia para eso> `<path>:<line>`

## Fuera del repositorio
- <lo que toca> (<servicio | datos | historial>): <qué hace el cambio con eso> `<path>:<line>`
  <herramienta>: <lo que devolvió, en una línea> (traza <id>)
No se pudo revisar:
- <lo que toca>: <sin herramienta | falló la herramienta | requiere acceso | la consulta factura | la herramienta no es de solo lectura | fuera del corte de 20 llamadas>: <detalle>

## Qué se probó y cómo
- `<path>:<line>` <qué prueba>; se apoya en: <mocks, servidor falso, base real, nada>
Sin test: `<path>` <qué hace ese camino>
CI (PR #<n>): <n> checks: <n> pasaron, <n> fallaron, <n> pendientes, <n> omitidos, <n> cancelados
  <workflow> / <nombre>: <estado>

## Qué falta
Juzgado contra: <intent_read> (<declarada en el PR | declarada en los commits | inferida del diff>)
- [<severidad>] <qué no está> `<path>:<line>`
  <por qué importa>

## Qué podría estar mejor
- [<severidad>] <qué hace hoy el código> `<path>:<line>`
  <por qué otra forma es mejor>

## Reglas que asume
Criticidad inferida: la deduje del código, nadie la declaró para este repositorio.
1. [<nivel>] <la regla, en una oración> (heredada)
   `<path>:<line>` · leer: <dónde leer>
   <para qué sirve el código anclado y quién lo usa>
Quedaron <n> reglas fuera del corte de 10.

## Qué puede salir mal
- [<nivel>] <el caso que rompe una regla y qué hace entonces el código> `<path>:<line>`
  Lo nota: <quién y cómo>

Descarté <n> afirmaciones con un ancla que no existe en este ref o un camino sin test que el cambio no toca: <n> de explore, <n> de rules, <n> de evaluate, <n> de outside.

## Evidencia
Sobre lo que introduce este cambio:
Secretos: <n> nuevos
  `<path>:<line>` <regla>
Dependencias: <n> nuevas, <n> heredadas
  <advisory> en <paquete> <versión>, `<path>`
Workflows: <n> nuevos, <n> heredados
  `<path>:<line>` <regla> <detalle>
No corrió: <check o manifiesto>: <razón>
Escáneres: <nombre> <versión> (datos del <fecha>); Go <toolchain>, go.mod <versión>

## Sin leer (<n>)
<path>
<path>
```

- **The first line names what was reviewed once.** A range written `A..B` is
  its base and its head already, so it is named as written and nothing follows
  it: `Revisión de `e9c19a5..5ea9972`. Leí ...`. A ref, a PR and a range written
  `A...B` are followed by `<base corta>..<head corta>`, which is what they
  resolved to.
- **An agent that did not run** leaves each of its sections with one line:
  `No corrió explore: <razón>.` `explore` owns what it is, how it works, the
  business rules, the impact and the tests; `rules` owns the assumed rules and what
  can go wrong; `evaluate` owns what is missing and what could be better; `outside`
  owns what lies outside the repository. The CI line and the log's whys do not
  depend on any agent and stay.
- **The caps are the shape**: what it is in at most five lines, how it works in at
  most ten steps, business rules and impact at most eight each, missing and better
  at most five each, assumed rules at most ten, what can go wrong at most five.
  What a cap leaves out is counted in one line, `<n> más fuera del corte.`, never
  dropped silently.
- **How it works is steps with `path:line`.** Draw it as mermaid only when the
  answer is a graph, such as two flows that split and meet again, and never for a
  sequence a numbered list already says.
- **Fuera del repositorio is `outside`'s account, as it came back and survived
  step 8.** One entry per thing it touched: what it is, its kind, what the change
  does with it and its anchor, then one line per check with the tool's name as the
  host exposes it, what came back, and `(traza <id>)` when there was one. Then
  `No se pudo revisar:` with one line per thing in `not_checked`, each with its
  reason and its detail, the error and its trace when a tool failed; the list is
  absent when it is empty. When `outside` returned `did_not_run`, or was not
  launched because its role is not installed on this host, the section is one
  line, `No corrió outside: <razón>.`, as for any agent that did not run. When
  it returned `nothing_outside`, the section is one line, `El cambio no toca nada
  fuera del repositorio.` The orchestrator adds no thing, no tool and no result of
  its own, and calls no outside tool itself.
- `explore`'s `gaps` go at the end of the section they bear on, one line each:
  `Sin establecer: <la pregunta>`. A gap is not filled with a guess.
- **The why comes only from the log.** `Por qué, según el log` lines appear only
  for records step 6 read, each cited `<id> (<fecha>)`. The `Log:` line is always
  there: the kinds and areas consulted and how many records were cited, or
  `Log: consulté <tipos> sobre <áreas> y no hay nada.`, or
  `Log: no se leyó: <razón>.`
- **The tests section names each test the change added or touched** with what it
  stands on, then every changed path no test exercises. A change whose only tests
  stand on mocks says so in those words: it is the case where a test can pass on a
  false assumption, and the line appears only when every `test:` finding names
  mocks.
- **Every line of the tests section, the CI line apart, is an `explore` claim as
  it came back.** A test line is a `test:` finding that survived step 8, with what
  it stands on in the agent's words, never shortened into a kind it did not name.
  A `Sin test` line is an `untested:` finding that survived step 8, one per path it
  kept. The orchestrator never adds a `Sin test` line, a test or a `se apoya en`
  of its own, whatever it saw in the diff: a path that looks untested and no agent
  reported stays out of the report.
- **The CI line** is one of: the counts by `bucket` and one line per check that
  did not pass; `CI (PR #<n>, sobre <headRefOid corto>, que no es el head
  revisado)` when the branch's PR points at another commit; `CI: sin checks
  reportados.`; `CI: la rama no tiene un PR abierto.`; `CI: no se lee en un rango.`;
  `CI: no se leyó: <razón>.` Pending checks are counted as pending, never as
  passed.
- **What is missing opens with what it was judged against**, `intent_read` in the
  agent's own words and where the intent came from, because a finding against a
  misread intent looks exactly like a real one. Each finding keeps its reasoning in
  one line, shortened rather than dropped. `evaluate`'s `could_not_read` goes in one
  line at the end of the section when it is not empty. `holds_up` is not printed.
- The levels read `seguridad`, `dinero o posiciones`, `negocio o regulatorio`,
  `experiencia` and `interno`, or the same in the user's language. The severities
  read `alta`, `media` and `baja`.
- `(heredada)` goes on a rule whose anchor is in a file the change did not touch.
  When no rule has it, say so in one line instead: `Ninguna regla heredada: todas
  caen en archivos del cambio.` Never invent one.
- **Each assumed rule is three lines and no more**: the rule, its anchor and where
  to read, and its context. A line that does not fit gets shortened, not wrapped
  onto a fourth.
- The line about rules past the cut and the line about dropped anchors appear only
  when they are not zero.
- **The evidence block goes between what can go wrong and `Sin leer`** and holds
  facts from the JSON and nothing else. It says no `limpio`, no `seguro`, no
  `sin riesgo`: no word that grades the change.
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
- On a PR reviewed whole the block is one line: `Evidencia: no corrió: en un PR no
  se exporta la base.` When `evidence` itself failed it is one line too: `Evidencia: no corrió:
  <el error>.`
- `Sin leer` lists every path in `unread.txt`, one per line, and goes last so that
  a long list does not push a section off the screen. When nothing was left unread,
  the section is not there and the count in the first line says it.
- **An empty section is a real result.** No rules, nothing missing, nothing that
  can go wrong: say that none came up, in one line, and never pad.
- **On a topic the report is about that topic alone**, or about the union when
  several were named, and these lines change:
  - the first line names it after what was reviewed and counts its files:
    `Revisión de `<ref, rango o PR>`, `<base corta>..<head corta>`, tema <topics>.
    Leí <leídos> de <total> archivos del tema.`, with `<topics>` the `topics:`
    value step 2's program printed and `temas` for several, where a range written
    `A..B` is named once, as the first line's rule says: `Revisión de
    `e9c19a5..5ea9972`, tema `aum`. Leí ...`, or `Revisión de
    `e9c19a5..5ea9972`, temas `flight`, `restyclient`. Leí ...`
  - `Juzgado contra` ends `(declarada en los commits del tema <topics>)`, or
    `de los temas <topics>` for several.
  - a rule whose anchor is in a file another topic changed, as step 8 found it,
    reads `(del tema <tema>, <tema>)` where `(heredada)` would go. `(heredada)` is
    left for a file no topic changed.
  - in `Sin leer`, a path the awk printed with other topics after it reads
    `<path> (compartido con <tema>, <tema>)`; every other path is bare.
  - the CI line and the `Dependencias` line each end `(de todo el cambio)`, since
    neither is cut down to a topic.

Add nothing after it: no opinion on merging, no next step offered.

## Key principles

- **The decision is the person's, and the report is what it stands on.** Every
  section is something they need to decide and can check, and none of them grades
  the change.
- **Explaining and judging come from different readers.** `explore` says what the
  change is without judging it, `evaluate` judges it against what it declares, and
  `rules` says what has to hold for it to be right. One reader doing all three
  would explain the change in the terms of its own verdict.
- **An inherited rule is the reason `rules` exists**, and it cannot be seen from
  the changed lines, which is why `rules` gets the code as well as the diff.
- **A rule needs its context to be read.** What the anchored code is for and who
  uses it is what turns a rule into something the person can weigh.
- **The why comes from the log or not at all.** The code shows what is there; a
  reason read off it is a guess dressed as a decision.
- **What lies outside is read with what the host offers, and only what reads.**
  No list of tools lives here, so a tool added to the host is used without this
  skill changing. A tool is trusted to read by its own description and by nothing
  else, because a write misjudged as a read lands in production, where reverting
  ai-kit does not undo it.
- **Evidence is what this change introduced.** A report of the whole repository
  would read the same on every branch, so what is inherited is a count and nothing
  more.
- **What can be checked is checked here.** Every agent's anchors are read off the
  code and coverage is subtracted from the changed files, because a report that is
  trusted without the diff has nothing else standing behind it.
- **The reviewed repository is read and never touched**, and that includes the
  index, the refs and its code, which never runs. The log read's `.ai-kit/` is the
  one trace it leaves.
