---
name: plan
description: "Author an objective before any code is written — explore the problem, clarify what is genuinely ambiguous, weigh the real options with the user, and produce a reviewable draft, with acceptance criteria and the fidelity each one demands, that they can read in full and correct. Use this whenever the user describes something they want built, changed, migrated or fixed and the shape of the work is not already settled between you. Use it again at any depth: opening a more specific objective underneath one that already exists is this skill's job, and it will keep descending until it reaches one a single session can deliver. Triggers on: 'plan this', 'planeá esto', 'quiero hacer X', 'necesito implementar', 'how should we approach', 'armemos un plan', 'ayudame a pensar', 'vamos a refactorizar', 'necesito migrar', 'quiero agregar la feature', 'me gustaría que el sistema haga X', 'esto es más grande de lo que pensábamos', 'abramos algo más chico debajo', 'define the acceptance criteria'. Use it for small work too — a two-file fix gets the same structure, just one objective instead of a tree, so never skip it because the task looks small. To move an objective that already exists and already carries fidelity-annotated criteria forward, use the `work` skill instead; `plan` authors, `work` executes. When what arrived is a symptom whose cause nobody has established, this skill invokes `debug` before writing any criterion, and authors against what it finds."
---

# plan

Read [the host runtime](references/runtime.md) first for tool invocation,
delegation, repository paths and the memory prerequisite.

Turn a request into a written objective, at whatever depth the work actually
sits, and stop there. When the objective is too big for one session, plan one of
its children in the same conversation, and keep going until you reach one a
session can deliver. That one goes to `work`, which executes it.

## The shape this produces, and why it is not a phase list

Objectives nest. An objective may derive from another, and this skill writes one
at any depth: a root for a job nobody has framed yet, or something more specific
underneath an objective that already exists. Going one level down is an ordinary
move, not an admission that the objective above it was wrong.

There is deliberately no numbered list of phases with their contents settled up
front, and no enumeration of the tree underneath the objective you are writing.
Writing the detail of a grandchild objective now means writing it with the least
information you will ever have about it, and once it exists as an approved
artifact, changing it costs enough that people stop changing it and start
following a plan they know is wrong. Defining each objective just before it runs
makes reordering, splitting and dropping cost nothing — they are edits to a
list, not a re-planning ceremony.

What you *do* fix now: this objective, its constraints, what is out of scope,
and a rough orientation of the areas involved — with the children it probably
has left as prose in that orientation. That is enough to judge whether any given
child is worth opening, which is the only thing the descent needs from you.

## Hard rules

1. **Never implement anything.** This skill writes the objective and stops. If
   the user says "just do it", write the objective first — it takes one turn and
   it is what makes the rest of the work checkable.
2. **Every assumption gets written down.** Not asked about, necessarily —
   written. The draft has a section for them. An assumption you make silently is
   the exact mechanism by which what you build diverges from what was wanted,
   and it is invisible until it is expensive.
3. **The draft is the readback for the objective, the constraints and the
   criteria.** Do not restate those in chat and ask "does this look right?" —
   the user needs to read the detail, not your summary of it. Point them at the
   file. The one exception is scoped by content, not by timing: your judgment
   calls and the still-open questions go in front of the user in chat — step 4's
   round, whenever it runs, including when step 6's feedback opens a new one.
   Those are the cheapest things to correct and the most expensive ones to find
   buried in a long draft.
4. **A library version, an SDK shape or a provider API is resolved, never
   recalled.** Your training data lags releases. If the plan turns on an external
   dependency, look it up — `context7` if available, otherwise the pinned source
   in the dependency tree, otherwise the web — and cite what you used.
5. **Conversation in the user's language; everything written to disk in
   English.**
6. **Never invent a path, a name or a number.** If you need one and cannot
   derive it, that is a question or an assumption, and both have a home.
7. **Never write a state edge.** A `planned` objective is one with no state edge
   at all. The edge that says execution has started is `work`'s to write, at the
   moment execution starts — authorship finishing and execution starting are two
   different events, and only the second one is a change of state.
8. **The parent is context, never a constraint.** Nothing here derives a
   requirement, a criterion or a refusal from the objective above.

## The memory log

State lives in an append-only log, read and written only through the `memory`
CLI. Never edit the log by hand.

`memory kinds` lists every valid record kind and edge type and reports the count
at which `query` starts withholding; `memory --help` lists the flags each command
takes and how naming a plan bounds what a read returns. `memory state` reports
the objectives in the log with each one's `parent`, `children` and `depth`. A
command this file does not show is one of those reads away.

## Steps

### 1 — Understand the request, place it in the tree, then go look

Read what the user asked for. Then find out where it sits:

```bash
memory state
memory query --plan <tree-slug> --kind objective --current
```

`state` is given no plan filter on purpose: `--plan` is accepted there and
ignored, because an objective's parent, its children and its depth are
properties of the log rather than of the question. Several roots in one log are
normal — nothing requires an objective to have a parent.

Three cases, and between them they are the whole placement decision:

- **Nothing open that this belongs under** — you are writing a root objective.
- **The request is more specific than an objective already open** — you are
  writing that objective's child. Take the parent's id and its tree slug from
  `state`.
- **The request *is* an objective already open whose draft already carries
  `## Acceptance criteria` with a fidelity line on each one** — it has been
  authored, and re-authoring it would move criteria that are supposed to be
  settled. Point the user at `work`, which executes it.
- **The request is that those frozen criteria are *wrong*** — this is not the
  case above and it is not refused. It is a **supersede**, and it is yours:
  write a new objective with the criteria the user actually wants, and in step 7
  link it with `--type supersedes` to the objective it replaces. Editing frozen
  criteria in place is what this refuses; replacing them with a record that says
  so is authorship. `work`'s gate routes feedback that touches a criterion here
  on purpose, so this is the case arriving from there.

The line between the last two is "run this" versus "these criteria are wrong".
The first goes to `work` untouched. The second never gets applied by the thread
that will be graded against it.

Then investigate before forming an opinion — launch the `explore` subagent
rather than reading widely in this thread, so the main context stays free for
the objective itself:

```
topic: <the one question that decides the shape of this work>
repo_root: <absolute path>
scope: both
```

When you arrive here from step 8 — a descent into a child — this same
invocation also carries `prior_findings`, the parent's `## Findings` with every
claim and its `path:line` anchors intact, and asks for revalidation rather than
a fresh exploration. Step 8 specifies that block and what comes back; launching
`explore` without it makes the child pay again for what the parent already
established.

Fold its `findings` into your understanding and keep its `gaps` — they belong
in the draft's "What I don't know". Keep the findings with their `path:line`
evidence intact: a parent objective's draft carries them under `## Findings`, and
a paraphrase that drops the anchors is what makes the next level down pay for the
same exploration twice.

#### When the cause is not established, `debug` runs first

This is the check to make here, with `explore`'s findings in hand and before step
2 asks anything, because it is the last cheap moment: from step 5 onward you are
writing criteria, and criteria written against a cause nobody has established
freeze a guess into the thing every later step is graded against.

The test is narrow, and it is **not** "the request mentions a bug":

- **The work turns on *why* something is failing, and nobody can say why** — the
  symptom is known, the cause is not, and what to build depends on which it turns
  out to be. That goes to `debug`.
- **The cause is understood and what is left is to build the fix** — somebody can
  state it in a sentence and point at where it happens, whether they found it
  five minutes ago or six months ago. That is an ordinary objective and it is
  yours. A bug being the subject changes nothing about the shape of the work.

The line is whether the criteria you are about to write depend on an answer
nobody has. When they do, say so and **invoke the `debug` skill in this same
conversation** — the Skill tool, skill `debug`, or `ai-kit:debug` where the plugin
namespace is in play — with three things: the symptom as the user stated it, the
tree slug you resolved above, and whatever `explore` established, its claims and
`path:line` anchors intact. It takes all three as given rather than paying for
them again — the slug is the one its hypotheses get written under and the one you
will pass to `add` in step 7, and the findings go into its own `explore` call as
`prior_findings`. It narrows the cause, writes each hypothesis it tests to the
log, and comes back here with a confirmed one, or with an honest "I don't know"
and the list of what it ruled out. Then start this step again against what it
found.

All three of its findings are things you can author against, and the third one is
the one worth naming. An investigation that **confirmed a cause** gives you the
thing to write criteria about. One that **ran out of hypotheses** gives you the
objective that makes the thing observable, which is real work with real criteria.
One that concluded **nothing is broken** — a misread output, an expectation that
was never true, a difference between two environments — is not a wasted trip and
not an input to refuse: it is a finding, and what follows from it is a question
for you and the user, whose honest answer is sometimes a small objective (a
guard, a clearer error, a line of documentation) and sometimes none at all. Say
so plainly when it is none; a `plan` that concludes there is nothing to build has
done its job. What no finding gives you is permission to guess.

**When you were invoked *from* `debug`, this check is already answered** and
sending it back would be a loop between two skills each waiting for the other.
The finding it handed you — the confirmed hypothesis with its fidelity, or the
list of what was ruled out — is what you author against, and step 7 links the
objective to it.

#### Show the immediate parent, and say what it is not

When the objective you are about to write has a parent, show it to the user
before writing anything — its title and its id, straight from `state`:

> Esto queda debajo de **`<parent title>`** (`<parent id>`). Te lo muestro como
> contexto, no como límite: sirve para que veamos si lo que vamos a hacer aporta
> ahí, pero no decide nada de este objetivo.

Say it in those terms, because it is true. The parent is there so the two of you
can see what this objective is in service of, and for nothing else:

- It adds no requirement. A constraint binds this objective only if it is
  written into *this* draft.
- It adds no criterion. The parent's success criteria are the parent's, and
  restating them here would grade a child against work it was never scoped to
  do.
- It is never a reason to refuse. Not the depth, not "the parent does not mention
  this", not "that belongs to a sibling". If the request does not fit under this
  parent, then it is a root objective or a child of something else — say which,
  and keep going.

Only the *immediate* parent is shown. Walking the chain all the way up turns
context into a wall of text nobody reads, which comes to the same thing as
showing none of it.

### 2 — Ask only what is genuinely ambiguous

Ask when the answer changes what gets built:

- the outcome is clear but the surface is not — which endpoints, which screens
- two or more reasonable approaches lead to materially different work
- an unresolved decision sits underneath everything else — data model,
  ownership boundary, sync vs async
- there is no way to tell when this is done

Ask those in one `AskUserQuestion` call, multiple choice where the options are
discrete. Do not ask a question whose every answer leads to the same next step.

One answer sends you backwards rather than forwards: **"we don't actually know
why it does that."** An unknown cause surfaces here at least as often as it does
in step 1, because it is a question's answer more often than it is a request's
wording. Go back to step 1's `debug` check with it — the criteria have not been
written yet, and that is the only reason this is still cheap.

Everything you did **not** ask — every judgment call you made to keep moving —
goes on the table in step 4 and then in the draft under Assumptions. That is the
trade: you get to keep moving without interrogating the user, and in exchange
nothing you decided stays invisible.

### 3 — Put the real options on the table

When there is more than one reasonable way to do this, say so with the
tradeoffs and a recommendation. Two or three options, each with what it costs
and what it buys, and a plain statement of which one you would pick and why.
The user chooses; never pick silently and never present a single option as
though it were the only one.

Options that turn on an external dependency get rule 4 applied first — resolve
the version, then present the option.

Before presenting them, read what this repository already chose in the areas the
options touch, one area at a time:

```bash
memory areas --kind preference
memory query --scope both --kind preference --about "<one of the areas the options touch>" --current
```

`areas` names the areas anybody has a recorded choice about, in this repo's log
and the user's, and which record is live in each; `query --scope both` then reads
that area's live answer out of both of those logs as one answer, most recent
first, each record naming in its own `scope` which log it came from. **`--scope
both`, not the default**: on its own `query` reads this repo's log alone, so a
choice promoted up out of some other repository is named by `areas` one line
earlier and then comes back `count: 0` — a query that ran correctly and asked the
wrong log. A promoted choice applies here exactly as much as a local one does,
which is the whole reason the user's log is read at all. No `--plan` on either
line — these records are written without one because they belong to the repository
rather than to a tree, and a plan-bounded read returns them anyway. Read per area
rather than in bulk, so the query stays under the count at which `query` withholds
its records and answers `count: N, returned: 0` instead. Asking for less of each
record does not move that line — the guard counts the matches before `--fields`
and `--body-lines` project them — but it is what makes a returned record cheap.
`--fields id,at,title` is the citation below and nothing else, and
`--body-lines 0` keeps the reasoning out of this thread when only the title is
going in front of the user.

What comes back goes **beside** the options, as context, never in place of one:

> Opción A: consultas generadas. Opción B: un ORM. Contexto: hay una preferencia
> registrada para este repo que dice consultas generadas y no ORM
> (`01KZ…`, 2026-03-14). No decide esto — te la muestro para que la tengamos a la
> vista.

**Every option that would have been presented is still presented**, including the
one the recorded choice argues against. That is the trade this makes and it is
worth naming: it re-opens things somebody already settled, and it costs a round
that would otherwise not have happened. What it avoids is worse — a choice made
months ago quietly freezing this one, with nobody in the conversation aware that
an option was dropped before they saw it. A recorded choice is a fact about what
was picked before and why; it is not a ruling on this objective, and it is never a
reason to present fewer options or to present one as already settled. And when the
round concludes that the recorded choice is itself wrong, it does not get corrected
here — step 7 says where a choice that outlives this tree belongs.

### 4 — Check your judgment calls before any file exists

Nothing is written yet, and that is what makes this round cheap. The same content
inside a 300-line draft costs the user a full read and costs you a full rewrite,
so the decisions that are still soft get looked at here instead.

Give every judgment call you made as **numbered prose, one line each** — what you
decided, the alternative it displaced, and where the decision came from:

> 1. Los rechazos van dentro del record de `decision` como campos, no como
>    records aparte — desplazó un record por opción rechazada. Origen: `my call`.

Provenance is a closed set of three values, and each one means exactly what it
says:

- **`you chose this`** — the user picked it, in this conversation, when asked
  about *this* decision.
- **`their words`** — it is in something they said or wrote, close enough that
  they would recognize it as theirs.
- **`my call`** — you decided it. This is the default, and it stays the default
  for every assumption nobody has looked at yet.

Answering a nearby question never promotes an assumption to `you chose this`, and
answering a multiple-choice question that happens to touch the same area does not
either — an option they picked answers the question you asked, not everything you
decided around it. This repo has the failure on disk:
`plans/loop-ergonomics/draft-objective.md`, under **Assumptions, with where each
one comes from**, records six assumptions marked confirmed on the strength of
adjacent multiple-choice answers, from a list the user had never been shown.

Then ask, in two halves that stay distinguishable:

- **The discrete questions, batched.** Everything still open whose answers are a
  short closed set goes in **one** `AskUserQuestion` call — one call for the whole
  round, not one per question.
- **The open ask, on its own.** Multiple choice cannot carry "number 3 is wrong"
  and it cannot carry "you missed an entire area", so ask for both in words:

> Arriba están las suposiciones numeradas. Corregime cualquiera por número —
> decime cómo va en realidad — y decime qué falta que no esté en la lista.
> Todavía no escribí nada, así que corregir acá no cuesta nada.

This round covers **only** the assumptions and the open questions. Not the
objective, not the constraints, not the criteria, not what is out of scope —
those are what the draft is for, and reading them back in chat is how the file
stops being read.

Then update the labels from what they answered, and only from that. An assumption
they corrected in words becomes `their words`; one they chose between, in a
question that was about that assumption, becomes `you chose this`; one they did not
touch keeps `my call`, however obvious it looks to you.

### 5 — Decide whether this objective is executable, then write the draft

An objective is **executable** when one session can deliver it and somebody else
can check it afterwards. That is a test with five parts, and it has to pass all
five:

- **It delivers something demonstrable.** At the end you can show it working.
  Not "the layer is refactored" — "you can cancel an order and see the reason
  recorded".
- **It fits one session with room to spare.** Context that fills up is context
  that starts forgetting the objective, which is the failure this whole shape
  exists to avoid. When in doubt, split.
- **It is worth doing on its own.** If it only makes sense once a sibling
  objective lands, either merge them or do that one first.
- **It names what it does not cover.** Two or three non-goals. This is the
  cheapest scope control there is.
- **It says how it serves the objective above it**, in one line. If that line is
  hard to write, that is the signal — ask the user before going further.

An objective that fails any of the five is a **parent objective**. It is real, it
gets written and recorded like any other, and its completion comes from its
children closing underneath it rather than from a verification pass — there is
nothing for `verify` to grade in a draft no single session can execute.

**The heading over the criteria is what says which of the two you wrote**, and
you write one heading or the other, never both in the same draft:

- **`## Success criteria`, with no fidelity line anywhere in the draft** — not
  executable. Its criteria are observable statements about the whole objective,
  met when its children are, so nothing grades this draft directly. They describe
  the outcome and never restate the children: a criterion that names a child is a
  phase list in a criterion's clothes, and it goes stale the moment the tree
  changes shape.
- **`## Acceptance criteria`, numbered, with a `fidelity required:` line on
  every criterion** — executable, and that section is the only thing `verify`
  grades.

`## Acceptance criteria` is the exact heading `verify` reads — a criterion under
any other heading is a criterion nobody will check. The mistake runs the other
way too: fidelity lines under a `## Success criteria` heading, or acceptance
criteria on an objective one session cannot deliver, promise a verification pass
that has no honest way to run.

Write `.ai-kit/plans/<objective-slug>/draft-objective.md`. No record exists yet,
so you coin the slug here — and it is the record's own `slug` too, because step 7
puts this same slug on the objective's batch line, where it is taken exactly as
given. Derive it
from the title the way the tool would if you left it out: lowercase, every run of
characters outside `a-z0-9` collapsed to a single hyphen, no leading or trailing
hyphen, at most 60 characters — truncate there and trim any hyphen the cut leaves
behind. One directory per objective, flat, because the nesting lives in the log
and a path that mirrored the tree would have to move every time a parent changed.

Under `.ai-kit/` and not at the repository root, because that is the directory the
`memory` CLI keeps out of git by itself — it writes a `.gitignore` containing `*`
inside it when it creates it. A draft at the root needed somebody to add a line to
the repository's own `.gitignore` by hand, which is why the drafts of this loop
have only ever stayed out of a pull request in the one repository where somebody
did.

If a draft for that slug already exists, that slug belongs to another objective —
two objectives can legitimately derive the same one — and writing there would
overwrite somebody else's draft: pick a slug that distinguishes this objective, and
put that same slug on the objective's line in step 7. Look in **both** locations,
the new one first:

```bash
sh -c 'for p in .ai-kit/plans/<slug>/draft-objective.md plans/<slug>/draft-objective.md; do
  [ -f "$p" ] && { echo "$p"; exit 0; }
done; exit 1'
```

`plans/<slug>/` at the repository root is where drafts were written before this
moved, and the ones already there are not migrated — so the slug is taken if
either path answers, while a new draft is always written under `.ai-kit/`.

It prints the first path that exists and **exits 0 only when it found one**, which
an `ls` over both paths does not: `ls` exits 1 whenever either argument is missing,
so a draft that exists in one location comes back printed and failed at the same
time, and the next edit that chains this with `&&` reads that as no draft. The
`sh -c` wrapper is not decoration either — nothing here says which shell this turn
got, and a `for` loop written this way is not something every shell parses.

#### A parent objective's draft

```markdown
# Objective: <one line>

## What I understood
<The problem in your words, with enough detail that a wrong reading is
obvious to the user. Not a restatement of their sentence.>

## Findings
- <one line per claim `explore` established, followed by its `path:line` anchors.
  Keep the anchors — they are what lets a child revalidate this claim mechanically
  instead of exploring it again from scratch.>

## Objective
<What must be true when this is done. One paragraph.>

## How this serves the objective above
<One line. A root objective serves the request that opened it.>

## Assumptions I made
- <each judgment call you made without asking, one line: what you decided, the
  alternative it displaced, and `provenance:` one of `you chose this`,
  `their words`, `my call`>

## What I don't know
- <each open question, including gaps `explore` reported>

## Constraints
- <technical, product, or process limits that bind the work>

## Out of scope
- <what this objective explicitly does not cover>

## Success criteria
- <observable statements. Each one must be able to fail.>

## Orientation
<Areas likely involved, known risks, a tentative order of attack, and the
children this objective probably has — one sentence each, as prose. This is a
sketch to steer by, not a contract. Say so in one line.>

## Sources
- <what you read or resolved; library versions with where you checked>
```

#### An executable objective's draft

```markdown
# Objective: <one line>

## What I understood
<The problem in your words, with enough detail that a wrong reading is
obvious to the user. Not a restatement of their sentence.>

## Deliverable
<What will be demonstrably working when this is done. One paragraph.>

## How this serves the objective above
<One line. A root objective serves the request that opened it.>

## Assumptions I made
- <each judgment call you made without asking, one line: what you decided, the
  alternative it displaced, and `provenance:` one of `you chose this`,
  `their words`, `my call`>

## What I don't know
- <each open question, including gaps `explore` reported>

## Constraints
- <technical, product, or process limits that bind the work>

## Out of scope
- <what this objective does not touch>

## Acceptance criteria
1. <Observable statement that can fail.>
   - **Verified by**: <the specific check> — **fidelity required**: real-dependency | integration | mocked | code-read | you-run
2. ...

## Files and areas expected
<Best current understanding. Not a contract — deviations get recorded.>

## Open questions carried in
- <anything from memory still unanswered that this objective touches>

## Sources
- <what you read or resolved; library versions with where you checked>
```

Every section the loop reads is in there, so nothing downstream has to add one:
`verify` opens the file, grades `## Acceptance criteria` and checks the changed
files against `## Out of scope`; `implement` and `review` read the whole thing.
`work` extends the sections around the criteria as it goes and leaves the
criteria themselves alone. A parent's `## Findings` is what step 8 hands down to
the child's `explore`, which is why the anchors go in the file instead of being
paraphrased into `## What I understood`.

**The fidelity line on each criterion is the load-bearing part.** It is decided
here, with the user, before anyone has an interest in the answer. Naming one
above `mocked` is what makes it impossible for verification to later report the
criterion satisfied on the strength of a mock. Choosing `mocked` is allowed — it
just has to be a choice made in the open, now, rather than a convenience
discovered later.

`you-run` is the fifth value and the only one that names whose turn it is, and
which criteria get it is a mechanical test rather than a judgment about how
risky the check feels. Read the check against `agents/verify.md`'s closed list
of what no agent ever runs: a criterion whose check lands on that list — a real
database, a queue, a call over the network — is written `you-run` here, because
no agent is going to execute it and any other fidelity written on it comes back
`unverified` by construction. `implement` then leaves a runnable script and
written instructions in `.ai-kit/plans/<slug>/manual-checks.md` and runs
neither, `verify` reports it handed over instead of ever reporting it `met`, and
`work` asks the user what happened before the objective closes. Written as
`real-dependency` instead it buys an `unverified` that reads as *the agent could
not manage it*, which is a different sentence from *this one is yours*.

So `real-dependency` is what is left for a check that reaches something real the
agent may still touch — the working tree's own filesystem, a process it starts
itself — where the bar is above a double and still reachable by whoever
verifies.

**A unit test is authorized here or nowhere.** `agents/implement.md` prohibits
writing one nobody asked for, and this is the only door left open: what
authorizes a unit test is an acceptance criterion that names the business rule
it covers. The naming is the whole of the condition — "the module is tested"
authorizes nothing, "a transfer that would overdraw the source account is
rejected" authorizes exactly one test and says what it asserts.

**That is not a call to make on your own while writing the draft.** Ask, in the
round this step already has with the user: name the rules you believe are worth
a test of their own, say what each would cost to get wrong, and let them keep
the ones that earn it. What they keep becomes a criterion; what they do not gets
no test, and that is the intended outcome rather than a gap for somebody to fill
in later. A draft that names none is an ordinary draft, not an oversight.

Two sections carry most of the value. **Assumptions** is what makes "don't
assume anything" auditable instead of aspirational. **Out of scope** is what
stops the work from quietly growing — it is worth pushing for two or three
entries even when the user shrugs.

### 6 — Hand the draft over and wait

This is the only time anybody asks the user to read these criteria. `work` does
not ask again, so what they let through here is what gets built and graded.

For an executable objective, the criteria and the fidelity each one demands are
the thing to read:

> Escribí el objetivo en `.ai-kit/plans/<slug>/draft-objective.md`. Leelo entero
> — sobre todo los **criterios de aceptación** y la fidelidad que pide cada uno,
> porque son los únicos contra los que se va a verificar después y no se cambian
> una vez que arrancamos. Después mirá **Assumptions I made** y **What I don't
> know**, que es donde está lo que decidí sin preguntarte. Contame por chat qué
> está mal — el archivo lo reescribo yo con lo que me digas, no hace falta que lo
> edites.

For a parent objective there is nothing to verify yet, so the scope and the
judgment calls are what matter:

> Escribí el objetivo en `.ai-kit/plans/<slug>/draft-objective.md`. Abrilo y
> leelo entero — sobre todo **Assumptions I made** y **What I don't know**, que es
> donde está lo que decidí sin preguntarte. Contame por chat qué está mal — el
> archivo lo reescribo yo con lo que me digas, no hace falta que lo edites.

Then stop. Do not proceed on silence, and do not re-explain the draft's
contents in chat — that defeats the point of having written it.

Feedback comes back in chat, and applying it is yours. When it introduces a
judgment call the round never covered — a new decision, an alternative you had not
weighed, a question that is now open — take those items back through step 4's
round before you touch the file. Rewriting the draft around an assumption the user
has never seen is exactly what the checkpoint exists to prevent, and it costs more
here than it did before the file existed.

Then rewrite the **whole file**, not the parts that changed. A draft patched in
place becomes a collage where the corrections no longer agree with the paragraphs
around them. Afterwards, name the sections that moved and stop there:

> Reescribí el archivo. Cambiaron **Assumptions I made**, **Out of scope** y el
> criterio 3.

Naming a section is an index, not a summary — the content still has to be read in
the file. Saying what those sections now say is the chat summary hard rule 3
forbids, so do not do it: no "ahora dice que…", no paraphrase of the new criterion,
no diff in prose.

### 7 — Commit it to memory

Only once the user approves. The draft is a negotiating surface; memory is the
record.

`--plan` names the **objective tree**, not this objective. For a child, take it
from the parent's own record — `origin.plan`, which `state` reports as `plan` —
rather than coining a slug out of the new objective's name. For a root, coin one
for the tree — unless `debug` ran, in which case it already resolved or coined
that slug and wrote its hypotheses under it, and using a second one would put the
investigation and the objective it produced behind two different queries. And
never run `memory init` for a child: one log holds one objective tree, `init` is
for a repo that has no log at all, and an edge in a batch still names an endpoint
already in the log it is written into — so the batch below, which is what
attaches the child to its parent, is refused outright when the child goes into a
log of its own. `link` would take that edge, since an edge may now cross logs,
but `graph` and `edges` still read one log and neither would see the tree it
joined.

What this step writes is the objective, one record per surviving assumption, one
per open question, one per decision the conversation settled, and the edges that
attach all of it to what it came out of. That is a dozen records carrying
multi-paragraph bodies, which is a script rather than a command line — so it is
written as files and appended by commands that read them.

**It is two files, in this order, and the order is forced.** Every record other
than the objective carries a `derives_from` edge naming the objective it was
written for, which is what makes "what did this objective leave open" a question
the log answers rather than a judgment somebody makes at the close by reading
titles. That edge names an id, so the objective has to be on disk before the
lines pointing at it can be written — and one batch cannot do both halves, because
`internal/memory/batch.go:96-100` says there is no way to draw an edge between two
entries of the same batch: neither of them has an id a caller could write down. So
the objective goes in alone and comes back with an id, and the id goes into the
second file by hand.

**Write both files with whatever writes files. Not with the shell.** One JSON
object per line, each line carrying the fields `add`'s flags carry — `kind`,
`title`, `body`, `slug`, `about`, `anchor` — plus the edges that line's record
takes part in. No `echo`, no heredoc, no quoting to get wrong: a body with
paragraphs in it is a JSON string, and the shell never sees it.

That is the same precaution the rest of this file takes, applied where it matters
most: **nothing here says which shell this turn got**, and three shapes vary
between them — an assignment, `${braces}`, and a bare `<…>` that a shell may read
as a redirect. A shell that refuses one of them refuses the whole block at parse
time, with none of its lines having run. A dozen bodies going through a heredoc is
a dozen chances to meet one of the three; a file the shell never opens has none.

First, `.ai-kit/plans/<objective-slug>/objective.jsonl`, holding exactly one line —
the objective and nothing else:

```jsonl
{"kind":"objective","slug":"<the slug you derived in step 5>","title":"<objective one-liner>","body":"<Objective or Deliverable + Constraints + Out of scope + the criteria as written>","about":["<area>","<area>"],"edges":[{"type":"derives_from","to":"<parent-objective-id>"}]}
```

```bash
memory add --plan <tree-slug> --batch .ai-kit/plans/<objective-slug>/objective.jsonl
```

**Read the id off the row that command printed — the one whose kind is
`objective` — and type it into the second file.** Never capture it: no
`OBJ=$(memory add …)`, no substitution of any shape. An assignment of a command
substitution is one of the shapes a shell may refuse outright, at parse time,
before any line of the block has run, while a shell that accepts it runs the same
text through — and the id is going into a file the shell never opens anyway. One
you can read where you wrote it is one you can see is wrong.

Then `.ai-kit/plans/<objective-slug>/records.jsonl`, one line per record, every
one of them carrying that id:

```jsonl
{"kind":"assumption","title":"<what you assumed rather than established>","body":"Provenance: <you chose this | their words | my call>","about":["<area>"],"edges":[{"type":"derives_from","to":"<the objective id the first command printed>"}]}
{"kind":"open-question","title":"<what is genuinely open>","about":["<area>"],"edges":[{"type":"derives_from","to":"<the objective id the first command printed>"}]}
{"kind":"decision","title":"<the choice the conversation settled>","body":"<why this over the alternatives>","about":["<area>"],"edges":[{"type":"derives_from","to":"<the objective id the first command printed>"}]}
```

```bash
memory add --plan <tree-slug> --batch .ai-kit/plans/<objective-slug>/records.jsonl
```

The edge goes on every line of that file and there is no exception — an assumption
without it is one nothing can ask about later. `edges` is a list, so a line that
already takes an edge of its own keeps it and takes this one alongside: a
`decision` that settles a question carried in from an earlier turn carries its
`answers` edge and its `derives_from` in the same array.

**Every entry in one file is appended in one write, or none of them is.** A line
the tool refuses refuses the whole batch, names the line and the field, and leaves
the log exactly as it was — which is what makes a file of a dozen records safe to
write at all. A dozen separate commands had the failure this log cannot undo: the
third lands, the fourth is refused, and half a plan is on disk forever. That
guarantee is per file and the split does not weaken it: the objective lands whole
or not at all, and so do its records.

So exit 1 on either command needs no repair, only a re-run of **that** command:
nothing from its file landed, the file is still on disk, and the remedy is to fix
the line the refusal named and run it again. Nothing is written twice, because
nothing was written the first time.

What the split does cost is one state that could not happen before. **A refusal of
the second command leaves the objective on disk carrying none of its assumptions,
questions or decisions.** That is visible rather than silent — the refusal names
the line and the field, and the objective is there to be queried with nothing under
it — and it is repaired by fixing that line and running **only the second command**
again. Do not re-run the first. The objective is already in the log, an append-only
log cannot take a duplicate back, and a second objective line would coin a second
id that the records pointing at the first one do not name.

**Each line's edges name one endpoint, and the line's own record is the other.**
`"to"` says this record is the edge's source, which is what `derives_from` wants —
the child derives from the parent whose id you already have, and every record in
the second file derives from the objective the first one wrote.

`work` finds the draft by the record's slug, and passing `slug` is what makes
that slug the draft directory's name too — one name, rather than two that have to
be kept in step. It is taken exactly as given.

An objective derives from at most one objective, and a second `derives_from` is
refused naming the parent it already has. There is no taking a parent back
afterwards, so point it at the one it is actually under. **A root has no parent:
drop the `edges` key from its line entirely.**

When you were invoked from `debug`, the objective also points at the hypothesis
whose verdict is `confirmed`, and that is a second edge on the same line:

```jsonl
{"kind":"objective","slug":"...","title":"...","body":"...","about":["<area>"],"edges":[{"type":"derives_from","to":"<parent-objective-id>"},{"type":"caused_by","to":"<confirmed-hypothesis-id>"}]}
```

It goes in now, on the objective's own line, because this is the last moment
anybody knows which hypothesis this work came out of. Left to a later step, the
investigation and the work it produced are associated only by sharing a plan slug —
which is to say by a string, findable by nobody who does not already know to look.
`caused_by` already exists and points a record at what led to it; there is no new
edge type here.

Only when `debug` ran. An objective planned from a request has nothing to point
at, and inventing an endpoint for the edge is worse than not having it — a batch
naming an id the log does not hold is refused whole, which is the loud version of
that mistake. When the investigation ended without a confirmed hypothesis, there is
no target either: the objective you just wrote is the one that makes the thing
observable, and it caused nothing yet.

In the second file: one line per surviving assumption — these are open until
something answers them, and they are what a later verification pass is allowed to
challenge. The provenance label goes in the body, on its own line, exactly as
written above. One line per open question. One line per decision the conversation
settled, especially a chosen option.

That `Provenance:` line is read, not just stored — `work`'s step 1 puts these
records in front of the user across later turns, and it can only show the label
if the record carries it. Keep the value to one of the three tokens step 4
defined, verbatim. A label that lives only in the draft file lasts exactly as
long as somebody is reading that draft, and re-deriving it later from memory is
how six assumptions ended up marked confirmed.

A `decision` is scoped to this objective tree, and some of what the conversation
settled is not. "We use generated queries rather than an ORM" will still be true
two trees from now, and filed under this slug it disappears from view the moment
this tree closes. A choice that outlives this objective tree belongs in a
`preference`, and the skill that gathers those — checks them against what the
repository actually does, and offers to carry them up to user scope — is `prefer`.
**Name it here and do not run it.** No step of `plan` invokes that skill and
nothing in `plan` writes that kind of record: say in one line which of the settled
choices looks like one, and leave the invoking to the user.

Tag every line's `about` with the areas and paths a future session would search
by. A record nobody can find by filtering is a record nobody will read.

`add --batch` prints `count`, how many edges went with it, and one row per record —
id, kind, slug, title — and no bodies: they are already in the file you wrote. The
first command prints a single row and its kind is `objective`: that id is what the
second file's edges name and what step 8 reports to the user. Every command prints
JSON on success and on failure alike, so check the exit code rather than scanning
the text.

Leave the draft file and both batch files on disk. The draft is the human-readable
form of what was agreed and the loop will want to look at it; the batch files are
what was actually appended, and `.ai-kit/` is already out of git.

### 8 — Descend, or hand off

**If what you just recorded is executable**, the descent is over. Say what was
recorded and point at the next step:

> Objetivo registrado (`<id>`), con N suposiciones y M preguntas abiertas. Los
> criterios ya los aprobaste, así que `work` arranca directo: implementa,
> verifica contra esos criterios y te trae el resultado.

If they want to start now, continue into `work` in this same session. The
criteria they just read are the ones it runs against, and it does not make them
read the same text a second time.

**If it is not executable**, do not stop at the top of the tree. Offer to plan
one of its children right now, in this same conversation:

> `<title>` no entra en una sesión. Abajo veo `<A>`, `<B>` y `<C>` como
> candidatos. Puedo planear uno ahora mismo — ¿con cuál arrancamos?

The user picks; never pick silently among several. Then go back to step 1 for
that child, with the objective you just recorded as its parent, and do the whole
thing again: place it, look, ask what is genuinely ambiguous about *it*, put your
judgment calls on the table, write its draft, hand it over, record it with
`derives_from`. Stop descending at the first objective that passes all five parts
of step 5's test — that one is what goes to `work`.

The child's `explore` does not start from nothing. Hand it what the parent's
`## Findings` already established — the `findings` array as it came back, each
element keeping its `claim` and its `evidence` — and ask it to revalidate those
and explore what is new, instead of establishing the topic again:

```
topic: <the one question that decides the shape of this child>
repo_root: <absolute path>
scope: both
prior_findings: <the parent's findings, claim and evidence intact>
```

It returns `revalidated` alongside its own `findings`: one
`{claim, evidence, verdict}` per inherited claim, with `verdict` exactly one of
`still_holds`, `changed` or `could_not_check`. That is what tells you which case
you are in — you cannot know before checking the anchors whether this child needs
a full exploration or a paragraph of updates. A claim that still holds is a file
nobody has to read twice; `changed` and `could_not_check` are where the child's
own looking starts.

**Stop descending when the context is filling**, before opening the next child
rather than partway into it: every level costs another `explore` and another
round with the user in this same context — the round is larger now that it
carries step 4's checkpoint, and the `explore` is smaller when the parent's
findings still hold — and step 5's second sizing bullet already says what a
context that fills up does to the objective it is holding. The objective step 7
just recorded is the parent the next session picks up from, so say which child
comes next and hand over a prompt the user can paste into a fresh session:

```
Seguí el árbol `<tree-slug>` en <repo>. Corré el skill `plan`: el padre es
`<parent title>` (`<parent id>`), ya registrado; el próximo hijo a abrir es
<one line>.
```

A descent that runs out of room mid-level is worse than one that stopped cleanly
a level early, because the objective it was about to write is the one nobody will
remember.

Only the child being opened now becomes a record. The rest stay as prose in the
parent's `## Orientation`, a sentence each, until somebody is about to work one.
Turning them all into records up front is the phase list this shape exists to
avoid, written at the moment you know least about them.

None of this commits to a depth. Two levels is common, one is fine, and an
objective that passes the five-part test on the first pass never has a child at
all.

## Key principles

- **The objective is the thing that has to survive.** Every later step is
  checked against it, so it is worth the turn it costs to write properly.
- **Write what you assumed, ask what you cannot assume.** Interrogating the
  user is not rigor; recording your judgment calls is.
- **The detail goes in the file, not in the chat.** Being shown only the
  headline is how a plan gets approved that nobody actually read.
- **Orientation is a sketch, not a contract.** The moment it hardens into a
  phase list with contents, changing course starts costing more than it should.
- **Stop at the handoff.** Planning that slides into implementing is planning
  nobody got to review.
- **Descending is cheap; detail written too early is not.** One record and one
  edge open a child, and the parent is still there afterwards.
- **The criteria get read once, here.** Everything downstream treats them as
  settled, which is exactly why this is the step that cannot be rushed.
