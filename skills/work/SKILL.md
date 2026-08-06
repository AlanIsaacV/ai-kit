---
name: work
description: "Execute one objective end to end: resolve which objective you are on, confirm it is executable — its acceptance criteria each carrying the fidelity they require — freeze those criteria, implement it, verify it against them in a fresh subagent, review the code with the user, record what was learned, and decide what comes next. Use whenever an objective is already recorded and the user wants to move it forward. Triggers on: 'sigamos', 'continuemos', 'seguí con el plan', 'arrancá', 'dale', 'implementá la siguiente parte', 'retomemos', 'continue where we left off', 'what's next', 'terminá lo que quedó', or the start of a fresh session on work already planned. This skill executes and does not author: when no objective exists yet, when the one you are on is too big for a session, or when this turn discovers work nobody has framed, it invokes the `plan` skill to write that objective and get its criteria approved, then comes back here to run it."
---

# work

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
2. **Never write production code in this thread.** That is the `implement`
   subagent's job. Reading a few files to check drift or spot-check a claim is
   fine; editing project source is not.
3. **The user approves the executable objective before anything is built.** Hard
   stop, every time. Silence is not approval.
4. **Acceptance criteria are frozen at approval.** Nothing downstream may add,
   soften or reinterpret them. A condition discovered later is an
   `open-question` for the user, never a criterion the system grades itself
   against.
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

```bash
MEMORY="${CLAUDE_PLUGIN_ROOT}/bin/memory"
[ -x "$MEMORY" ] || ( cd "${CLAUDE_PLUGIN_ROOT}/tools/memory" && go build -o "$MEMORY" ./cmd/memory )
```

Queries report their match count before handing over records, and withhold
them above a threshold. When that happens, **narrow the query** — do not raise
the limit to make the wall of text appear. If you genuinely need to read a lot
of history, that is a job for a subagent, not for this context.

`$MEMORY kinds` is the authority on which record kinds and edge types exist and
on which record each state edge is allowed to point at; `$MEMORY --help` on
which flags exist. A command this file does not show is one of those two reads
away.

## Steps

### 1 — Resolve which objective you are on, load only the history that bears on this turn, and put it on the table

```bash
$MEMORY query --plan <tree-slug> --kind objective --current
$MEMORY state
$MEMORY query --plan <tree-slug> --kind open-question,assumption --current
```

That is the whole start-up read, and it is bounded by the tree's shape rather
than its age. Pull anything else on demand and by area — `--about <area>` for
the surface this turn will touch, `--kind wont-fix` before proposing work that
someone already decided was not worth doing.

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

Drafts live at `plans/<objective-slug>/draft-objective.md`: one directory per
objective. Take that slug from the objective record step 1 loaded — its own
`slug` field, which `state` and `query` both report, not the `origin.plan` that
names the tree — rather than deriving one from the title yourself. `plan` named
the directory after that same field when it recorded the objective, and a
derived name that lands one character off finds no draft and sends this step down
the no-draft branch for an objective that has one. `plan` wrote that file. Open
it and check two things, and nothing else:

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

> El objetivo está en `plans/<objective-slug>/draft-objective.md`. Leelo
> entero — sobre todo los **criterios de aceptación** y la fidelidad que pide
> cada uno, porque son los únicos contra los que se va a verificar después y no
> se pueden cambiar una vez que arrancamos. Decime por chat qué cambiarías y
> decime cuando arrancamos. Si lo que cambia es un criterio, lo pasamos de vuelta
> por `plan`: desde acá no los toco.

Stop, and wait for their words. Feedback comes in chat — there is one correction
channel, because two of them is how the file and the conversation drift apart.

Feedback that touches an acceptance criterion is a **supersede, not an edit**. The
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
FREEZE=$($MEMORY add --plan <tree-slug> --kind decision \
  --title "Approved for execution: <short name>" \
  --body "<the acceptance criteria as approved, each with its required fidelity>" \
  --about <areas> \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["record"]["id"])')

$MEMORY link --type approved_for_execution --from <objective-id> --to "$FREEZE"
```

That decision is the freeze as a record. The edge has to point at the record
that justifies it — `approved_for_execution` accepts a `decision` and nothing
else — so what was approved, and when, is on disk before any code is. What
verification is read against is the draft file: `verify` opens
`plans/<objective-slug>/draft-objective.md` and grades the criteria under
`## Acceptance criteria` there. The decision is the append-only evidence of what
that section said at approval, not a second copy that gets graded. Making the
record the graded artifact would mean recutting `verify`, which has not happened
yet.

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

### 5 — Implement

Launch the `implement` subagent. It gets the objective, not just the task — a
subagent that only knows its ticket will satisfy the ticket and nothing else,
which is how work ends up technically complete and beside the point.

```
objective: <the parent objective's title and body, verbatim — or this one's, if it is a root>
objective_draft: <absolute path to draft-objective.md>
repo_root: <absolute path>
memory_cmd: <absolute path to the memory binary>
plan_slug: <tree-slug>
objective_id: <the executable objective's id>
constraints: <anything from memory that binds this surface — decisions, wont-fix>
```

`objective_draft` is the executable objective's draft file and `objective_id` its
record id. `objective` is the parent's title and body — orienting context, not
the thing being built — which is why the draft carries a name of its own.

When it returns: if it is `blocked` or `partial`, stop and bring it to the
user. Do not chain another attempt at the same wall.

When the wall is external — a credential, an upstream decision, someone else's
release — record what is being waited on and say so on the objective:

```bash
BLOCKER=$($MEMORY add --plan <tree-slug> --kind open-question \
  --title "<what is being waited on>" --about <area> \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["record"]["id"])')

$MEMORY link --type blocked --from <objective-id> --to "$BLOCKER"
```

`blocked` points at the `open-question` or the `decision` that says what the
wait is; being held up by your own open child needs no edge, because that is
read off the tree.

### 6 — Verify

Launch the `verify` subagent — a different one, with no memory of having
written the code:

```
objective_draft: <absolute path to draft-objective.md>
repo_root: <absolute path>
changed_files: <from the implement result>
memory_cmd: <path>   plan_slug: <tree-slug>   objective_id: <the objective's id>
```

Read its result against three things:

- Any criterion reported met at a **lower fidelity than the objective required**
  is not met. Treat it as unverified regardless of how confident the report
  reads.
- Any `unverified` criterion is named to the user explicitly, with its count.
  "Ready to close, two criteria unverified" is an honest sentence; silence
  about them is not.
- Every `assumption` and `open_question` it returns is surfaced, not absorbed.

### 7 — Review

Launch the `review` subagent while the diff is still small. This is the step
that stops you from arriving at the end of the objective without knowing how
any of it works.

```
objective_draft: <absolute path to draft-objective.md>
repo_root: <absolute path>
changed_files: <from the implement result>
```

Give the user its walkthrough more or less as written. It is meant to be read
in a few minutes and to leave them able to navigate the code themselves.

### 8 — Triage, together

Put the findings, the risks, the assumptions and the unverified criteria in
front of the user in one pass, ranked by how plausible they are. For each, the
options are: fix now, record for later, or **not worth it** — and when what they
want kept for later is a path tried and abandoned or a place reality differed
from the objective, say it will be a `dead-end` or a `deviation` and carry it to
step 9 as `on_demand_kinds`, because the close writes neither unless asked.

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
diff, verification's report and review's walkthrough since the session started,
and the tail of that turn is the worst place left to be composing titles and
choosing `--about` tags. That is the whole reason this step delegates.

```
objective_draft: <absolute path to draft-objective.md>
repo_root: <absolute path>
objective_id: <the executable objective's id>
plan_slug: <tree-slug>
memory_cmd: <absolute path to the memory binary>
verify_result: <the verify subagent's returned JSON>
review_result: <the review subagent's returned JSON>
triage_outcome: <per finding: fix now / record for later / not worth it, with the user's own words on anything they called not worth fixing>
closing_state: <met | cancelled | separated>
confirmed_hypothesis:
on_demand_kinds:
existing_closing_record:
missing:
```

Pass pointers, not prose. `objective_draft` and `repo_root` are paths because the
subagent opens them itself, and there is deliberately **no field for a summary of
what happened this turn** — that synthesis is the expensive thing being moved, and
writing it here to hand over would mean paying for it anyway.

`confirmed_hypothesis` is filled **only** when this objective came out of a
`debug` investigation, and then with the hypothesis record's id and its claim —
nothing else, and never a note explaining that `debug` did not run. Read the id
off the objective's own record rather than off your memory of the conversation:
`plan` linked the two with a `caused_by` edge, and `$MEMORY graph --from
<objective-id> --depth 1` shows it. What the subagent needs the claim for is
deciding whether the lesson generalizes past this bug, which is the one judgment
at the close that cannot be reconstructed from the diff.

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
generalizes beyond this bug — a bug whose lesson does not closes without one.

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
a `decision` instead, and neither counts toward the parent's completion.

#### Confirm the close landed, do not trust the report

```bash
$MEMORY state --id <objective-id>
```

Read the objective's `state` out of what comes back rather than out of the
subagent's report. A `link` that refused inside a subagent still printed JSON and
still exited, so "closed" in a report is a claim and this is the evidence.

- It reads the state you asked for — `met`, `cancelled` or `separated`. The close
  landed. Before step 10, give the user the subagent's `failures` and
  `not_recorded` in one line: a state edge that landed says the objective closed,
  not that everything this turn agreed to write got written. Anything the user
  triaged **record for later** that comes back in `not_recorded` is a
  disagreement to put to them, not a call the subagent got to make — they were
  offered that option and are owed the news that it did not happen. Nothing in
  either list and one line says so.
- It still reads `approved_for_execution`, or anything else from before the
  close: **the close did not land.** The objective is open whatever the report
  said. Say so to the user in one line, show the subagent's `failures`, and
  relaunch `record` with the same handover — `missing` naming exactly what did
  not land, and `existing_closing_record` carrying the closing record's id when
  one already exists, so nothing is written twice. Do not write the edge from
  this thread. Do not report the objective closed, and do not move to step 10 as
  though it were. A second failure is a stop: bring it to the user with the CLI's
  own error, because an objective that reads closed in the conversation and open
  in the log is the one failure this check exists to catch.

Then, before anything else is chosen, look up at the parent:

```bash
$MEMORY state --id <parent-objective-id>
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

Then `$MEMORY validate` to confirm the log is still sound.

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
they outlive it (`$MEMORY promote --ids ... --to user`). That choice is the
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
  a real database is not a passing check, it is a false one.
- **The one who built it does not get to bless it.**
- **"Not worth fixing" is a real answer, and it gets recorded.** Otherwise the
  same imaginary risk comes back every turn.
- **Stop before the context fills, not after.** Splitting an objective is cheap;
  losing the thread halfway through one is not.
