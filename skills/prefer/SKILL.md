---
name: prefer
description: "Get a repository's stack and convention preferences out of somebody's head and onto the record: which library, which tool, which pattern, and why that one rather than the alternative it displaced. It reads the preferences already recorded first — per area, bounded to `--kind preference`, and out of the user's log as well as this repository's, since a preference carried up from somewhere else applies here too — and only then looks at what the repository actually does, delegating that look to the `explore` subagent so the manifest inventory never enters this thread and is discarded rather than written down. What it puts in front of the user is the divergence between what a preference says and what the repository does; a preference the repository already obeys never becomes a round. With preferences already on the record it makes two passes, one validating them and one fast sweep for the choices nobody has an opinion on yet; with none on the record it makes a single detailed pass and presents proposals, because there is nothing to diverge from. Nothing a manifest can be asked is ever written: `go.mod` already answers which version of GORM this is, and why GORM and not sqlc is what has nowhere else to live. Preferences are written with no `--plan`, because they belong to the repository rather than to any objective inside it, and one that replaces another arrives with the `supersedes` edge that leaves a single live answer per area. At close it offers to carry them up to user scope, where they outlive this repository. Triggers on: 'guardá mis preferencias', 'anotá que acá usamos X', 'estas son las herramientas que uso', 'qué stack usamos en este repo', 'por qué elegimos X y no Y', 'quiero que esto valga para todos mis repos', 'esta convención ya no vale', 'cambiamos de librería', 'fijá las convenciones de este proyecto', 'revisá si el repo sigue mis preferencias', 'record my stack preferences', 'write down our conventions', 'these are the tools I use', 'why did we pick this library', 'this preference is no longer true', 'we switched libraries', 'check whether this repo follows my preferences', 'carry these preferences to my other repos'. A human invokes it and it stops there: no step of `plan`, `work` or `debug` calls it, it authors no objective and invokes no other skill, and the only thing it writes anywhere is records through the `memory` CLI."
---

# prefer

Record what this repository chose, and why, in the one place a *why* can live.

A choice has two halves and only one of them needs writing down. **Which library
is at which version is a question the manifests answer at the moment of asking** —
`go.mod` says GORM v1.25, `package.json` says which package manager, the lockfile
says it better than either. Written into a record, that half is true on the day it
lands, unchecked for as long as the record lives, and indistinguishable from the
records around it that are still true. **Why GORM and not sqlc has nowhere else to
live.** Nothing regenerates it, nobody can derive it from the tree, and it is gone
the moment the person who decided it stops answering questions.

So this skill writes the second half and throws the first half away. The
substitution test decides every sentence, the same one `bootstrap` applies to a
README: before writing anything, ask which surface could have been asked for it,
and if one could, do not write it.

## The order is the whole design

**It reads the preferences already on the record first, and looks at the
repository second.** That order is not an implementation detail and it is not
reversible.

Reversed, the skill goes and finds out what the repository does, brings that back,
and asks the user to bless it. What comes out of that is not a preference. It is a
description of the status quo with somebody's name on it, and it is worse than
writing nothing, because from then on the repository's own habits are on the
record as though somebody had chosen them. The next reader cannot tell a decision
from an accident that got approved.

Read first and the exploration has something to check *against*, which is a
different and much cheaper question. "Does this repository follow the seven things
you already told me?" is answerable in a handful of files. "What does this
repository believe?" is not answerable at all.

## Two shapes, decided by what step 1 finds

**Preferences already on the record — two passes.** One validates the live ones
against what the repository actually does. One sweeps quickly for the choices
nobody has an opinion on yet. Both go to the same subagent with different prompts,
and only the disagreements reach the user.

**Nothing on the record — one pass, in detail, and no divergence.** There is
nothing to diverge *from*, so nothing is presented as a divergence. What the user
sees is a **proposal**: here is a choice this repository made where a real
alternative existed, is it one you would make again, and why. A proposal read as a
finding is exactly the failure the order above exists to prevent, so it is named
as a proposal when it is shown.

Nothing here runs on its own. No step of `plan`, `work` or `debug` calls this
skill; a human invokes it, and it records what they settle and stops.

## Hard rules

1. **The live preferences are read before anything explores the repository.**
   Step 1 runs to completion before step 2 starts. There is no fast path where
   the exploration goes first "because the log is probably empty" — that the log
   is empty is one of the two things step 1 is there to establish, and it decides
   which shape the whole run takes.
2. **Nothing is written that a manifest, a lockfile or the code can be asked.**
   No version, no package name as a fact, no directory listing, no "this repo uses
   X". The record carries the choice and the reasoning that made it worth making,
   and the reader gets the inventory from the tree, where it is always current.
3. **Every `preference` is written with no `--plan` at all**, and never under a
   sentinel plan slug invented for this skill. A preference is true of the
   repository rather than of one objective inside it, and a plan-bounded read
   returns it anyway — which is what the missing flag is for, not an oversight to
   be tidied up later. **Omitting the flag is not enough to enforce this**, and
   that is the one part of this rule that has to be done rather than intended: the
   CLI falls back to the `AI_KIT_PLAN` environment variable when the flag is
   absent, so an exported variable stamps the record silently and at exit 0. Every
   command in this skill runs under `env -u AI_KIT_PLAN` for that reason, and every
   write checks that `origin.plan` is absent from the record `add` prints back.
4. **Preferences are read with `--kind preference`, per area, never in bulk.**
   `query` withholds its records once a match count runs past a limit it reports
   in `memory kinds`, returning `count: N`, `returned: 0`, `truncated: true` at
   exit 0 — an answer-shaped non-answer. Reading one area at a time keeps every
   read far below that limit, and this skill never widens the setting.
5. **What goes in front of the user is divergence, not an enumeration.** A
   preference the repository already obeys is not a round, not a bullet and not a
   line of reassurance. It costs the user attention and returns nothing.
6. **A preference that replaces another is written with the `supersedes` edge in
   the same breath.** Without the edge both records answer `--current` and there
   is no single live answer for that area; the newest-wins convention is computed
   mechanically in exactly one place, the `live` field of `memory areas`, and
   nowhere else.
7. **A preference's first `--about` tag comes from an area that already exists in
   the log.** Inventing a new one is allowed only when the user says so, in words.
   Nothing in the CLI enforces either the position or the vocabulary; the existing
   areas *are* the vocabulary, and two writers keying the same question as
   `persistence` and `database` is how a log stops answering.
8. **Promotion to user scope is offered and never performed unasked.** The offer
   is the skill's; the act is the user's.
9. **Discovery is delegated to the `explore` subagent, and what it read is
   discarded.** The manifest inventory never enters this thread and never enters
   the log. This skill launches no other subagent and defines none.
10. **Read-only over the repository, with the log as the single exception.** No
    file is edited, no `docs/` is created, no scratch file is left behind. Never
    edit the `.jsonl` by hand.
11. **It records and stops.** No objective, no acceptance criteria, no fix, no
    invocation of `plan`, `work`, `debug`, `ask` or `bootstrap`. When the run turns
    up work worth doing, name it in one line and leave the next move to the user.
12. **Conversation in the user's language; everything written to disk in
    English.**

## The memory log

State lives in an append-only log, read and written only through the `memory`
CLI. Never edit the `.jsonl` by hand.

`memory kinds` lists every valid record kind and edge type and reports the count
at which `query` starts withholding; `memory --help` lists the flags each command
takes. A command this file does not show is one of those two reads away.

### Every command here clears `AI_KIT_PLAN`

That is why `env -u AI_KIT_PLAN` sits in front of every `memory` command in the
steps below, and it is not decoration. When `--plan` carries nothing the CLI reads
`AI_KIT_PLAN` from the environment — `memory --help` documents it as a supported
way to name a plan — so a user who exported it in a shell profile changes what
this skill writes *and* what it reads, silently and at exit 0. Both halves,
measured:

```
AI_KIT_PLAN=some-tree add --kind preference --about persistence
                                    →   origin.plan: "some-tree"   (hard rule 3 defeated)
AI_KIT_PLAN=some-tree areas --kind decision,adr
                                    →   count: 0, where the same call with the
                                        variable cleared enumerated the log in full
```

The read half has a safety net that the write half does not, and it is worth
knowing which: a plan-bounded read still returns records written with **no** plan,
so step 1's preference reads come back complete even under an exported variable —
but only for as long as every preference really is plan-less, which is the write
half's job. The two failures hold each other up, so clear it on both.

**The check is not the same field on both halves**, and assuming it is leaves the
write side unguarded — which is the side that cannot be taken back.

On a **read**, the response echoes the plan it resolved in a top-level `plan`:
`""` when nothing named one, and anything else means the environment named it for
you. **Of the five commands this file cites**, the split is:

```
areas, query    →  carry a top-level `plan`
add             →  ['path', 'record'] — no top-level plan
link, promote   →  no top-level plan
```

So on a **write** there is no top-level `plan` to read, and looking for one finds
nothing on a stamped record exactly as it finds nothing on a clean one. The field
that answers is **`origin.plan`, inside the record `add` prints back** — absent
when the write was plan-less, present and naming the tree when it was not. That is
the check hard rule 3 names and the one step 5 runs.

There is nothing to check on `link` or `promote`: an edge carries no `origin` at
all, and `promote` copies records with whatever `origin.plan` they already had, so
a record stamped at write time arrives in the user log still stamped however the
promotion was run. Which is the argument for catching it at `add` and nowhere else
— promotion cannot fix it, and by then it has been copied into a second
append-only log.

**The log is the one belonging to the repository this session is standing in**, so
no `--dir` appears on any command here. This skill is not pointed at somebody
else's checkout the way `ask` and `bootstrap` are: preferences about a repository
are gathered by somebody working in it, and taking a path as an input would mostly
buy a way to file one repository's choices in another's log.

The cost of leaving the flag off is worth naming rather than discovering. With no
`--dir`, every command resolves the log from whatever directory the session is
standing in, silently and at exit 0 — there is no refusal to catch a session that
started somewhere else. So print the repository once, at the top of step 1, and say
it back before anything is written:

```bash
git rev-parse --show-toplevel
```

Empty output means the session is not inside a git repository at all, and that is
a stop: say so and ask the user to run this from the repository whose preferences
they want recorded.

### What this skill writes, and under what

```
(no --plan)   --kind preference
(no --plan)   --kind open-question
```

**No `--plan` on either, and that is the whole point of them.** A choice about
which library this repository uses belongs to the repository and outlives every
objective tree in it; filed under a tree's slug it would vanish from view the
moment that tree was finished. There is no fourth sentinel slug here either — the
CLI stopped requiring a plan precisely so that a record could be repository-owned
rather than pretending to belong to a plan called `repo-something`.

What that buys is worth stating, because it is easy to assume the opposite: a
`--plan`-bounded read **does** return a record written with no plan. Measured:

```
memory query --plan some-tree --kind preference --current   →   count: 1
```

against a log whose only preference was written with no `--plan` at all. That line
is a measurement and not a read this skill performs — every read it performs is in
the steps below, bounded by `--kind preference` and taken one area at a time. What
it establishes is what somebody *else* sees: a later session working an objective
tree gets these records without knowing they exist, because what a plan-bounded
read filters out is records belonging to a *different* plan, not records belonging
to none.

`--kind preference` is the kind for a choice and its reasoning. It requires no
anchor — `memory kinds` reports `anchored_kinds` as `gotcha` alone — and that is
deliberate rather than a gap: a preference is about a choice, not about code that
exists at a line.

`--kind open-question` carries a probe that came back unanswered, following
`bootstrap`'s precedent. It is not a weaker `preference` and it is not a placeholder
for one: it is the record that says nobody in the room knew, so the next run does
not read the silence as "there is nothing here".

## Steps

### 1 — Read the live preferences, before looking at the repository

This is the step hard rule 1 protects. Nothing has explored anything yet, and that
is what makes what comes back trustworthy.

Print the repository first, by the section above, and name it to the user in the
same turn — it is the one value everything else lands in.

```bash
git rev-parse --show-toplevel
env -u AI_KIT_PLAN memory areas --kind preference
```

One call answers for both scopes. `areas` takes no `--scope` — it reports the
repository log and the user log separately inside one response — and `--kind`
bounds it twice over: it decides which areas appear *and* it decides which record
is `live` in each one, among records of that kind alone.

Read five things out of it, per scope:

- **`count`** — **the number of areas in that scope, and not the number of
  records.** It is exactly `len(areas)`. Measured both directions: one preference
  tagged `persistence,tools/db/` gives `count: 2`, and two preferences sharing the
  single tag `persistence` give `count: 1`. It is not live-only either — the
  enumeration counts through the settled records as well, so a tag whose every
  record has been retired still has a row and still adds one to `count`. Never
  report it to the user as a number of preferences.
- **`records`, on each row** — how many records carry that tag, settled ones
  included. **This is the per-record number**, and it is per area by construction.
  The other honest source for one is the `count` the per-area `query` below
  returns, which is the live records for that area alone.
- **`areas`** — one row per `--about` tag any preference carries. **`count: 0` with
  `areas: []` in both scopes is the no-preferences branch**, and correcting what
  `count` means does not disturb that: `add` refuses a record with no tag at all —
  `"at least one tag is required; without it the entry is unreachable by any
  filtered query"`, exit 1 — so under `--kind preference` no rows and no
  preferences are the same state, and any row at all means at least one preference
  exists. **That is the signal, and nothing about the file on disk is.** Every
  `memory` command creates the location it resolves, a read as much as a write, so
  `.ai-kit/memory.jsonl` exists from the moment of the first query whether or not
  anything was ever written into it, and its size is a proxy that lies the same
  way. On a repository the loop has worked in without ever running this skill, the
  empty branch is the ordinary answer rather than the exotic one.
- **`live`** — the record that currently answers for that area: its `id`, `title`
  and `at`. **`live: null` is not the same as an absent area**: it means every
  record carrying that tag has been settled by something that supersedes or answers
  it, which is a tag somebody had an opinion about and then retired. Measured, on a
  log whose only preference was settled by an `answers` edge: that area comes back
  `records: 1, live: null`, and the per-area `query --current` returns `count: 0`.
  Step 5 says what to do when you are about to write into an area in that state.
- **`key`** — `true` when that tag was the record's *first* `--about` entry. It is
  reported and never enforced, and step 5 is where it gets honoured.

Then read the live preference for each area it named, one area at a time:

```bash
env -u AI_KIT_PLAN memory query --kind preference --about <area> --current
env -u AI_KIT_PLAN memory query --scope user --kind preference --about <area> --current
```

`--current` drops what something explicitly supersedes or answers, which for
preferences is exactly the retired ones. The second line is not optional on an
area the user log also carries: a preference promoted up out of some other
repository applies here too, and a validation pass that never saw it cannot report
this repository as diverging from it — the silence then reads exactly like
agreement. `records` comes back `null` rather than `[]` when nothing matched, so
read `count`.

A tag ending in `/` matches by path prefix, so an area recorded as `tools/db/` is
also reached by `--about tools/`. Measured, on a log whose only live preference was
tagged `persistence,tools/db/`:

```
memory query --kind preference --about tools/ --current   →   count: 1
```

Close the step by saying in one line what came back, including nothing — a filter
that ran and found nothing is a different thing from a step that was skipped:

> En este repo hay `<n>` preferencias vivas en `<m>` áreas, y `<k>` más a nivel
> usuario. Voy a mirar el repo contra ellas.

or, on the empty branch:

> No hay ninguna preferencia guardada todavía, ni en este repo ni a nivel usuario.
> Entonces esta corrida es de propuestas: miro qué eligió el repo y te pregunto
> cuáles de esas elecciones son tuyas.

`<m>` is the one number `count` gives you. **`<n>` is not** — take it from the
per-area queries you just ran, counting the *distinct record ids* they returned,
because a preference carrying two tags comes back in two of those queries and is
still one preference. `<k>` comes the same way off the user-scope line. Reading
`<n>` off `count` would print the number of areas twice in one sentence, where it
reads as two independent figures agreeing.

### 2 — Send the repository to `explore`, in the shape step 1 decided

The reading is delegated, and delegation is what makes hard rule 2 free. Twenty
manifests read in this thread is twenty manifests of context spent, and every one
of them would then be sitting here tempting somebody to write down what it says.
Read in a subagent, the inventory is discarded by construction: what comes back is
`findings` with `path:line` evidence and `gaps`, and the files themselves never
arrive.

Use only the input block `agents/explore.md` declares — `topic` and `repo_root` are
required, `paths_hint`, `scope`, `budget_hint` and `prior_findings` are optional,
and there is no other field to invent.

**With preferences on the record — two passes.**

The first checks the live ones against the repository:

```
topic: Does this repository follow these recorded preferences, and specifically where does it not? <one line per live preference, its title and the area it is filed under>
repo_root: <absolute path>
scope: code
paths_hint: <the manifests and the directories the areas name — a path-prefix area like `tools/db/` is already the hint>
budget_hint: <about 8, the default, is right when there are a handful of preferences; raise it toward 15 for a dozen>
```

**Do not hand the live preferences in as `prior_findings`.** That field takes an
earlier run's findings, each with its `claim` *and its `evidence`*, and revalidates
them by opening the anchor. A `preference` carries no anchor — it is about a choice
rather than about code at a line — so every one of them would come back
`could_not_check`, which is a receipt for a check nobody could run. The preferences
belong in `topic`, as the question.

The second sweeps for what is missing, and it is deliberately cheap:

```
topic: Which choices does this repository make that nobody has recorded an opinion about — a library, a tool or a pattern picked where a real alternative existed? Name the choice and where it shows, not the version.
repo_root: <absolute path>
scope: both
budget_hint: 5
```

Small on purpose. This pass exists to notice a gap, not to inventory the
repository, and the areas step 1 already named are the ones the run is about.

**With nothing on the record — one pass, in detail.**

```
topic: What are the significant technical choices this repository has made — libraries, tools, layout and patterns — where a real alternative existed? For each one, name what was chosen and where it shows.
repo_root: <absolute path>
scope: both
budget_hint: 12
```

One pass and not two, because there is nothing to validate. Larger than the sweep
above, because this is the run that has to find the areas from scratch — but still
bounded: what it returns is a candidate list to ask about, not a document.

Whatever the shape, its `gaps` are kept as gaps. "I could not establish which HTTP
client this uses" is information, and rounding it into a confident sentence is how
this skill would start producing the thing it exists to replace.

### 3 — Reduce what came back to divergence

The subagent returns findings about the repository. The user gets none of that
directly. **What reaches them is the disagreement**, and this step is where the
findings become one.

**On the empty branch only the second half of this step applies.** There are no
live preferences, so there is nothing to sort into the three piles below and no
disagreement to reduce to; the single detailed pass stands where the sweep stands,
and everything it returned goes through the candidate test and the two drop rules.
Skip straight there. Running the first half against an empty set is not harmless
bookkeeping — it is what turns a proposal
into a finding, by sorting what the repository does as though something had been
checked against it, and this branch is where that mistake is easiest to make and
hardest to see afterwards.

Sort every live preference into exactly one of three:

- **Obeyed.** The repository does what the preference says. **This produces
  nothing**: no round, no bullet, no line of confirmation. It is the ordinary case
  and it is why the run is cheap.
- **Diverged.** The repository does something else, with a `path:line` saying
  where. This is the material, and the divergence is stated as the two things it
  is made of — what the preference says, and what the repository does — never as a
  verdict about which is right.
- **Could not tell.** The pass did not establish it. This goes to the user as a
  gap, plainly labelled, and **never as a divergence** — "I did not find it" and
  "it is not there" are different claims and only the first one is yours.

Then sort what the sweep found — or, on the empty branch, everything the detailed
pass found. This is the harder judgment and worth being honest about: **not every gap is a preference worth writing.** The test this skill
uses is whether a real alternative existed and somebody picked between them. A
repository that uses `pytest` where `unittest` was right there made a choice; a
repository whose files end in `.py` did not. That test is a judgment and not a
rule — when a candidate sits on the line, it goes to the user as a *question*
about whether they have an opinion, not as a proposal they have to reject.

Two things are dropped here regardless of which pile they landed in:

- **Anything the manifests answer.** A finding that says "this repository pins GORM
  v1.25" is not a candidate preference; it is a row of an inventory that is already
  in the tree and always current. What survives is "this repository uses an ORM
  where generated queries were an option", which is a choice somebody made.
- **Anything already true and already recorded.** A repository obeying its
  preference has nothing to record, and appending a record saying so would leave
  two ids for one fact.

### 4 — Put the divergence in front of the user

Open with one line of arithmetic, so they can see what was checked without being
shown it:

> Miré 7 preferencias contra el repo. 5 las cumple. Te traigo las 2 que no, y 1
> que no pude verificar.

Then the divergences, one each, as the two halves and the evidence:

> **`persistence`** — la preferencia dice *acceso a datos con consultas generadas,
> no ORM*. El repo usa GORM en `internal/store/user.go:12-40` y en otros cuatro
> lugares. ¿Cambió la preferencia, o esto es deuda que ya conocés?

That question is the only one worth asking, and it is worth asking in exactly that
form, because a divergence has two honest endings and they produce opposite
records. **The preference changed** — a new `preference` supersedes the old one.
**The repository is behind** — the preference stands, nothing is written, and what
that leaves is work, which is `plan`'s to open and never this skill's.

The gaps go in the same round, labelled as gaps and not folded in with the
divergences.

**On the empty branch, nothing above applies, and this is where that matters
most.** There is nothing to diverge from, so **no divergence is presented** — and
saying "the repository diverges from nothing" would be the same claim in a
politer voice. What is presented is a **proposal**, and it is named as one:

> Todavía no hay preferencias guardadas, así que esto no es una comparación: son
> propuestas. Encontré tres elecciones que el repo hizo donde había alternativa
> real. Decime cuáles son tuyas y por qué, y cuáles son simplemente lo que había:
>
> 1. Consultas generadas en vez de ORM — `internal/store/` — ¿decisión o herencia?
> 2. …

The distinction is the whole reason this branch exists separately. A proposal the
user waves through is a preference; a proposal *presented as a finding* is the
repository's status quo laundered into a decision, and once it is on the record
nobody can tell the difference.

Ask the discrete parts in **one** `AskUserQuestion` call — one call for the round,
not one per divergence — and ask the open part in words, because multiple choice
cannot carry "the third one is wrong for a reason you did not list" and it cannot
carry an area nobody looked at:

> Y decime qué falta: alguna elección tuya que no esté acá, o algo que anoté mal.

**What the user does not say, nothing writes.** A divergence they did not answer
is not a preference and not a question they can be assumed to have settled.

### 5 — Write what the user settled

One record per preference, never one record per conversation. Everything on the
record is in English, whatever language the round was held in.

Before the first write, settle the tags. Hard rule 7 proposes them from the log
itself, because nothing in the CLI validates a tag's content and the areas that
exist *are* the known list. Look in two places, in this order, and stop at the
first that fits:

**The areas that already answer for preferences** — step 1's own
`areas --kind preference` rows, already in hand, no new call. This is where a
preference's tag belongs when the repository has any: reusing `persistence`
because a preference is already filed under it is the whole of what keeps two
writers from keying one question as `persistence` and `database`.

**Only when none of those fits**, the wider vocabulary — the areas the log's
*other* choice-shaped records use:

```bash
env -u AI_KIT_PLAN memory areas --kind decision,adr
```

`--kind decision,adr` and not an unbounded call, for two reasons that hold whatever
size the log grows to. It **cannot return a `preference` at all**, so it is a read
of tag *names* and unambiguously not a read of preferences — the preference
vocabulary having already been covered for free by the line above. And it is where
a choice-shaped tag actually lives: a tag that has only ever appeared on an
`assumption` or an `open-question` is not vocabulary for a choice.

The bounded call is also the smaller one, and no number for that is written here on
purpose: both counts climb every time the loop closes an objective, and the two
attempts to record them in this file each went stale inside the session that
measured them.

`areas` takes exactly three flags — `--dir`, `--kind`, `--plan` — so `--kind` is
the only bound available and there is no limit flag to reach for. What keeps this
cheap is therefore not a flag: read the rows that name the surface this preference
is about and leave the rest, and put that handful in front of the user rather than
the whole enumeration.

Then propose the closest existing tag, name it to the user, and coin a new one
only when they say so in words.

```bash
env -u AI_KIT_PLAN memory add --kind preference \
  --title "<the choice, stated as a choice: 'Generated queries over an ORM for data access'>" \
  --body "<why this one, and what it displaced. The reasoning is the record: an
alternative that was considered and rejected, the cost accepted, the constraint
that decided it. No version, no package name as a fact — the manifests hold those
and hold them correctly.>" \
  --about <area>,<path-prefix>/
```

Five things about that write carry the weight:

**No `--plan`, by hard rule 3 — and the absent flag is only half of it.** When the
flag carries nothing the CLI reads `AI_KIT_PLAN` from the environment, which
`memory --help` documents as a supported way to name a plan, so a user who
exported it in a shell profile turns every write here into a plan-stamped record.
Measured, with `AI_KIT_PLAN=some-tree` exported and no `--plan` on the command:

```
add --kind preference --about persistence      →   origin.plan: "some-tree"
env -u AI_KIT_PLAN add --kind preference …     →   origin.plan absent
query --plan other-tree --kind preference --current
                                               →   returns the second, not the first
```

Which is the whole failure in three lines: the stamped record drops out of every
tree but one, the log is append-only so it cannot be taken back, and nothing in
the output of the write says anything went wrong. `env -u AI_KIT_PLAN` is what
prevents it.

**Then check it landed clean.** `add` prints the whole record, so this costs
nothing but reading one field: `origin.plan` must be **absent** from the output.
If it is there, the record is already on disk and cannot be edited — say so, write
the plan-less one, and supersede the stamped record with it.

**The first `--about` is the area.** `areas` reports `key: true` for whichever tag
came first and enforces nothing, so the position is a convention this skill keeps
and the CLI does not. Put the area first and the path prefixes after it.

**The title is the index.** Kind plus title in chronological order is how these get
read months later, so state the choice, not the topic: "Generated queries over an
ORM for data access", not "Notes on persistence".

**The body is the half that has nowhere else to live.** A body that restates the
title has recorded nothing. What was rejected and why is usually the sentence a
later reader actually needed.

#### When it replaces one that is already there

Two commands, not one, and the second is not optional. **They are two separate
runs, with nothing captured between them:**

```bash
env -u AI_KIT_PLAN memory add --kind preference \
  --title "<the choice as it is now>" \
  --body "<why it changed, and what the previous one got wrong or outlived>" \
  --about <area>,<path-prefix>/
```

```bash
env -u AI_KIT_PLAN memory link --type supersedes \
  --from <the id the add above printed, at .record.id, pasted literally> \
  --to <the id step 1 reported as `live` for that area>
```

There is no `NEW=$(…)` around the first command, and that is not an oversight —
it is the same correction `ask` and `bootstrap` already carry for their target
paths. **Nothing here says which shell this turn got**, and an assignment of a
command substitution is one of the shapes a shell may reject outright, before any
of the block's lines have run. The claim is about this shape and no wider one — the
`env -u AI_KIT_PLAN` prefix in front of both commands is a different construction
and runs everywhere.

In most places that would be a formatting nit. Here it is not: these two commands
land in an append-only log, and the failure is not symmetric — the `add` half
writes a durable `preference` and the `link` half never runs, leaving the area
with two live answers and no way to take either back. That is precisely the state
hard rule 6 exists to prevent, produced by the mechanism meant to prevent it.
Nothing is lost by dropping the capture — a variable would not survive to the next command
either — and `add` prints the whole record, so the id is on your screen at
`.record.id`. Paste it. An id you can read in the command is an id you can see is
wrong.

**This next sentence reports a measurement rather than stating a rule**, which is
why it names shells at all: both commands were run in the form written above under
`fish` and under `sh`, and they behave identically in each, multi-line `--body`
included, leaving one live answer. What was measured is that this form survives two
shells — not that yours is one of them.

The edge runs from the new record to the one it replaces, and step 1 is what told
you which id that is — the `live` field of the area. **Without the edge both
records keep answering `--current`.** Measured, on two preferences sharing one tag:

```
before the link:  query --kind preference --about persistence --current   →   count: 2
after the link:   query --kind preference --about persistence --current   →   count: 1
```

and `areas --kind preference` names the surviving one as that area's `live` in both
states, which is exactly why the edge matters: without it the two commands
disagree, one saying there are two live answers and the other picking one by date.

**When that area's `live` is `null` there is nothing to point the edge at, so only
the `add` runs and no `link` at all** — the id you would otherwise reach for
belongs to a record something already retired. Brief because it is rare: the state
needs an `answers` or `supersedes` edge onto the last preference in an area, and
nothing in this repository writes one. It was produced to measure it, and it comes
back `records: 1, live: null` with a per-area `query --current` of `count: 0`.

#### When a preference stops being true and nothing replaces it

`supersedes` needs two endpoints and there is only one. This skill has no good
answer for that case and does not fake one: **it writes no edge, the old record
stays live, and the situation is recorded as what it is.**

```bash
env -u AI_KIT_PLAN memory add --kind open-question \
  --title "<the preference, named> was reported as no longer true, and nothing replaced it" \
  --body "The user said this preference no longer holds but did not name a
successor. It is still returned by --current, because supersedes needs a record to
point at. What replaces it is unsettled." \
  --about <the same area>
```

Say that out loud in the same turn, because it is the one outcome the user would
otherwise discover by reading a stale preference next month.

#### The probes nobody could answer

An area the user could not settle — they inherited the repository, the person who
chose is gone — is an `open-question`, not a guessed preference:

```bash
env -u AI_KIT_PLAN memory add --kind open-question \
  --title "<the question, as it was asked>" \
  --body "Asked during a prefer run; nobody in the conversation knew why this
choice was made." \
  --about <area>
```

An invented rationale is the worst possible content for this log: it is
indistinguishable from a recorded one and nothing will ever check it.

#### Confirm what landed

```bash
env -u AI_KIT_PLAN memory areas --kind preference
env -u AI_KIT_PLAN memory query --kind preference --about <each area you wrote to> --current
```

**These two carry the same filter as each other and as step 1's**, and that
identity is the point: a confirmation narrower than the read that set the baseline
reports on records nobody looked at. What has to be true is one live answer per
area you touched — `count: 1` from the query, and the id you just wrote sitting in
that area's `live`. `count: 2` is the edge you did not write. The *old* id in `live`
is the edge drawn backwards, from the record being replaced to its replacement.
Nothing already written is ever edited, so the fix in both cases is another record
or another edge, and it is said out loud rather than quietly appended.

Then say what landed, in words: how many preferences, in which areas, and that the
log is append-only so this is not a write anybody takes back.

### 6 — Offer to carry them up, and stop

A preference that only exists in this repository's log dies with this repository.
The user log is where one outlives it — and moving it there is the user's act, by
hard rule 8.

Show what it would do before asking, rather than describing it:

```bash
env -u AI_KIT_PLAN memory promote --kind preference --to user --dry-run
```

`--dry-run` writes nothing at all — not the records, not the edges, not the
destination directory — and reports the same shape it would otherwise, plus
`dry_run: true`. Read three fields out of it with the user:

- **`promoted`** — the ids that would travel. `--kind preference` selects **every**
  preference in this repository's log, the retired ones as well as the live ones,
  and that is the behaviour you want here: a preference that arrived without the
  one it replaced would arrive as an assertion.
- **`edges_copied`** — the `supersedes` edges that travel with them. This is what
  keeps the user log's answer single rather than double.
- **`border`** and **`edges_skipped`** — what the walk stopped at. The reach is one
  hop from the named set and never the transitive closure, and it is about the
  destination rather than about this run, so a record some earlier promotion left
  half-connected is reported here too.

Then ask, once, and run it without `--dry-run` only on a yes:

> Estas `<n>` preferencias pueden subir a tu log de usuario, donde sobreviven a
> este repo y las ve cualquier otro. ¿Las subo? Si preferís sólo algunas, decime
> cuáles y las mando por id.

`--ids` and `--kind` are a union, so a subset goes up by id — and one hop still
applies, which is worth knowing rather than discovering: naming only the live
record of a superseded pair carries the record it replaced along with it, because
the edge joins them.

Two refusals, so neither surprises you mid-turn. `--kind` takes one of `adr`,
`dead-end`, `postmortem`, `preference` and refuses anything else with
`kind_not_promotable` at exit 1. `--to repo` is refused outright with
`target_removed` — only `--to user` crosses logs.

A second run is safe: ids already at the destination come back under
`already_present` rather than being written twice.

Then stop. If the run turned up work — a divergence the user confirmed is debt, an
area nobody could settle — name it in one line and leave the next move to them.
Turning it into an objective is `plan`'s job, invoked by them.

## Key principles

- **Read the record first, look at the repository second.** The other order
  produces a description of the status quo wearing a decision's clothes, and
  nobody downstream can tell the difference.
- **The manifests keep the inventory; the log keeps the reasons.** A version in a
  record is a fact with an expiry date and no way to notice it passed.
- **Divergence is the product.** A preference the repository already obeys is
  worth exactly nothing to say out loud, and saying it anyway is what trains
  somebody to stop reading these rounds.
- **A proposal is named as a proposal.** On a repository with nothing recorded
  there is nothing to diverge from, and presenting a finding as though there were
  is how the repository's habits get laundered into somebody's preferences.
- **No plan, no sentinel.** These records belong to the repository, every
  plan-bounded read returns them anyway, and a slug invented to look like
  ownership would be a lie the log would carry forever.
- **One live answer per area, and the edge is what makes it one.** The convention
  is computed in exactly one place; everywhere else somebody has to draw it.
- **What nobody knew is recorded as unknown.** An invented reason is
  indistinguishable from a real one, forever, and nothing will ever check it.
- **It records and stops.** Promotion is offered, work is named in a line, and
  what happens next belongs to the user.
