---
name: work
description: "Run one unit of work against an existing objective, end to end: define what this turn delivers, get it approved, implement it, verify it against the approved criteria, review the code with the user, record what was learned, and decide what comes next. Use whenever there is already an objective recorded and the user wants to move it forward. Triggers on: 'sigamos', 'continuemos', 'next unit', 'seguí con el plan', 'arrancá', 'dale', 'implementá la siguiente parte', 'retomemos', 'continue where we left off', 'what's next', or the start of a fresh session on work already planned. If no objective exists yet, use the `plan` skill first — do not invent one to have something to point at."
---

# work

One turn of the loop: define a unit, build it, check it honestly, learn from
it, decide what is next.

You are the orchestrator. You hold the objective, you talk to the user, and you
delegate the actual work. You do not write production code and you do not
verify your own criteria — you defined them, which is exactly why you are the
wrong one to judge whether they were met.

## Hard rules

1. **The objective is loaded first and stays loaded.** Every unit answers "how
   does this move the objective". A unit that cannot answer it is a unit that
   should not run.
2. **Never write production code in this thread.** That is the `implement`
   subagent's job. Reading a few files to check drift or spot-check a claim is
   fine; editing project source is not.
3. **The user approves the unit before anything is built.** Hard stop, every
   time. Silence is not approval.
4. **Acceptance criteria are frozen at approval.** Nothing downstream may add,
   soften or reinterpret them. A condition discovered later is an
   `open-question` for the user, never a criterion the system grades itself
   against.
5. **Verification runs in a different subagent than implementation**, with
   fresh context.
6. **State changes go through the `memory` CLI.** Never hand-edit the log.
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

## Steps

### 1 — Load the objective and only the history that bears on this turn

```bash
$MEMORY query --plan <slug> --kind objective --current
$MEMORY query --plan <slug> --kind unit --limit 3          # what the last turns delivered
$MEMORY query --plan <slug> --kind open-question,assumption --current
```

That is the whole start-up read, and it is bounded by the plan's shape rather
than its age. Pull anything else on demand and by area — `--about <area>` for
the surface this turn will touch, `--kind wont-fix` before proposing work that
someone already decided was not worth doing.

Then look at reality, not just the record: `git log --oneline -15` and the
state of the files the objective's orientation named. **The objective was
written at a point in time and the repo has moved.** If something that the plan
assumed is no longer true, surface it now, before defining a unit on top of it.
Finding drift is a good outcome, not a delay.

### 2 — Choose what this unit delivers

Pick from what the objective still needs, given what exists today. The
criteria a unit has to meet:

- **It delivers something demonstrable.** At the end you can show it working.
  Not "the layer is refactored" — "you can cancel an order and see the reason
  recorded".
- **It fits one session with room to spare.** Context that fills up is context
  that starts forgetting the objective, which is the failure this whole shape
  exists to avoid. When in doubt, split.
- **It is worth doing on its own.** If it only makes sense once a later unit
  lands, either merge them or do the later one first.
- **It names what it does not cover.** Two or three non-goals. This is the
  cheapest scope control there is.
- **It says how it serves the objective**, in one line. If that line is hard to
  write, that is the signal — ask the user before going further.

Do not derive the unit from a pre-existing phase list. There isn't one, on
purpose: you are choosing against the current state of the repo, which is
better information than anything written weeks ago.

### 3 — Write the unit draft

Write `plans/<slug>/draft-unit.md`. Overwrite the previous one; the closed
units live in memory.

```markdown
# Unit: <short name>

## Deliverable
<What will be demonstrably working when this is done. One paragraph.>

## How this serves the objective
<One line.>

## Acceptance criteria
1. <Observable statement that can fail.>
   - **Verified by**: <the specific check> — **fidelity required**: real-dependency | integration | mocked | code-read
2. ...

## Out of scope
- <what this unit does not touch>

## Files and areas expected
<Best current understanding. Not a contract — deviations get recorded.>

## Open questions carried in
- <anything from memory still unanswered that this unit touches>
```

**The fidelity line on each criterion is the load-bearing part.** It is decided
here, with the user, before anyone has an interest in the answer. A criterion
about behavior against a database, a queue or a network says
`real-dependency`, and that is what makes it impossible for verification to
later report it satisfied on the strength of a mock. Choosing `mocked` is
allowed — it just has to be a choice made in the open, now, rather than a
convenience discovered later.

### 4 — Gate

> Escribí la unidad en `plans/<slug>/draft-unit.md`. Leela entera — sobre todo
> los **criterios de aceptación** y la fidelidad que pide cada uno, porque son
> los únicos contra los que se va a verificar después y no se pueden cambiar
> una vez que arrancamos. Corregí lo que quieras en el archivo y decime cuando
> arrancamos.

Stop. Re-read the file when they come back — their edits are the decision.

On approval, record the unit and link it to the objective:

```bash
UNIT=$($MEMORY add --plan <slug> --kind unit --title "<short name>" \
  --body "<deliverable + acceptance criteria + out of scope>" --about <areas> \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["record"]["id"])')

$MEMORY link --plan <slug> --type implements --from "$UNIT" --to <objective-id>
```

`add` prints the whole record; the id is at `.record.id`. Every command prints
JSON on success and on failure alike, so check the exit code rather than
scanning the text.

### 5 — Implement

Launch the `implement` subagent. It gets the objective, not just the task — a
subagent that only knows its ticket will satisfy the ticket and nothing else,
which is how work ends up technically complete and beside the point.

```
objective: <the objective's title and body, verbatim>
unit: <absolute path to draft-unit.md>
repo_root: <absolute path>
memory_cmd: <absolute path to the memory binary>
plan_slug: <slug>
unit_id: <the id from step 4>
constraints: <anything from memory that binds this surface — decisions, wont-fix>
```

When it returns: if it is `blocked` or `partial`, stop and bring it to the
user. Do not chain another attempt at the same wall.

### 6 — Verify

Launch the `verify` subagent — a different one, with no memory of having
written the code:

```
unit: <absolute path to draft-unit.md>
repo_root: <absolute path>
changed_files: <from the implement result>
memory_cmd: <path>   plan_slug: <slug>   unit_id: <id>
```

Read its result against three things:

- Any criterion reported met at a **lower fidelity than the unit required** is
  not met. Treat it as unverified regardless of how confident the report reads.
- Any `unverified` criterion is named to the user explicitly, with its count.
  "Ready to close, two criteria unverified" is an honest sentence; silence
  about them is not.
- Every `assumption` and `open_question` it returns is surfaced, not absorbed.

### 7 — Review

Launch the `review` subagent while the diff is still small. This is the step
that stops you from arriving at the end of the objective without knowing how
any of it works.

```
unit: <absolute path to draft-unit.md>
repo_root: <absolute path>
changed_files: <from the implement result>
```

Give the user its walkthrough more or less as written. It is meant to be read
in a few minutes and to leave them able to navigate the code themselves.

### 8 — Triage, together

Put the findings, the risks, the assumptions and the unverified criteria in
front of the user in one pass, ranked by how plausible they are. For each, the
options are: fix now, record for later, or **not worth it**.

Take "not worth it" seriously. A verification pass that keeps raising scenarios
the user knows cannot happen burns time and trains everyone to skim the
findings. When they say a risk is not real — the gateway serializes it, the
data can't be negative here, we'll tell the team instead — that is a decision
worth keeping:

```bash
$MEMORY add --plan <slug> --kind wont-fix \
  --title "<the risk, stated plainly>" \
  --body "<the user's reason>" --about <area>
```

Before raising anything in a future turn, check `--kind wont-fix` for that
area. Re-raising something already dismissed is exactly the waste this record
exists to prevent.

### 9 — Close the unit

Record what this turn actually produced:

```bash
$MEMORY add --plan <slug> --kind decision  --title "..." --about <area>   # choices made mid-flight
$MEMORY add --plan <slug> --kind deviation --title "..." --about <area>   # where reality differed from the unit
$MEMORY add --plan <slug> --kind assumption --title "..." --about <area>  # what had to be assumed
$MEMORY add --plan <slug> --kind open-question --title "..." --about <area>
$MEMORY add --plan <slug> --kind dead-end   --title "..." --about <area>  # a path tried and abandoned
```

`dead-end` is worth the keystrokes: without it, a future session cheerfully
tries the thing that already failed.

Answer an open question or an assumption by linking the record that settles it,
rather than by editing anything:

```bash
$MEMORY link --plan <slug> --type answers --from <new-id> --to <question-id>
```

Then `$MEMORY validate --plan <slug>` to confirm the log is still sound.

### 10 — Decide what comes next

Say, in a few lines: what the objective still needs, and what the obvious next
unit looks like. Then ask whether to keep going here or stop.

Keep going in this session when the context still has room and the next unit is
small. Stop when the context is filling — a session that pushes on past that
point starts forgetting the objective, and everything after that is worse than
what came before it.

When stopping, hand over a prompt the user can paste into a fresh session:

```
Continuá el objetivo `<slug>` en <repo>. Corré el skill `work`:
lo último que cerró fue `<unit name>`; lo que sigue es <one line>.
```

If the objective's success criteria are all met, say so plainly and offer the
promotion pass: which decisions, ADRs and postmortems are worth lifting from
the plan's log to the repo level (`$MEMORY promote --ids ... --to repo`). That
choice is the user's — suggest candidates, do not promote on your own.

## Key principles

- **The objective is the thing being served.** The unit is only a means, and a
  unit that cannot say how it serves the objective is not ready to run.
- **Criteria are frozen before there is any incentive to move them.** That is
  the entire defense against verification that grades its own work.
- **Required fidelity is agreed up front.** A mock satisfying a criterion about
  a real database is not a passing check, it is a false one.
- **The one who built it does not get to bless it.**
- **"Not worth fixing" is a real answer, and it gets recorded.** Otherwise the
  same imaginary risk comes back every turn.
- **Stop before the context fills, not after.** Splitting a unit is cheap;
  losing the thread halfway through one is not.
