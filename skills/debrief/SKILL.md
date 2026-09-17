---
name: debrief
description: "Read code the person wrote themselves and tell them two things: what they missed, and what would be better — each one with the reasoning that makes it worth keeping rather than a line item to close. It changes nothing on disk except one record in the log: no edits, no fixes, no patch offered at the end, because the person who wrote it is the person who is going to fix it. Works with a `brief.md` on disk, where 'what you missed' has a referent the user wrote and corrected, and without one, where the intent is read off the diff and the report says so in those words. Triggers on: 'ya lo escribí', 'terminé de escribirlo', 'ya está el código', 'qué me faltó', 'qué estaría mejor', 'leé lo que escribí', 'dame un debrief', 'hacé el debrief', 'debrief this', 'I already wrote it', 'tell me what I missed', 'read what I wrote and tell me what is wrong'. It is the second half of the door `brief` opens, and it never writes production code."
---

# debrief

Read [the host runtime](references/runtime.md) first for tool invocation,
delegation, repository paths and the memory prerequisite.

The second read, after the code is written and by somebody who did not write it.
Two halves: what is missing against what the work was trying to do, and what
would be better. Each finding carries the reasoning, and nothing on disk moves.

This is not `review`. `review` is the pass you ask for over code an agent wrote,
on an objective whose acceptance criteria something else already graded, so
whether the work achieved what it was *for* is settled before it starts. Here
nobody graded anything, the reader wrote every line, and the intent is theirs —
so "what you missed against what you were trying to do" is the half that only
exists on this side of the door, and the *why* under each finding is the actual
product.

There are two modes and they are not equally easy. With a `brief.md` on disk,
"what you missed" has a referent the user wrote and corrected: what we were doing
and which checks they meant to run by hand. Without one, there is no referent,
and the honest move is to read the intent off the diff, say out loud that it is
inferred, and let them correct it. The second mode costs nothing to enter, which
is probably why it is the one that gets used.

## Hard rules

1. **Never write code, and never fix anything.** Not a patch at the end, not "the
   line should read", not a helpful edit while you were in there. Every fix is
   theirs, including the obvious ones — an obvious fix pasted is a thing they
   would have learned turned into a thing they accepted. Reading the repository
   is the whole job; writing to it stops at the log.
2. **The reasoning is the deliverable.** A finding that says what to change and
   not why the other way is better is a ticket, and this flow is not a ticket
   queue. If you cannot say why, you do not have a finding yet.
3. **Nothing here is graded and nothing here is scored.** No numbered checklist,
   no pass, no percentage. The two halves are read and argued with, and a number
   at the top is what stops them from being read.
4. **The inferred intent is always labelled as inferred**, in the report and in
   the sentence you say out loud, in those words. Their whole ability to correct
   your reading depends on knowing there was one.
5. **One record at the close, and at most one edge.** With a brief: one
   `review-finding` and one `met` edge on the objective the brief opened. Without
   one: the `review-finding` alone, and no `objective` record — writing one after
   the fact would be recording a plan that never existed.
6. **Do not pad, and do not invent a finding to fill a half.** "Nada te faltó" is
   a real outcome and a short report anybody reads beats a long one nobody does.
7. **Conversation in the user's language; everything written to disk in English.**

## The memory log

State lives in an append-only log, read and written only through the `memory`
CLI. Never edit the log by hand.

Every placeholder in a command below is quoted, and that is a precaution rather
than a habit: **nothing here says which shell this turn got**, and three shapes
vary between them — an assignment, `${braces}`, and a bare `<…>` that a shell may
read as a redirect. A shell that refuses one of them refuses the whole block at
parse time, with none of its lines having run. A block written this way is one you
can paste into whatever shell this turn happens to have.

**The writes in step 6 name their tree with `--plan`.** The reads in steps 1 and
3 do not, and they are prefixed `env -u AI_KIT_PLAN`: step 1 is looking for an
objective whose tree it does not know yet, and step 3 asks what this *repository*
already settled — a `gotcha` binding the surface they just touched was very
likely recorded under some other tree, and a plan-bounded read hides it behind a
`count: 0`. Omitting the flag alone is not enough. The CLI falls back to the
`AI_KIT_PLAN` environment variable — exit 0, no warning, bounded again to
whatever an earlier session exported — which is why the prefix is there and not
just the missing flag.

## Steps

### 1 — Find the brief, or say plainly that there is none

```bash
find .ai-kit/plans -maxdepth 2 -name brief.md
```

- **Nothing comes back** — standalone mode. Say so in one line, and carry on. It
  is the ordinary case, not a degraded one.
- **Exactly one** — read its title line, and do not take it as settled yet. One
  brief on disk is not evidence that it is *this* brief: once briefs accumulate,
  the likeliest state of the only one you find is that it belongs to some earlier
  piece of work. The user is the one who knows, and you ask them below.
- **More than one** — ask which. Name them by their directory and their title
  line; do not pick the newest by mtime and hope.

Read the one you have, whole. It carries a title and seven sections — What we are
doing, The terrain, What you decided, What it displaced, Where this touches,
Manual checks to run, What is still open — and any of them may be thin or missing
entirely, because a brief is written to the conversation that produced it and not
to a template that must be filled. Take **What we are doing** as the intent and
**Manual checks to run** as the checks, and where a section is absent, treat it as
absent rather than reconstructing what it would have said.

Then find the objective that brief opened. Its `--slug` is the name of the
directory the brief sits in:

```bash
env -u AI_KIT_PLAN memory query --kind objective --slug "<that directory name>"
```

Two different strings come out of this and **collapsing them is the mistake to
avoid**: the record's own `slug`, which is the directory name you just passed, and
the tree slug at `origin.plan`, which is what `--plan` takes in step 6. Read the
tree slug off the record rather than assuming the directory name is also it. The
record's `id` is step 6's `--from`.

`brief` writes that record and deliberately no edges, so `planned` is the state
you should find it in. Read the state rather than assuming it — `work` picks from
the open objectives in the log and this is one of them, so it may already carry an
`approved_for_execution` or a `met`:

```bash
env -u AI_KIT_PLAN memory state --id "<that objective id>"
```

The state is at `.objectives[0].state`.

Then put the brief and that state in front of the user, and wait for a yes before
reading a line of code:

> Encontré este brief: «`<el título>`» (`<el directorio>`), objetivo en
> `<el estado>`. ¿Es contra esto que leo lo que escribiste?

The question costs one line and what it prevents does not: judging today's diff
against an older intent, and then closing an unrelated objective with a `met` edge
an append-only log cannot take back. If they say no, they either name another
brief or you go on in standalone mode — their call, not yours.

**When the state is anything but `planned`**, say which state it is and read the
code anyway, but that objective is not yours to close: step 6 writes the finding
and skips the edge. Something else already moved it, and a `met` stacked on top of
somebody else's sequence records a close that did not happen here.

**In standalone mode there is no record and no tree**, so coin a tree slug from
the work itself — lowercase, runs of anything outside `a-z0-9` collapsed to one
hyphen — and say it out loud at the close. A finding written under a slug nobody
was told is a finding nobody will ever find again.

### 2 — Name what changed

The files to read come from the working tree, which is where hand-written work
sits before anybody commits it:

```bash
git status --porcelain
git diff --name-only
```

Together those give the untracked, the staged and the unstaged. That is the list,
and step 3 needs it as much as step 4 does.

**When the tree comes back clean**, the work is already committed, and the anchor
is the moment the brief was written — the `at` on the objective record from step
1:

```bash
git log --oneline --since "<the at timestamp on the objective record, pasted literally>"
```

```bash
git diff --name-only "<the commit just before the oldest one that log named>..HEAD"
```

In standalone mode there is no such anchor, so a clean tree means asking: which
commits is this, or since when. Do not guess a range; a range guessed wrong reads
somebody else's work and reports it back to them as theirs.

Show the user the file list before spending a subagent on it. A list with the
wrong files in it is cheaper to correct here than to discover in the report.

### 3 — Read what this repository already settled

Ask which areas exist before filtering on one, because a tag you guessed comes
back empty and empty is exactly the answer that must not be misreported:

```bash
env -u AI_KIT_PLAN memory areas --kind gotcha,preference
```

That names every area anybody recorded a gotcha or a preference against, in this
repository's log and the user's. A tag is the only bound `query` has on the
surface a record is about, so take the `--about` values from what it names — the
ones the files step 2 named land in — and read them one at a time:

```bash
env -u AI_KIT_PLAN memory query --scope both --kind gotcha,preference --about "<area>" --current
```

**`--scope both`, always.** One call reads this repository's log and the user's as
one answer, each record naming in its own `scope` which of the two it came from.
On the default `query` reads this repo's log alone — so a preference promoted up
out of another repository is named by `areas` one line earlier and then comes back
empty here, from a query that ran correctly and asked the wrong log. It binds this
code exactly as much as a local one does, which is why step 4 hands `evaluate` the
two scopes as one list.

**One call means one threshold over both logs.** `--scope both` pools the two
before the guard runs, so the count measured against the line at which `query`
withholds its records is their sum, and an area that sat under it in each log
alone can cross it here. Past that line the answer is `count: N, returned: 0,
truncated: true` at exit 0, and what step 4 would hand `evaluate` is an empty
list out of an area that is anything but empty. Re-read that one area with
`--scope both --threshold -1 --fields id,kind,scope,title,at --body-lines 0`,
which names every match with the log it came from and reads none of them, then
take whole by `env -u AI_KIT_PLAN memory query --id --scope both` the
ones that bear on the files step 2 listed: `--id` is bound by the scope
and the plan it is asked under, so without both a record the pooled read
just found in the user's log comes back under `not_found` at
exit 0. Step 4 hands `evaluate` the records themselves, so a title is enough to
choose which ones those are and never enough to hand over. `--limit` would
bound the pool rather than each log, and the guard counts the matches before
`--fields` and `--body-lines` project them — they are no way past it, and what
they change is what being past it costs.

A `gotcha` is the highest-value thing in this log for the half of the report that
asks what was missed: it is a constraint the code obeys without stating it, so
nobody finds it by reading the diff. Check whether the code it cites is still
where it was, and hand the verdict on to `evaluate` with the record:

```bash
env -u AI_KIT_PLAN memory anchors --kind gotcha --path "<a path or a directory from the step 2 list>" --current --verdict changed
```

**`--path` here and not `--about`**, because this is the one bound in this step
that needs no guess: it matches the anchor's own cited path as a case-insensitive
substring, so the file list step 2 already produced *is* the filter, and one call
per directory the change touches asks which recorded constraints are anchored in
the code this work actually edited. `query` has no `--path`, so the tag guess
stays on the lines above and stops here.

**`--verdict changed` because that is the verdict this step reaches for first.** A
gotcha whose anchors came back `changed` is a constraint that may already have
been broken by the work you are about to read — a finding, not a housekeeping
note — while one that still holds is a constraint the diff left where it was.
Neither flag narrows the check: `matched`, `checked` and `counts` are computed
before `--path` and `--verdict` see anything and still describe every anchored
gotcha the filter admits, and `returned` says how many of them came back. So
`returned: 0` reads as "nothing recorded in these files drifted" and never as "the
log is empty", and `counts` beside it is how you say that with a number.
`--verdict` takes one value, so a `counts.could_not_check` above zero is a second
call with `--verdict could_not_check`, and it is worth making: an anchor nobody
could resolve is not an anchor that held.

Say in one line what came back, including "nada". A filter that ran and found
nothing is a different thing from a read that was skipped.

### 4 — Launch `evaluate`

One subagent, which reads the code and returns the two halves. It gets what steps
1 to 3 resolved; it never queries the log itself, because the resolving is done
and doing it twice in a fresh context is how the two halves end up judged
against different things.

```
intent: <What we are doing, from the brief — or your own reading of the diff>
intent_source: <stated | inferred>
manual_checks: <Manual checks to run, verbatim — or empty>
repo_root: <absolute path>
changed_files: <the list from step 2>
constraints: <the gotcha and preference records from step 3, each gotcha marked with the verdict its anchors returned — changed, or could_not_check>
```

`intent_source` is `stated` only when the intent came from the user — a brief they
corrected, or a correction they made in this conversation. Your own summary of
their words is still `stated`; your reading of a diff is `inferred`, even when you
are confident, and especially then.

### 5 — Give them the two halves

Present the report yourself, in their language, in the order the agent returned
it: what was missed first, then what would be better. Keep every `reasoning` —
the reasoning is what they are here for, and a report compressed to its titles is
a list of chores.

**Say what `could_not_read` named out loud, before either half.** That field is
the agent's only way to tell you it could not judge part of the change, and a
partial read presented as a whole one is a report they will trust further than it
has earned. Empty needs no sentence. `holds_up` is worth one line when it came
back with something and nothing when it did not.

In standalone mode, open with `intent_read` — the agent's own sentence, not your
summary of it — and stop there for a beat:

> Ojo: no hay brief, así que lo que leí como intención es esto — `<intent_read>`.
> Si erré, corregime y lo vuelvo a mirar contra lo que era.

If they correct it, go back to step 4 with the corrected intent and
`intent_source: stated`. That second pass is the cheapest thing in this skill and
the one that decides whether the first half is worth anything.

Then let it be a conversation. They will argue with findings, and some of those
arguments will be right — they wrote the code and they know what it is for. When
one lands, say so plainly and drop the finding rather than defending it into the
record. When one does not, say it once more in a line and leave it standing:
nothing here blocks, so a disagreement can simply be a disagreement.

**When they ask you to fix something, you do not.** Talk it through as long as
they want — that is conversation, not editing — and the code stays theirs. Rule 1
is not a formality here; it is the reason this door exists.

### 6 — Write the close

Two runs, in this order, and nothing captured between them — and the second one
does not always run. **The `link` happens only when step 1 found that objective at
`planned`.** At any other state, and in standalone mode, the `add` below is the
whole close.

The finding first. Its body is what this read established, in the two halves,
built from what the agent returned and what survived step 5 — not from optimism.
**Whatever `could_not_read` named goes in the body too**: four files read out of
six, recorded as six, is the one error in here that nobody can catch later.

```bash
memory add --plan "<tree-slug>" --kind review-finding \
  --title "<what the second read established, in one line>" \
  --body "What was missed: <the findings that stood, with their reasoning>
What would be better: <the same>
Judged against: <the intent from the brief, or the inferred intent said to be inferred>
Not read: <whatever could_not_read named, or the word nothing>" \
  --about "<area>,<area>"
```

`add` prints the whole record and the id is at `.record.id`. Read it, and paste
it literally into the edge:

```bash
memory link --type met --from "<the objective id from step 1>" \
  --to "<the id the add above printed, at .record.id, pasted literally>"
```

Nothing captures the first command's output into a shell variable for the second,
and that is not an oversight. An assignment of a command substitution is one of the
shapes a shell may reject outright, *before any of the block's lines have run*,
while a shell that accepts it runs the block through — and nothing tells you which
shell this turn got. Nothing in an append-only log undoes whichever half already
landed. An id you can read in the command is an id you can see is wrong.

`met` takes a `decision` or a `review-finding` as its justification, so the
finding is enough on its own and no `decision` needs inventing to hold the edge.
Nothing verified anything here, and nothing was going to: this flow has no
gradeable list and the person who wrote the code is the one who read it. `met` is
the honest state anyway — the work was done and looked at — and the alternative
is a brief's objective that stays open forever because no machinery exists to
close it.

Those are the only two writes. **No edge saying this was approved for execution**,
not before and not now: `brief` opened this objective with zero edges on purpose,
and the one `met` is all this door ever adds.

**In standalone mode, the `add` runs and the `link` does not.** No objective
record is written either. There is nothing to point a state edge at, and the
finding under its coined slug is the whole trail — say the slug in the closing
line so it is findable:

> Queda el registro bajo `<tree-slug>` (`<id>`). No había brief, así que no
> inventé un objetivo para cerrarlo.

**And the same, for a different reason, when the brief's objective was not at
`planned`:** the `add` runs, the `link` does not, and the closing line names the
state you found it in and says you left it there. Say the rest of it too — that
unless that state was itself a close, the objective stays open, and that nothing
in this flow will close it, so closing it is theirs to decide and somewhere else
to do. Left unsaid, it is an objective open forever that nobody was told about.

## Key principles

- **The reasoning is the product.** They can find the bug themselves eventually;
  what they cannot get from their own diff is why another shape would have been
  better. Strip the reasoning and this skill is a linter with worse latency.
- **Nothing on disk changes but one record.** No fixes, no patches, no "I went
  ahead and". Someone chose this door to write the code themselves and that
  choice does not expire when the code turns out to be improvable.
- **Inferred is said out loud, every time.** The standalone half of this skill is
  only honest if the guess is visible as a guess.
- **A short report gets read.** Two findings with real reasoning beat twelve with
  titles, and the twelve get skimmed together.
- **Nothing here is graded.** There is no list to score against, by design: the
  person implementing was in the room the whole time.
