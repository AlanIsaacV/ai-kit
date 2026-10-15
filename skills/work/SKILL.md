---
name: work
description: "Execute one objective end to end: resolve which objective you are on, confirm it is executable — its acceptance criteria each carrying the fidelity they require — freeze those criteria, implement it — split into parts that cannot conflict and run at once where the work allows it — verify it against them in fresh subagents that did not write the code, review the code when a review is asked for, record what was learned, and decide what comes next. Use whenever an objective is already recorded and the user wants to move it forward. Triggers on: 'sigamos', 'continuemos', 'seguí con el plan', 'arrancá', 'dale', 'implementá la siguiente parte', 'retomemos', 'continue where we left off', 'what's next', 'terminá lo que quedó', or the start of a fresh session on work already planned. This skill executes and does not author: when no objective exists yet, when the one you are on is too big for a session, or when this turn discovers work nobody has framed, it invokes the `plan` skill to write that objective and get its criteria approved, then comes back here to run it."
---

# work

Read [the host runtime](references/runtime.md) first for tool invocation,
delegation, repository paths and the memory prerequisite.

One turn of the loop: pick the objective this turn executes, build it, check it
honestly, learn from it, decide what is next.

You are the orchestrator. You hold the objective, you talk to the user, and you
delegate the actual work. You do not write production code and you do not
verify your own criteria — you ran the work and own the outcome, which is
exactly why you are the wrong one to judge whether they were met.

Objectives nest. An objective may derive from another, and the one this turn
runs against — the **executable objective** — is the one whose criteria carry a
required fidelity. That is the whole marker: an objective is executable when its
own criteria say what fidelity they need, whatever sits underneath it, because
depth suggests and does not decide.

Opening a more specific objective underneath the current one is an ordinary move,
not an admission that the plan was wrong: it costs one record and one edge, and
the parent is still there afterwards. Writing it is `plan`'s job rather than this
skill's — here you notice that the objective in front of you is not the one to
run, and invoke `plan` for the one that is.

## Hard rules

1. **The objective is loaded first and stays loaded.** Every objective answers
   "how does this move the one it derives from" — a root objective answers to
   the request that opened it. An objective that cannot answer it is an
   objective that should not run.
2. **Never write production code in this thread.** That is the job of the parts
   step 5 launches — the `implement` subagent, or Codex where that step says so.
   Reading a few files to check drift or spot-check a claim is fine; editing
   project source is not.
3. **The user approves the executable objective before anything is built.** Hard
   stop, every time. Silence is not approval.
4. **Acceptance criteria are frozen at approval.** Nothing downstream may add,
   soften or reinterpret them. A condition discovered later is an
   `open-question` for the user, never a criterion the system grades itself
   against. **The user may drop one outright, in words, and that is the only
   exception** — steps 4 and 8 say how. Dropping is not softening: a criterion
   that is gone grades nothing, where a criterion that was made easier grades the
   same work against a lower bar and reports it green.
5. **Verification runs in a different subagent than implementation**, with
   fresh context.
6. **State changes go through the `memory` CLI, and at the close it is the
   `record` subagent that runs it.** Never hand-edit the log. A state is an
   appended edge pointing at the record that justifies it, so nothing already
   written is ever rewritten to change one. This thread writes the freeze at
   step 4 and the blocker at step 5; everything the close records is written by
   the subagent, and this thread's job afterwards is to read the log back and
   confirm it landed.
7. **Conversation in the user's language; everything written to disk in
   English.**

## The memory log

Queries report their match count before handing over records, and withhold
them above a threshold. When that happens, **narrow the query** — another
`--about`, one `--kind` at a time — and do not raise the limit to make the wall
of text appear. `--fields` and `--body-lines` are not a way out of this: the
threshold counts what matched before any projection runs, so a withheld answer
stays withheld however little of each record you asked for. What they change is
the price of the records that do come back. If you genuinely need to read a lot
of history, that is a job for a subagent, not for this context.

The surface is twelve commands and these are all of them: `init`, `add`,
`link`, `query`, `search`, `areas`, `anchors`, `edges`, `state`, `promote`,
`validate`, `kinds`. The steps below run the ones this loop needs and
write the flags out at each call site, so there is no command here to go and
discover. `memory kinds` stays worth running for what it alone reports: every
valid record kind and edge type, which record each state edge is allowed to
point at, and the count at which `query` starts withholding.

Another repository's records are one flag away: `--dir` resolves the log of
whichever repository the path sits in, so
`memory query --dir <another-repo> --kind adr,decision --current` answers out of
that repository without this turn leaving its own. Reach for it when the surface
this turn touches lives in more than one repository — an `adr` written next door
binds this change exactly as much as a local one — and put `--repo-root` beside
`--dir` on `anchors`, which is what lets the check read the cited lines out of
that working tree.

## Steps

### 1 — Resolve which objective you are on, load only the history that bears on this turn, and put it on the table

```bash
memory query --plan <tree-slug> --kind objective --current
memory state
memory query --plan <tree-slug> --kind open-question,assumption --current
```

Those three are where the turn starts, and they are bounded by the tree's shape
rather than by its age. The gotcha block below them is the rest of the start-up
read: a `gotcha` is loaded and checked every turn, because a constraint nobody
loaded is a constraint this turn breaks. Nothing else runs unasked.

Pull anything else on demand and by area — `--about <area>` for the surface this
turn will touch, `--kind wont-fix` before proposing work that someone already
decided was not worth doing, `--kind adr` for that same surface before building
on it, because an ADR outlives the objective that wrote it and one written
elsewhere can still bind what this turn changes.

#### Read what this tree already owes, before anything gets chosen

`memory state` above reports every objective's `state`, and any of them reading
`blocked` is an objective this loop stopped rather than closed. What it is waiting
on is the record the block points at:

```bash
memory state --text
```

That prints **Blocked on a question or a decision**, grouped by the record each
objective waits on, over every log in the store rather than this tree alone. It is
the ledger for the debt step 9 writes — a `you-run` check nobody ran, a landing
condition that has not happened — and reading it *here* is what makes the debt
something the next turn pays rather than something the last turn recorded.

**Name every blocked objective in this tree before step 2 chooses anything**, one
line each, with what it waits on. The ask is short because there are only ever
three answers: they ran it and it passed, they ran it and it failed, or it is not
going to be run. The first closes that objective `met` this turn, before a new one
starts; the second is a fix; the third is a `wont-fix` in their own words and then
the close.

> Antes de elegir: `<title>` quedó bloqueado esperando `<lo que falta correr>`.
> ¿Lo corriste, falló, o no se va a correr?

**Nothing forces them to pay it now.** "Later" leaves the objective blocked and
this turn goes on to something else. What changed is that the debt was named
before the next one was taken on, which is the whole distance between one unpaid
check and seven. Several in one tree is a count and one line, not a list — it is
the same sentence about the same loop, and repeating it per objective is how it
stops being read.

#### Read the gotchas for this turn's surface, and check they still point at code

```bash
memory query   --plan <tree-slug> --kind gotcha --about <area> --current
memory anchors --plan <tree-slug> --kind gotcha --about <area> --current --verdict changed
```

A `gotcha` is a constraint the code obeys without stating it, written because
reading the code will never recover it. Nothing regenerates one, so a session
that does not load them is a session that breaks them — and the second command is
what says whether each one still describes the code it was written about. It is
one Go process and no tokens: no model call, no network, milliseconds.

**The two lines carry the same filter, and that identity is the point.** A query
narrower than the check reports on records the check never saw, and a check wider
than the query verdicts records nobody loaded — either way the round would be
describing two sets while reading as one. Change one line's filter and change the
other with it.

**`--verdict changed` is not part of that filter, which is why it can sit on one
line and not the other.** `anchors` matches, checks and counts before `--verdict`
decides which records to hand back: `matched`, `checked` and `counts` are
computed over everything the filter admitted and still answer for the whole set,
and the flag only shortens `records`, adding `returned` to say by how much. Run
the line both ways against your own log and the two answers show it, whatever the
day's numbers are: `matched`, `checked` and `counts` come back the same in each,
the only thing that moves is the length of `records`, and `returned` is on the
filtered answer alone to say how many of that checked set it kept. So every record
the query loaded was still checked — `counts` is the proof — and what the flag
saves is the bodies of the records there was no move to make about.

`--current` drops what something else supersedes, and for a gotcha that is not
tidiness: **a re-anchored constraint supersedes the record it replaces**, so the
stale citation is meant to stop being checked the moment the replacement exists.
Both are in the log, only the current one is loaded, and a `changed` verdict on a
record somebody already fixed would send this round back to the user every turn
for work that is done.

`--plan` leaves out the gotchas *other* objective trees wrote about this same
area, and that filter answers to the rule below under **Put the carried-in
questions and assumptions in front of the user**: name what it dropped in one
line, with its count, on exactly the grounds stated there. The same query without
`--plan` and with `--threshold 1` is where the count comes from: it reports the
true number and withholds the records once there is more than one, so naming what
was dropped does not cost the round the thing the bound was there to save.

Every anchored record carries one verdict and `counts` says how many took each.
Only one of the three comes back as a record, and it is the one that needs you:

- `changed` — the cited lines were rewritten. **Put it in front of the user in
  this same round**, with the gotcha's title and its `path:line`. Nobody can tell
  from here whether the constraint still binds: either the rewrite respected it
  and the record wants re-anchoring, or the rewrite broke it and this turn is
  about to build on something that is no longer true. That is a question for the
  user, not a call to make quietly.
- `still_holds` — the cited lines are still there, and `counts.still_holds` is
  the whole of what this round needs from it. `moved: true` on a record you did
  get back means the lines sit at a different number now, which is not a problem
  and needs nothing from you.
- `could_not_check` — the file is gone or the anchor cannot be resolved. Say so
  with the same weight; an unchecked claim must never be reported as a confirmed
  one. `--verdict` takes one value, so this one arrives as a number:
  `counts.could_not_check` above zero is a second run of the `anchors` line with
  `--verdict could_not_check` in place of `--verdict changed`, and it is worth
  the call every time it is not zero.

Never say a gotcha holds because the check said `still_holds` about *some* of the
records — the count is per record, and `checked` next to `matched` says how many
carried anchors at all. Nothing is rewritten by this. The check only reports: a
constraint the user says still binds is re-anchored by writing a **new** `gotcha`
against the code as it is now, which `implement` does with `--anchor` in step 5,
and the stale one is superseded rather than edited.

**Read the `current_objective` field, not just the records.** The objective
query returns it as `current_objective` and `state` returns the same answer as
`current`: an `id`, a `title`, and the `rule` and `why` that resolved it. The
rule is either `approved_for_execution` — an open objective carries the latest
freeze in the log — or `activity`, meaning none does and this is simply the one
most recently touched. Name that objective and its rule to the user in one line,
and when more than one objective is open, offer the switch and let them pick.
Do not choose silently among several; do not ask when only one is open, because
then there is nothing to ask.

`--current` returns **every** open objective rather than the one you are on:
settled for an objective means `met` or `cancelled`, and nothing else takes it
off that list — not depth, not having an open child, not `separated`. The
`depth` in `state` is advisory: it says where an objective sits, not which one
to work. A list coming back is not a cue to take the deepest one. The answer to
"where am I" is `current_objective` plus the user.

`state` is given no plan filter on purpose: `--plan` is accepted there and
ignored, because narrowing the derivation would change the answers rather than
the view. An objective's parent, its children and which objective is current are
properties of the log, not of the question. Several roots in one log are normal —
nothing requires an objective to have a parent.

`--plan` names the **objective tree** — the value in every record's
`origin.plan` — and not the drafts directory, and the two do drift apart. A
query that comes back `count: 0` while `current_objective` carries
`outside_filter: true` is telling you the slug is wrong, not that the log is
empty: the derivation read the whole log, found the objective and named it from
outside your filter. Take the slug from the objective's own record, not from the
directory its draft sits in.

A log written before objectives went recursive also holds `unit` records.
Nothing writes them any more and the read above does not ask for them, so their
absence is history rather than a gap.

Then look at reality, not just the record: `git log --oneline -15` and the
state of the files the objective's orientation named. **The objective was
written at a point in time and the repo has moved.** If something that it
assumed is no longer true, surface it now, before choosing what to execute on
top of it. Finding drift is a good outcome, not a delay.

#### Put the carried-in questions and assumptions in front of the user

The `open-question,assumption` query above pulls records written in earlier
turns, and loading them as ambient context is not the same as anybody having read
them. Close this step by putting them on the table in **one** round.

Filter first, and filter in the open. The test is narrow: a record bears on this
turn when it touches the objective about to run, or the surface that objective is
going to change. Everything else belongs to a sibling objective, or to code this
turn never opens, and it stays in the log. A round that dumps every open record
trains the user to skim the one thing you needed them to read.

That test runs on `title` and `about` and on nothing else in the record. When the
tree has enough history that the query above comes back as a wall of bodies, take
it in two: every open record named for the price of its title, the filter made on
exactly the fields the test uses, and the survivors read whole afterwards.

```bash
memory query --plan <tree-slug> --kind open-question,assumption --current --fields id,kind,title,about --body-lines 0
memory query --plan <tree-slug> --id <the surviving ids, comma-separated, pasted from the answer above>
```

Two calls where one was enough on a short tree, so it is worth it only once the
bodies outweigh the extra process — and it is no help at all against a withheld
answer, which stays withheld by the rule above. Narrow that one; this one is about
what the records cost after they come back.

Then name what the filter dropped, in one line, with its count. The judgment is
yours to make and it is not yours to make quietly — a record left out in silence
is a question the user never learns existed:

> Dejé afuera seis preguntas abiertas que no tocan esta superficie; decime si
> querés verlas.

Naming a count is not running a round over all of them, so the saving stays and
the user can still ask for any of it. When the filter leaves nothing, there is no
round — an empty one spends the same attention as a real one and buys nothing —
**and you say that** rather than moving on wordlessly. "No quedó nada abierto que
toque esto" is one line and it is the difference between a filter that ran and a
step that was skipped.

Then one message, with three parts:

- The assumptions as **numbered prose, one line each**: what was assumed, what it
  costs if it is wrong, and where it came from. Not a table, not a wall of record
  ids.
- The open questions that have discrete answers, batched into **one**
  `AskUserQuestion` — one call for all of them, not one call each.
- An open ask for what the list is missing, and for corrections by number. An
  assumption nobody would have picked out of a menu is exactly the one this
  round exists to catch.

Where an assumption came from is written into the record body as a line reading
`Provenance: <value>`, with one of three values — `you chose this`, `their words`,
`my call`. Show it with the assumption, because it is what decides how hard to
push here: a `my call` nobody has looked at since it was written is the one worth
arguing about, and `you chose this` is one the user already settled and does not
need to relitigate. Two assumptions that read identically are not the same risk
once you know which of them anybody ever agreed to.

When that line is absent the provenance is **unknown**, and unknown is not
`my call`. Records written before the convention carry no label at all, and
reading the weakest value into them would invent the very thing the label exists
to prevent — a provenance nobody stated. Say that it carries no label and let the
user say which it was.

> Antes de elegir qué ejecutamos: esto es lo que quedó abierto de turnos
> anteriores y toca lo de este turno. Corregime cualquiera por número, y decime
> qué falta que no esté en la lista.

This happens **before step 2 chooses the objective**, and the order is the whole
point — a carried-in open question can change *which* objective this turn should
run. Asked after the choice is made, the answer that would have sent you
somewhere else arrives against an objective already picked, and the cheap moment
to change course is gone.

**Nothing in this round gets labelled answered.** Answering is an appended
`answers` edge in step 9 and that edge needs a record to point at, so until one
exists the question stays open in the log however plainly the user spoke to it.
A round that quietly marks things resolved is how the log stops being a true
trace. The same holds for an assumption: the user answering the question next to
it is not the user confirming it, and only their words about that assumption are.

### 2 — Choose the objective this turn executes

Either the open objective you are already on, or one `plan` opens underneath it
because that one is too big for a session. Pick against what the objective
still needs, given what exists today.

You do not size the candidate here and you do not write it. Whether an objective
fits one session, delivers something demonstrable, is worth doing on its own,
names its non-goals and can say in one line how it serves the objective above —
that five-part test lives in `plan`, and it runs there with the user, ahead of
any criteria. So when the answer here looks like "not this one, something
smaller", the next move is to invoke `plan`, which step 3 says how to do.

Do not derive the objective from a pre-existing phase list. There isn't one, on
purpose: you are choosing against the current state of the repo, which is
better information than anything written weeks ago. Candidate children stay
**prose** — a sentence in the parent's draft — until someone is about to work
one. A record is for the objective being opened now; enumerating the rest of the
tree up front turns it back into the phase list this shape exists to avoid.

### 3 — Confirm the objective is executable, or send it back to `plan`

Drafts live at `.ai-kit/plans/<objective-slug>/draft-objective.md`: one directory
per objective. Take that slug from the objective record step 1 loaded — its own
`slug` field, which `state` and `query` both report, not the `origin.plan` that
names the tree — rather than deriving one from the title yourself. `plan` named
the directory after that same field when it recorded the objective, and a
derived name that lands one character off finds no draft and sends this step down
the no-draft branch for an objective that has one.

Both candidates hang off the objective's own repository root. `<root>` below is
the `root` step 1's `memory state` printed on that objective — the absolute
working-tree root of the log the objective is listed from — and you paste it in
literally. That is what the join is for: an objective can live in another
repository's log, and a relative pair resolves against whatever directory this
turn's shell is standing in, which finds nothing and reads as no draft. Step 1's
`memory state` runs bare and lists only this repository's objectives, so an
objective whose log is somewhere else is not among them: ask for that one by id
with `memory state --id <objective-id> --all`, which reads every log on this
host and reports the `root` this join needs.

Look in **both** locations under that root, the new one first, because
`plans/<slug>/` at the repository root is where drafts were written before this
moved and the ones already there are not migrated:

```bash
sh -c 'for p in <root>/.ai-kit/plans/<objective-slug>/draft-objective.md <root>/plans/<objective-slug>/draft-objective.md; do
  [ -f "$p" ] && { echo "$p"; exit 0; }
done; exit 1'
```

It prints the first path that exists, so `.ai-kit/` wins when both do — a slug can
exist in both places, and the newer file is the one `plan` last wrote to. What it
prints is absolute, and that is the path the subagents downstream are handed as
`objective_draft`. **Exit 1 is the no-draft branch and exit 0 is not**, which is
why this is not an `ls` over both paths: `ls` exits 1 whenever either argument is
missing, so an objective whose draft sits in exactly one location looks found and
failed at once, and the next edit that chains it with `&&` sends a draft that
exists down the no-draft branch. A root left unsubstituted reaches neither branch:
the shell reads the bare `<root>` as a redirect, refuses the line as a syntax
error — ``sh: -c: line 1: syntax error near unexpected token `<'`` — and exits 2
with nothing having run. That failure is loud and whole, which is the opposite of
the one this block is built against, and it is the other half of the sentence
above: exit 1 is the no-draft branch, exit 0 is not, and exit 2 is not an answer
about drafts at all — so a caller that reads anything-but-1 as found takes an empty
line for a path. The `sh -c` wrapper is not decoration either — nothing here says
which shell this turn got, and a `for` loop written this way is not something
every shell parses.

`plan` wrote that file. Open it and check two things, and nothing else:

- `## Acceptance criteria` is there.
- Every criterion under it carries its own `fidelity required:` line.

Both true and the objective is executable. The criteria in that file are what the
rest of this turn runs against, and they are the only ones. Go to the gate.

Either one false — a `## Success criteria` heading, criteria with no fidelity on
them, no draft at all — and this objective has not been authored for execution.
Do not fill it in from here. **Invoke the `plan` skill in this same
conversation**: the Skill tool, skill `plan`, or `ai-kit:plan` where the plugin
namespace is in play. Tell it which objective you are on and what this turn needs
from it. It places the objective in the tree, applies the five-part sizing test,
writes the criteria with the fidelity each one demands, has the user read and
correct them, records the objective and links it to its parent. Then re-read the
draft it wrote and come back to this check.

The same call, for the same reason, when this turn discovered an objective that
was not there before, or when the one you are on is plainly larger than one
session. `plan` runs inside this conversation and hands back an executable
objective; nothing about the session changes, and nobody gets sent elsewhere and
told to come back.

Writing those criteria here instead would hand them to the thread that is about
to be graded on them — the same reason verification runs in a subagent that did
not write the code. Rules 3 and 4 only mean something if the criteria were
written and approved before this thread had any interest in the answer.

On a resumption, `## Acceptance criteria` is not edited. That section is what
`verify` grades, so editing it after approval would move the criteria with
nothing recording that they moved; changing a criterion means superseding the
objective instead. Extending the draft in place is for the sections around it.

### 4 — Gate

**When `plan` ran in this turn, the gate already happened.** The user read those
criteria and the fidelity each one demands in `plan`'s own gate, minutes ago, in
this conversation, and corrected them there. Do not put the same text in front of
them a second time: asking somebody to re-read what they just approved is how
they learn to skim the one artifact this entire loop depends on them reading. Say
that is why, and go straight to the freeze:

> Los criterios que acabás de aprobar en el draft son los que se congelan — no te
> los hago leer de nuevo. Arranco con eso.

**When the criteria were approved in an earlier session** and nothing has frozen
them yet, this is where rule 3's hard stop happens:

> El objetivo está en `<la ruta del draft que encontraste>`. Leelo
> entero — sobre todo los **criterios de aceptación** y la fidelidad que pide
> cada uno, porque son los únicos contra los que se va a verificar después y no
> se pueden cambiar una vez que arrancamos. Decime por chat qué cambiarías y
> decime cuando arrancamos. Si lo que cambia es un criterio, lo pasamos de vuelta
> por `plan`: desde acá no los toco.

Stop, and wait for their words. Feedback comes in chat — there is one correction
channel, because two of them is how the file and the conversation drift apart.

**Dropping a criterion is not changing one, and it does not go through `plan`.**
If the user says a criterion is unreachable, or that it stopped making sense, and
they want it gone — not easier, not reworded, gone — take it here: say back which
one in its own words, write the `decision` below, and run the objective against
what is left.

```bash
memory add --plan <tree-slug> --kind decision \
  --title "Dropped a criterion before execution: <the criterion in a few words>" \
  --body "<the criterion, quoted from the draft>

Dropped because: <the user's own reason, in their own words>" \
  --about <areas>
```

That is safe where an edit is not, and the asymmetry is the whole of why it is
allowed: **a criterion that is gone grades nothing.** A criterion that was made
easier is graded, against a bar that moved after somebody had an interest in
where it sat, and comes back green. There is no version of dropping that reports
a pass it did not earn — the count of criteria goes down, the record says who
dropped it and why, and `verify` never sees it. The freeze is a defense against
this thread grading itself, and this thread cannot drop anything: only the user
can, only in words, and only with their reason on disk.

Do it before the freeze where you can, so the `decision` step 4 writes already
holds the list the turn actually runs against. Dropped after the freeze, it is the
same record and step 8 is where it lands.

Feedback that **changes** a criterion is still a **supersede, not an edit**. The
criteria sit in the objective's own record and in the `decision` its approval
pointed at, both append-only, and neither ever moves — so changing a criterion
means writing a **new objective that supersedes this one**, not correcting the one
in front of you. That is the same mechanism step 3 names for a resumption and the
same one the resumption paragraph below names for a second freeze; this is where
it gets used.

Route it to **`plan`**, which owns criteria authorship: invoke the `plan` skill in
this same conversation, say plainly that this is a supersede, and give it the
objective you are on, the criterion, and what the user said about it. It writes
the new objective, links it over the old one and gets its criteria approved. Then
read that objective's draft and come back through step 3 against it — the
objective this turn executes is now the new one. Do not ask `plan` to edit the
criteria of the objective you came in with: the record and the freeze cannot be
rewritten, so there is nothing there for it to edit.

`work` never authors or edits a criterion — the thread that will be graded does
not draft what it is graded against, the same reason `verify` runs in a subagent
that did not write the code. Feedback about the rest of the draft you can take
here, on the terms step 3 already set.

Either way the approval is the user's and it is explicit — what changes is
whether it happened in `plan`'s gate or in this one, never whether it happened.

On approval, and only on approval, freeze it — whether `plan` authored it minutes
ago or you were already on it:

```bash
memory add --plan <tree-slug> --kind decision \
  --title "Approved for execution: <short name>" \
  --body "<the acceptance criteria as approved, each with its required fidelity>

Level: <the `## Level` line from the draft>" \
  --about <areas>
```

**The level goes in this body because what was approved includes how much of the
objective it is for.** `plan` wrote the criteria from it — the main path alone,
the paths that will realistically fail as well, or the whole thing with what it
replaces retired — so a criterion list read later without its level reads as
either too thin or too much. It is recorded here and compared against nothing:
the level decides what went into the criteria, and meeting them is iterating at
every level. Leave the line out entirely when the draft declares no `## Level` —
a parent, or any draft written before that section existed.

```bash
memory link --type approved_for_execution --from <objective-id> \
  --to <the id the add above printed, at .record.id, pasted literally>
```

That decision is the freeze as a record. The edge has to point at the record
that justifies it — `approved_for_execution` accepts a `decision` and nothing
else — so what was approved, and when, is on disk before any code is. What
verification is read against is the draft file: `verify` opens the
`draft-objective.md` step 3 found — it receives that path as `objective_draft`
rather than deriving it — and grades the criteria under `## Acceptance criteria`
there. The decision is the append-only evidence of what that section said at
approval, not a second copy that gets graded. Making the record the graded
artifact would mean recutting `verify`, which has not happened yet.

A second `approved_for_execution` on an objective that already carries one is a
**resumption, not a re-freeze**: it is how a session that stopped picks the
thread back up. The criteria live in the objective's body and in the decision
the first approval pointed at, and the log never rewrites either — so changing
criteria means superseding the objective, not approving it again. The draft file
is the mutable one, which is why step 3 leaves `## Acceptance criteria` alone on
a resumption.

`add` prints the whole record; the id is at `.record.id`. Every command prints
JSON on success and on failure alike, so check the exit code rather than
scanning the text.

Neither this pair nor the blocker pair in step 5 wraps the write in
`FREEZE=$(…)`, and that is not an oversight: an assignment of a command
substitution is one of the shapes a shell may refuse, and it refuses the whole
block at parse time before any of its lines have run, while a shell that accepts
the shape runs the same block through — and nothing here tells the turn which one
it has. The write and the edge are one run each, in that order, and nothing in this
log undoes whichever of them already went in. Read the id and paste it: one you can
read in the command is one you can see is wrong.

### 5 — Implement

**Before anything is launched, read `## Premise` in the draft.** It names the
cheapest observation that would show this objective is not worth building, and
its falsifier line says which of three states that observation is in. What it
returned when `plan` ran it: the check is paid, carry on. `the first cut itself`:
carry on too — the cut ships and the landing condition collects the answer.
`nobody checked`, or no `## Premise` at all, which is every draft written before
that section existed: also carry on, and the turn runs exactly as it did before.

The one case that stops here is a falsifier this thread can run and nobody has —
seconds of looking against an hour of implementation. Run it; rule 2 allows
reading files and spot-checking a claim, and this is that. If what comes back is
against the premise, **nothing gets launched**: put the premise and what you
observed in front of the user in one round and let them say whether the objective
still stands. That round is not one the loop pays on the ordinary path — it fires
only where the alternative was building the thing anyway and finding out at the
close.

An objective is not automatically one piece of work, and handing all of it to a
single subagent is a choice rather than the default. One long run carries the
whole objective in one context, and by the end of it the parts decided first are
the ones it no longer remembers — which is how a long objective comes back with
something quietly dropped from the middle of it. A run that owns three files
finishes before it has anything to forget.

So before launching anything, split — or decide, out loud, that there is nothing
here to split.

#### Split by what the parts write, not by how big they are

There is no length to aim at and none is given here on purpose. How short a part
should be is a property of the work in front of you: a part is one part when it
can be written without waiting on another part and without opening a file
another part is writing. That is the whole test, and it is about **conflict**,
not size.

Read the draft and name, for each candidate part, the files it is going to
write. Then:

- **Two parts that write the same file are one part.** Not two agents taking
  turns — one part. Two runs editing one file in one worktree is one of them
  silently losing its work, and nothing downstream tells you which.
- **A part that needs another part's output is not parallel with it.** A shared
  type, a schema, an interface the rest implement against: that is a *seam*, and
  a seam runs first and alone, with the rest fanning out behind it. Two waves,
  not two racing agents.
- **A part whose files you cannot name yet is not a part.** When the split
  itself depends on something nobody has read, that is the signal not to split:
  launch one and keep the objective whole.

One part is a legitimate answer and often the right one. Splitting work that
shares a surface costs more than the serial run it was meant to save, and the
cost lands as a merge nobody asked for. Say which way it went and why, in one
line, before launching:

> Lo parto en tres: el store, el handler y los tests de contrato. No se pisan —
> ninguno abre un archivo de otro.

#### Launch the parts

Every part gets the objective, not just its slice — a subagent that only knows
its ticket will satisfy the ticket and nothing else, which is how work ends up
technically complete and beside the point. Splitting multiplies that risk rather
than removing it, so the objective and the draft go to **each** part in full:

```
objective: <the parent objective's title and body, verbatim — or this one's, if it is a root>
objective_draft: <absolute path to draft-objective.md>
repo_root: <absolute path>
plan_slug: <tree-slug>
objective_id: <the executable objective's id>
constraints: <anything from memory that binds this surface — decisions, adr, wont-fix, and the gotchas step 1 loaded for this area with their anchor verdicts>
scope: <the files this part owns, and the line saying nothing outside them is its to touch — omit when the objective runs as one part>
```

`objective_draft` is the executable objective's draft file and `objective_id` its
record id. `objective` is the parent's title and body — orienting context, not
the thing being built — which is why the draft carries a name of its own.
`scope` is what turns the objective into *this* part, and it is the only field
that differs between parts.

The gotchas go in `constraints` because that is what they are: a constraint the
code obeys without stating it is exactly what a subagent about to edit that code
cannot derive and will otherwise undo. Pass each one's verdict with it — a
`gotcha` whose anchor came back `changed` is a constraint that may already have
been broken, and the part should be told which of the two it is holding. Every
part gets the constraints, not only the one whose files they cite.

**Launch every part of a wave in a single message.** Parts sent together run at
once; parts sent one message at a time run one after another and buy nothing —
that is the entire saving, and it is lost by accident rather than by decision. A
wave is one message whatever the parts are dispatched through: a Codex part's
Bash call belongs in it next to the subagent launches, not in a message of its
own. Where there is a seam, launch the seam alone, wait for it, then launch the
rest together.

#### Claude Code can send a part to the optional Codex companion

This subsection applies only when the orchestrator is Claude Code and chooses
the optional Codex executor. When the orchestrator is Codex, launch its native
`ai-kit-implement` role through the host runtime and skip this subsection; no
companion plugin is required.

A part's executor is a choice, and `implement` is the default: it is the one
that already knows this contract, reads the draft itself and writes its own
`deviation` and `gotcha` records without being told how. Codex is the other
executor, and a part goes to it when the user asks for it, or when a part came
back `blocked` and a *different* model is worth putting against that wall rather
than the same one twice.

Codex is reached through its companion runtime, from this thread's Bash. That is
not this thread writing production code — rule 2 is about who does the work, and
the work is still being done somewhere else. What this thread must not do is open
the editor itself.

Find the runtime and read the path back, rather than assuming where the plugin
landed:

```bash
ls "$HOME"/.claude/plugins/marketplaces/*/plugins/codex/scripts/codex-companion.mjs
```

Nothing printed means the Codex plugin is not installed here and `implement` is
the only executor there is. Say that in a line and carry on with it — a fallback
nobody is told about is a fallback that looks like a choice.

With the path in hand, one run per part, pasted literally:

```bash
node <the path that printed> task --write --fresh --background '<the handover, as prose, ending in the return contract>'
```

- `--write` or Codex will not edit anything. A part without it is a part that
  reads the objective and changes nothing.
- `--fresh` because each part is its own run. Resuming would hand one part the
  previous part's thread and the file boundaries that came with it.
- `--background` whenever more than one part is in flight: a foreground run
  blocks this thread until it finishes, which serializes exactly what the split
  was for. It prints a job id — `status <job-id>` says whether it is still
  running, `result <job-id>` returns its final message once it is not. A single
  Codex part with nothing running beside it can go foreground and return
  directly.

A companion-launched Codex part does not automatically receive this skill or
the `implement` contract, so **everything
`implement` gets from its own definition has to be in the prompt text**: the
objective, the draft path, this part's `scope`, the constraints, and the rules
that actually bind — stay inside the draft's scope, match the code around you,
no commits, nothing under `docs/`, no tests that only prove their own mocks
work. The `memory` commands go in the same way, with the plan slug already filled
in, when you want deviations and gotchas recorded from that part: Codex has a
shell and will run them, and it will not invent them.

End the prompt by asking for the same JSON object `implement` returns, and read
it off the **last line** of what comes back — everything above it is the
runtime's progress log. A Codex run that ends without that JSON is a part you
have no result for: treat it as `partial` and bring it to the user rather than
reconstructing one from the log.

A Codex part writes into the same worktree as the `implement` parts beside it,
so it is under the same non-conflict rule as everything else. Which executor ran
a part changes nothing about what may open which file.

#### Collect the parts into one result

The results merge before anything downstream sees them: `files_changed` is the
union across parts, and `deviations`, `assumptions`, `concerns` and `not_done`
concatenate with the part each came from still attached to it. Steps 6 and 7
receive the union, because verification grades the objective and not a slice of
it.

`test_run` does not merge, and do not try to make it. Each part ran the tests
over its own files while the others were still writing theirs, so a red one may
be reporting a part that has since landed and a green one may have run before
the file that breaks it existed. So none of them is the answer — and **this
thread does not run the suite to produce one.** The whole-suite run belongs to
the verification pass, where step 6 hands it to exactly one part, on a worktree
nobody is writing to any more.

The reason it goes there rather than here is context. A suite run in this thread
puts its output in *this* thread, the one holding the objective, and it stays
there for the rest of the session — hundreds of lines of test log still being
carried at step 9, in the context that can least afford them. Run in a
verification part, the same command produces the same answer and the log dies
with the part. What step 6 is handed from here is the merged `files_changed`;
what the tests say comes back with the verification result.

If any part returns `blocked` or `partial`, stop and bring it to the user
together with the other parts' results in one round — including what the parts
that succeeded already wrote to the worktree, which is the state the repo is
actually in. Do not chain another attempt at the same wall, and do not launch a
replacement part over files a finished part already wrote.

When the wall is external — a credential, an upstream decision, someone else's
release — record what is being waited on and say so on the objective:

```bash
memory add --plan <tree-slug> --kind open-question \
  --title "<what is being waited on>" --about <area>
```

```bash
memory link --type blocked --from <objective-id> \
  --to <the id the add above printed, at .record.id, pasted literally>
```

`blocked` points at the `open-question` or the `decision` that says what the
wait is; being held up by your own open child needs no edge, because that is
read off the tree.

### 6 — Verify

Verification runs in `verify` subagents — different ones, with no memory of
having written the code — and it splits the way step 5's work does, on a
different axis. Nothing is being written here, so conflict cannot draw the line.
What draws it is what settles a criterion.

#### Split by criteria that settle without waiting on each other

**A group of criteria is one part when it can be settled without waiting on
another part's evidence and without waiting on another part's verdict.**
Criteria that depend on each other stay in the same part, and so do criteria
that are settled by reading the same thing: splitting those pays the same
orientation twice, and it can come back with two verdicts that do not agree,
which costs more than the serial run it was meant to save.

This is **not one agent per criterion.** Nine criteria are not nine parts; they
are however many groups that sentence draws out of them. One part is a
legitimate answer and often the right one, the same way it is at step 5.

Launch the parts the way step 5's **Launch the parts** paragraph says — one
message for the wave, so they run at once. Nothing about the mechanics changes
here.

Exactly one part carries `whole_change_checks`, and it is the part whose criteria
depend on the suite. When nothing distinguishes them it goes to the first part:
the tie-break is arbitrary on purpose, and what matters is that exactly one part
owns those checks and that the merge says which one it was. When verification
runs as a single part, that part carries it.

```
objective_draft: <absolute path to draft-objective.md>
repo_root: <absolute path>
changed_files: <the union across every part step 5 launched>
criteria: <the criteria this part owns, quoted from the draft's `## Acceptance criteria`, minus any the user dropped — omit only when verification runs as one part and nothing was dropped>
whole_change_checks: <true, on exactly one part — omitted on the others>
plan_slug: <tree-slug>   objective_id: <the objective's id>
```

`changed_files` stays the union on every part rather than a slice of it. A
verifier grading one criterion still needs to know what moved, because the file
that settles a criterion is frequently one the criterion never names.

**A dropped criterion is the second reason `criteria` is not optional.** The
draft's `## Acceptance criteria` is never edited, so a criterion the user dropped
at step 4 or step 8 is still sitting in the file `verify` opens — and a run told
to grade the whole heading grades it, reports it `not-met`, and hands back a
verdict about a criterion nobody is being held to. So when anything was dropped,
pass `criteria` explicitly on every part, carrying the survivors and nothing else,
whether or not verification was going to split at all. The `decision` recording
the drop says why the list got shorter; this is what makes the run agree with it.

#### Collect the verification parts into one result

The per-criterion results concatenate, and they are disjoint by construction —
no criterion was given to two parts. **The overall verdict is recomputed here,
across all of them, and never taken from whichever part answered first**:
`ready-to-close` still means zero `not-met` **and zero `handed-over`** anywhere,
so one part's `ready-to-close` beside another's `needs-rework` merges to
`needs-rework` — and so does a part that graded everything it owned cleanly and
handed one criterion over.
`risks`, `assumptions` and `open_questions` concatenate with the part each came
from still attached to it.

**A part that comes back `blocked` makes the merged verdict `blocked`**, whatever
the others returned. That part graded nothing — it was handed an empty `criteria`,
or names matching nothing under the heading — so the absence of `not-met` in it is
the absence of any verdict at all, and a merge that counts only `not-met` would
hand step 8 a `ready-to-close` for an objective where a whole group of criteria
was never looked at. Report that part's criteria as unverified, named and counted
with the rest, rather than letting them fall out of the tally by having no verdict
to concatenate. A merged `blocked` goes to the user and the turn stops there — not
on to review, to triage or to the close, because nothing has been graded yet to
triage or close about — the same way step 5 stops the turn on a `blocked` part.

There is exactly one `test_run` and one `scope.outside_scope_and_unreported`,
both from the part that carried `whole_change_checks`, and **the merge says which
part that was.** The others returned them empty because they were told to, and a
result that does not name the runner reads as though the suite ran everywhere or
nowhere.

If no part names itself the owner, that is an error and it is raised here, not
left for whoever reads the merged result to infer from an absent name: the suite
ran zero times and the out-of-scope sweep never happened. Every criterion that
needed one of those runs as its evidence is then unverified rather than met —
nothing executed it — and it is reported that way however confident the parts
sound about it.

Read the merged result against four things:

- Any criterion reported met at a **lower fidelity than the objective required**
  is not met. Treat it as unverified regardless of how confident the report
  reads.
- A criterion whose required fidelity is `you-run` never comes back `met`. It
  comes back `handed-over`, naming the script and the instructions, because
  `verify` cannot run what belongs to the person — so it is unresolved rather
  than fallen short, it is counted apart from the unverified ones for that
  reason, and step 8 is where it gets asked about. **Left unresolved it is the
  one verdict short of `not-met` that keeps the objective from closing `met`**:
  step 9 closes it `blocked` instead, so the count travels there rather than
  being spent in this round.
- Any `unverified` criterion is named to the user explicitly, with its count.
  "Ready to close, two criteria unverified" is an honest sentence; silence
  about them is not.
- Every `assumption` and `open_question` it returns is surfaced, not absorbed.

### 7 — Review

**This step launches nothing by default.** Most changes are followable from the
diff, the walkthrough that used to justify running a review every turn is gone,
and when somebody wants a change explained there is already a door for asking —
the `ask` skill, or a plain question in this same session. A step that runs every
turn whatever the change looks like is a step that gets skimmed every turn, and
what is left of `review` deserves better: it reads the code and criticises it,
which pays exactly when nobody would otherwise look.

#### Offer it in one line, only with a reason, and only after verification

The offer is one line **inside the round that already carries verification's
result**, never a round of its own, and it is made only when you can say why
*this* turn is one where a review would pay:

- the change was split **and** either it is large or its regions are unlike
  enough that a finding crossing between them is plausible;
- a part came back `partial` or `blocked`;
- the surface is bound by a `gotcha` step 1 loaded;
- the diff is large enough that nobody is realistically going to read it.

Having been split is not a reason on its own: step 5 draws parts by files that do
not collide, so nearly every objective splits, and a reason true every turn is a
line that stops being read on the third one. With no reason, say nothing at all —
an offer whose reason you cannot name is the ceremony that turned this step into
noise, with a sentence in front of it. Do not wait on the answer as though the
offer were a gate; it sits in a round the user was already going to reply to.

> Esto salió en cuatro partes y en registros bastante distintos, así que algo
> puede cruzar entre ellas: ¿le paso una review?

Review runs once verification's result is in **and any fix that came out of it
has landed** — a review of code that has since changed is a review of code that
no longer exists, and every finding then has to be re-checked against the file as
it is now, which costs more than the review bought. If the user asks for it
earlier, say that in one line and run it after. Fixes are decided in step 8's
triage, which is also where findings go, so say where late ones land or the two
instructions wait on each other: **when verification produced fixes, review runs
after they land and its findings come back to a second triage round** — the same
step 8 over the new material, not a new step. When there was nothing to fix,
review runs as soon as the offer is answered and its findings join the first
round. A second round is the ordinary shape of a turn that had both a fix and a
review, not a sign something went wrong; the alternatives are the two failures
this rule prevents — reviewing code that is about to change, or finding something
real with no round left to put it in.

#### When it does run

When the change is one no single context saw whole, split the review along the
parts step 5 drew: those file sets are disjoint by construction — that is what
made them parts — so there is nothing here to recompute. Launch them the way step
5's **Launch the parts** paragraph says.

```
objective_draft: <absolute path to draft-objective.md>
repo_root: <absolute path>
changed_files: <the union across every part step 5 launched, plus whatever the fixes after verification touched>
scope: <the files this part reports findings on — omit when the review runs over the whole change>
```

`scope` bounds what a part reports on and never what it may read, which is why
every part gets the full changed-file list beside it: deciding whether something
in its own region duplicates what the change added somewhere else means opening
files it will not report on. Say out loud what the split costs — duplication
between two parts of the same change is the finding nobody holding one region can
see, and it is the most expensive kind a review produces. That cost is accepted
rather than covered, since there is no extra whole-diff pass behind these parts to
catch it, so when the review was split, name it to the user instead of handing
over a result that reads as complete.

The parts' findings concatenate with the part each came from still attached, and
that concatenation is the one `review_result` step 9 receives — not the first
part's answer passed through, and not a list with the provenance flattened out of
it. Where two parts raised the same thing from different sides of the change, that
is **one** finding carrying both: triage it once and say it was seen twice,
because reaching two regions says something about how far it goes and that is
signal rather than noise. A part that came back with nothing is reporting nothing
— that region's result, not a gap for another part's findings to be spread over.
What comes back is findings and nothing else; they join step 8's triage on the
same terms as verification's risks, and **an empty findings list is a real
result** — the review saying it found nothing, not the review failing to run.

### 8 — Triage, together

Put the findings, the risks, the assumptions and the unverified criteria in
front of the user in one pass, ranked by how plausible they are — and run this
step again when step 7's ordering rule sends review's findings back to it after a
fix landed. The findings may be absent, because step 7 is on demand:
verification's risks, assumptions and unverified criteria are enough for this
step to read correctly on their own, and a turn where no review ran is the
ordinary case rather than a gap to apologise for. For each, the options are: fix
now, record for later, or **not worth it** —
and when what they want kept for later is a path tried and abandoned or a place
reality differed from the objective, say it will be a `dead-end` or a
`deviation` and carry it to step 9 as `on_demand_kinds`, because the close
writes neither unless asked.

**On a `first` cut, the risks do not get a round.** That draft says the
verification of this objective is somebody using it, and ranking hypothetical
failure modes against a thing nobody has used yet is precisely the cost the cut
was made to avoid. Write verification's risks into the draft under `## Cut` as
prose, say in one line how many went there, and spend this round on what decides
whether it ships: the criteria that did not come back met, and the handover
below. On an `iteration` cut the round runs as written above — there the thing is
already in front of somebody, and a risk is a statement about real traffic.

**A criterion that stopped making sense is dropped here, by the user, exactly as
step 4 drops one.** `not-met`, `unverified` and `untestable-as-written` all carry
a reading nobody has while writing criteria: the criterion itself was wrong. Put
that beside fixing it, in these words, because the difference is the whole point:

> ¿Esto lo arreglamos, o el criterio ya no tiene sentido?

Their answer travels to step 9 in `triage_outcome` — the criterion quoted, their
reason in their own words — and `record` writes the `decision`. **You do not
decide this, and you do not lean on a criterion that merely came back hard.** A
criterion this thread talked the user out of is a criterion this thread graded
itself against, which is the single thing the freeze exists to prevent. Ask, take
the answer, and if they want it kept, it stays `not-met` and the turn goes back to
a fix.

**A criterion that stopped making sense is dropped. A premise that stopped being
true is not a drop at all.** Sometimes what the parts or the verification bring
back says the thing `## Premise` rested on is false — the field is not there, the
volume is nothing like what was quoted, the path everybody assumed exists does
not. That is not a finding to rank and not a criterion to fix: it is the
objective no longer being worth finishing. Quote the premise, say what
contradicted it, and if the user agrees, the objective closes `cancelled` at step
9 with their reason in their own words. That is the cheapest turn this loop has,
and it is cheaper the earlier it is taken — which is why step 5 reads that
section before it launches anything.

**A comment the change added does not get ranked away.** `review` reports it
high against the rule in `agents/implement.md`, and nothing it reports blocks —
`agents/review.md` says as much itself. So the user reading it here is the whole
of the enforcement: put it in front of them with the file and the line the
finding names, never folded into a count of things that were also raised. What
they decide about it is theirs, including leaving it where it is; what is not
available is the objective closing before they have seen it.

Step 7 is on demand, so on most turns there is no finding to carry, and this
step is then the only route the rule has to the user. It does not run off a diff
this thread is holding: what the collect step merged back was `files_changed`
and prose, and keeping the change itself out of this context was the point of
merging it that way. So the diff is fetched here, narrowed to the question being
asked:

```bash
git diff --unified=0 -- <the changed files>
```

`--unified=0` drops the context lines and the path list drops every file the
change did not touch, which is what keeps this from reinstating the whole-diff
read the collect step declined. Read the added lines against rule 3 of
`agents/implement.md` — the rule and the exemptions it closes over live there,
and they are read there rather than restated here, because a second copy drifts
from the first and the lines a drifted copy raises are the ones `implement` was
required to write. An added line behind a comment marker that rule 3 does not
exempt goes in front of the user with its file; an exempted one is not a finding
and is not mentioned. Carry nothing else forward — what else printed is not
summarized here and does not travel to step 9.

A file the command prints nothing for is one the change created and git has not
been told about yet. Its added lines are the whole file, so read it and apply
the same reading: a new file is where this rule is easiest to break and the one
place a diff-shaped look reliably misses it.

**A `you-run` criterion is asked about, never assumed.** Step 6 hands it over
unresolved by construction, so it gets named here: the criterion, the script and
the instructions `implement` left in `.ai-kit/plans/<slug>/manual-checks.md`,
and the question of what happened when they ran it. Their answer travels to step
9 in `triage_outcome` in their own words, which is what puts it in the closing
record — this thread writes nothing about it itself.

**No answer is not a pass, and this is where that stops being a sentiment.** The
criterion is still `handed-over`, nobody has run it, and the objective does not
close `met` on it. There are exactly three ways out, and the user picks one in
words, in this round:

- **They ran it and it passed.** The criterion is met. Carry what they observed
  to step 9 in their own words.
- **They ran it and it failed.** That is a `not-met` criterion, and the turn goes
  back to a fix rather than on to the close.
- **It is not going to be run.** That is a decision, and it is kept as a
  `wont-fix` in their own words like any other "not worth it" — and once that is
  on disk the criterion stops holding the objective, because what closes it is
  now a choice somebody made instead of a check nobody ran.

Silence is none of the three, and it is no longer this thread's to resolve by
moving on. It leaves the criterion `handed-over`, and step 9 closes the objective
`blocked` on the open question naming the unrun check — the objective staying
open and the ask coming back at the top of the next turn, which is what step 1's
ledger is for. Seven of these in a row closed `met` because the default in this
round was to carry on; the default is now that the objective waits.

Take "not worth it" seriously. A verification pass that keeps raising scenarios
the user knows cannot happen burns time and trains everyone to skim the
findings. When they say a risk is not real — the gateway serializes it, the
data can't be negative here, we'll tell the team instead — that is a decision
worth keeping, and it gets kept as a `wont-fix`.

This conversation happens here, with the user, and it does not move. What moves
is the writing: the `wont-fix` records are written in step 9 by the `record`
subagent, along with everything else this close puts on disk. So while you are
here, **keep the user's own words** for anything they call not worth fixing —
their reason is the body of that record, and a reason you paraphrased after the
fact is worth much less to the session that finds it.

Before raising anything in a future turn, check `--kind wont-fix` for that
area. Re-raising something already dismissed is exactly the waste this record
exists to prevent.

### 9 — Close the objective, then look up at its parent

Hand the close to the `record` subagent. It writes every record and every edge
this close needs — including the closing `review-finding` and the state edge —
and **this thread writes none of them.** You have been holding the objective, the
diff and verification's report since the session started, and the tail of that
turn is the worst place left to be composing titles and choosing `--about` tags.
That is the whole reason this step delegates.

Before the handoff, read what this objective has left open:

```bash
memory state --id <objective-id>
```

Inside that objective's entry, beside `completion`, is `open_records`: the
`open-question` and `assumption` records that derive from this objective and that
nothing has answered yet, each with its id, kind and title. It is this
objective's own records, not the plan's — a handful, where a whole tree
accumulates hundreds — and the field is absent entirely when there are none,
which is what a clean close looks like rather than a command that failed.

It is read here, and not only at the read-back below, because this is the last
moment the turn can still settle what it settled. Put the list in front of the
user, one line each, and say for each whether this turn spoke to it. Their answer
travels to the subagent in `triage_outcome` in their own words, with the record's
id, the same way step 8's triage does: answering is an appended edge from the
record that settles the question, so `record` is what writes it and it can only
write one for something it was told about. **Nothing gets labelled answered
here** — an assumption the user did not speak to is still an assumption, and the
list saying so is what it is for.

**Nothing blocks on this list.** A question that stays open is an ordinary close
and a long list is not a reason to hold the objective back. What the list changes
is that the turn saw it while it could still do something about it, instead of
finding out afterwards from a subagent's account of its own work.

Then hand off:

```
objective_draft: <absolute path to draft-objective.md>
repo_root: <absolute path>
objective_id: <the executable objective's id>
plan_slug: <tree-slug>
verify_result: <the verify subagent's returned JSON>
review_result: <the review subagent's returned JSON — empty when step 7 did not run>
triage_outcome: <per finding: fix now / record for later / not worth it, with the user's own words on anything they called not worth fixing and on how any you-run criterion went — said as unanswered when they did not answer; each criterion the user dropped, quoted from the draft, with their reason in their own words; and each open record above that this turn settled, with its id and what settled it>
closing_state: <met | blocked | cancelled | separated>
confirmed_hypothesis:
on_demand_kinds:
existing_closing_record:
missing:
```

Pass pointers, not prose. `objective_draft` and `repo_root` are paths because the
subagent opens them itself, and there is deliberately **no field for a summary of
what happened this turn** — that synthesis is the expensive thing being moved, and
writing it here to hand over would mean paying for it anyway.

`review_result` comes over empty on every turn step 7 did not run, which is most
of them, and **empty is a value there rather than a gap.** The subagent builds
the closing record out of what it was given; nothing is synthesised to stand in
for a review nobody asked for. Pass it empty and say nothing about it — a note
explaining that no review ran is a sentence where a result goes, which reads to
the subagent as a result.

`confirmed_hypothesis` is filled **only** when this objective came out of a
`debug` investigation, and then with the hypothesis record's id and its claim —
nothing else, and never a note explaining that `debug` did not run. Read both off
the log rather than off your memory of the conversation: `plan` linked the two with
a `caused_by` edge, and two calls turn that edge into the id and the claim.

```bash
memory edges --from <objective-id> --type caused_by
memory query --id <the id in the to field of that edge, pasted literally> --fields id,kind,title,body --body-lines 1
```

`edges` returns edges and nothing else — no record at either end, no bodies — and
what bounds it is `--from`, `--to` and `--type`. Not the plan: `--plan` on it is
refused outright with `plan_not_on_edges`, and an `AI_KIT_PLAN` in the environment
is ignored rather than applied, so the answer spans the whole log and `--from` is
the only thing keeping it to this objective. A read that walked out from the id
instead would come back with the full record of every node it reached, in either
direction and at any edge type, when what this step needs is one id out of one
edge.

What the subagent needs the claim for is deciding whether the lesson generalizes
past this bug, which is the one judgment at the close that cannot be reconstructed
from the diff.

Leave it blank on every other objective. Blank is what says "no investigation
behind this", and the subagent writes no postmortem — which for work that started
from a request is correct, because there is nothing to derive one from.

The last four carry **nothing after the colon on purpose**, and copying them
exactly like that is correct. On the ordinary path their right value is no value:
`confirmed_hypothesis` is empty unless `debug` ran, `on_demand_kinds` is empty
unless step 8 produced one, and `existing_closing_record` and `missing` belong to
the relaunch at the end of this step. They are in the block anyway, because a
field the subagent declares and this thread never passes is exactly the parameter
drift that gets discovered the one time it matters — but the subagent branches on
whether they are empty, so a description of the field substituted where the value
goes reads to it as a value. An `existing_closing_record` holding a sentence about
being empty is how a `met` edge gets pointed at a sentence, and a
`confirmed_hypothesis` holding one is how a postmortem gets derived from an
investigation that never happened.

What it records by default is three kinds: `decision`, `assumption`,
`open-question`. Those are the three that anything ever reads — step 1's
carried-in round reads the last two, and `implement` receives decisions as
`constraints`. On top of that it writes what this turn produced rather than
synthesised: the `wont-fix` records from step 8's triage, in the user's own words;
an `answers` edge for anything the turn actually settled; and the closing record
with the state edge on it. When `confirmed_hypothesis` is set it also weighs one
`postmortem`, derived from that hypothesis and written only if the lesson
generalizes beyond this bug — a bug whose lesson does not closes without one. An
`adr` joins conditionally in the same way, on a test the subagent states and
applies to each choice it was going to file as a `decision`.

Let it make that call. Whether a lesson travels is a judgment about the artifacts
this thread stopped being the cheap place to read three steps ago, and asking for
a postmortem per fix is how the promotion pass at step 10 ends up offering a
folder nobody wants.

`deviation` and `dead-end` are still valid kinds in `memory`, available **on
demand**: name one in `on_demand_kinds` when the user asks for it. Nothing at the
close prompts for them.

Closing is an edge, never an edit, and it points at the record that justifies it.
Which state it is remains yours to decide with the user and the subagent's to
write, which is what `closing_state` carries: `met` is the state that counts
toward the parent; an objective that is not going to happen closes `cancelled`,
and one whose work continues outside this parent `separated`. Those two point at
a `decision` instead, and neither counts toward the parent's completion — and none
of these is reached on a turn whose verification came back `blocked`, which is
`verify` reporting that it graded nothing at all and which step 6 stops before
this step. That is a verdict and not a state, and it is not the `blocked` below.

**`met` is unavailable while anything this objective still owes is unrun.** Two
things put it there, and both resolve the same way:

- a criterion still `handed-over` after step 8's round, and
- a `## Landing condition` above `worktree` that has not happened yet.

Either one and `closing_state` is `blocked`, pointing at the `open-question` that
names what is unrun — the same pair step 5 already writes for an external wall,
and `record` already writes that question for a handover nobody reported on.

`blocked` is an **open** state in this log rather than a closing one, which is
what makes this a pause and not a failure: the objective keeps its approval, it
stays a candidate for `--current`, `approved_for_execution → blocked → met` is a
sequence the log resolves to `met`, and `memory state --text` lists it under
**Blocked on a question or a decision** grouped by the record it waits on. That
listing is the ledger step 1 reads at the top of the next turn, which is when this
gets paid. Nothing in the CLI changes for any of it.

Say it in one line, as a pause rather than an apology, and **hand over the
command that closes it**:

> El objetivo queda `blocked` esperando `<el chequeo o el aterrizaje>`. El código
> está hecho y verificado en todo lo demás. Cuando lo corras o lo despliegues y
> esté bien, cerralo vos — no hace falta abrir una sesión para esto:
>
> ```bash
> memory link --type met --from <objective-id> --to <closing-record-id>
> ```

**When `## Premise` names the cut itself as its falsifier, that landing is the
premise's verdict and not only the criterion's**, so ask for the answer that
matters in the same breath:

> Y si cuando lo despliegues resulta que la premisa era falsa — que no le sirve a
> nadie, que no era esto lo que hacía falta — no lo cierres `met`. Decímelo y lo
> cerramos `cancelled`, que es el resultado más barato que puede tener esto.

Both ids are in your hand already: the objective's, and the closing
`review-finding` the subagent just wrote. **Paste them in literally.** A command
with angle brackets left in it is a command somebody opens a session to get
filled in, which is the entire cost this branch exists to remove.

**Reopening a session to say "it worked" is waste, and this is what removes it.**
The debt is real and it holds the objective open, but paying it is one append to
the log by the person who already ran the check — not a turn that reloads the
tree, re-reads the draft, re-reads the log and spends a whole context to write a
single edge. Step 1's ledger is for the checks they never got to, not for the
ones they did.

#### Confirm the close landed, do not trust the report

```bash
memory state --id <objective-id>
```

Read the objective's `state` out of what comes back rather than out of the
subagent's report. A `link` that refused inside a subagent still printed JSON and
still exited, so "closed" in a report is a claim and this is the evidence.

- It reads the state you asked for — `met`, `blocked`, `cancelled` or
  `separated`. The close landed. `blocked` landing is the close landing: the
  objective is open on purpose and waiting on a named record, which is a
  different thing from the edge never having been written. Before step 10, give the user the subagent's `failures` and
  `not_recorded` in one line: a state edge that landed says the objective closed,
  not that everything this turn agreed to write got written. Anything the user
  triaged **record for later** that comes back in `not_recorded` is a
  disagreement to put to them, not a call the subagent got to make — they were
  offered that option and are owed the news that it did not happen. Nothing in
  either list and one line says so.
- It still reads `approved_for_execution`, or anything else from before the
  close — including `blocked` when that is not the state you asked for:
  **the close did not land.** The objective is open whatever the report
  said. Say so to the user in one line, show the subagent's `failures`, and
  relaunch `record` with the same handover — `missing` naming exactly what did
  not land, and `existing_closing_record` carrying the closing record's id when
  one already exists, so nothing is written twice. Do not write the edge from
  this thread. Do not report the objective closed, and do not move to step 10 as
  though it were. A second failure is a stop: bring it to the user with the CLI's
  own error, because an objective that reads closed in the conversation and open
  in the log is the one failure this check exists to catch.

**The same output also says what stayed open, and that gets said in words.** Read
`open_records` a second time, out of what this command just returned. Anything
the turn settled is gone from it — `record` appended the `answers` edge and an
answered record drops off the list — so what is left is what this objective is
leaving behind, and the field being absent altogether means it left nothing. It
is not the list you read before the handoff and it is not meant to be shorter:
the questions and assumptions `record` just wrote for this close derive from this
objective too, so they are on it. Tell the user which: how many, and each one's
title on a line. That is the difference between what stayed open being read out
of the log and it being inferred from `failures` and `not_recorded`, which are
the subagent's account of its own work.
A record still on the list that the user believes this turn settled is worth
saying out loud right here, while the conversation that settled it is still in
the room. It refuses nothing — the objective is closed either way, and this is
the account of what it closed with.

Then, before anything else is chosen, look up at the parent:

```bash
memory state --id <parent-objective-id>
```

Report two things to the user, in this order. First the **parent's success
criteria**, from its record body — step 1 already loaded it. Then its three
completion buckets exactly as the command returns them: `counted_done`,
`not_counted`, `outstanding`. The parent's criteria come first because a child
that closed is only worth something as a step toward the objective above it, and
this is the moment to notice that the parent is done, or that what it still
needs is something nobody has opened yet.

Report the buckets as they come back, without tidying them. `state` also returns
`blocked_by_open_child`, derived from the tree, and the two can disagree: a
child can be off the blocking list and still sit in `outstanding`. Show both and
ask the user. A disagreement there is a real question about what counts as done,
not something to settle on the objective's behalf.

A root objective has no parent to look up. Point `--id` at the objective you just
closed instead and report its own tally, so the turn still ends by saying what is
left rather than what was finished.

Then `memory validate` to confirm the log is still sound.

### 10 — Decide what comes next

Say, in a few lines: what the objective above still needs, and what the obvious
next objective under it looks like. Then ask whether to keep going here or stop.

Keep going in this session when the context still has room and the next one is
small. Stop when the context is filling — a session that pushes on past that
point starts forgetting the objective, and everything after that is worse than
what came before it.

Work you discovered mid-flight that is larger than this objective assumed does
not evaporate at the end of the turn. When somebody is going to pick it up, open
it through `plan` — the skill invocation from step 3, in this session if there is
context left for it, otherwise named in the handoff prompt below so the next
session starts there. When nobody is going to pick it up soon, leave it as prose
in the parent's draft. A record is for work someone will do; prose is for the
rest.

When stopping, hand over a prompt the user can paste into a fresh session:

```
Continuá el árbol `<tree-slug>` en <repo>. Corré el skill `work`: estábamos en
`<objective title>`; lo último que cerró fue `<name>`; lo que sigue es <one line>.
```

If the parent's success criteria are all met — the tally in step 9 is what tells
you — say so plainly and offer the promotion pass: which decisions, ADRs and
postmortems are worth lifting out of this repo's log to the user level, where
they outlive it (`memory promote --ids ... --to user`). That choice is the
user's — suggest candidates, do not promote on your own.

## Key principles

- **The objective above is the thing being served.** The one you execute is only
  a means, and an objective that cannot say how it serves the one it derives
  from is not ready to run.
- **Opening a child is not abandoning the parent.** One record, one edge, and
  the parent is still there — which is what makes "this is bigger than one
  session" a cheap thing to discover.
- **Nothing is rewritten to change state.** Every transition is an appended edge
  pointing at the record that justifies it, so the log stays a true trace rather
  than a current status.
- **Criteria are frozen before there is any incentive to move them.** That is
  the entire defense against verification that grades its own work.
- **Required fidelity is agreed up front.** A mock satisfying a criterion about
  a real database is not a passing check, it is a false one. `you-run` is agreed
  the same way and says something else: the check belongs to the person, so it
  comes back handed over rather than met, and the close asks them how it went.
- **An unrun check holds the objective open.** A fidelity nobody ever pays is not
  a fidelity, it is a way of approving without evidence — so `handed-over` and an
  unmet landing condition close the objective `blocked` rather than `met`, and the
  next turn opens by naming what is owed. Waiting is not failing, and the debt is
  cleared by running the check or by deciding in words that it never will be.
- **Parts are drawn by what would collide, not by size.** Implementation splits
  by the files parts write, verification by criteria that settle without waiting
  on another part's evidence or verdict, review along the parts implementation
  already drew — three axes, one rule. Two runs that cannot get in each other's
  way go at once whatever their length; two that can are one part however small
  each looked.
- **The one who built it does not get to bless it.**
- **"Not worth fixing" is a real answer, and it gets recorded.** Otherwise the
  same imaginary risk comes back every turn.
- **Stop before the context fills, not after.** Splitting an objective is cheap;
  losing the thread halfway through one is not.
