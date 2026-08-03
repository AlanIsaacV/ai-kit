---
name: plan
description: "Turn a request into a written objective before any code is written — explore the problem, clarify what is genuinely ambiguous, weigh solution options with the user, and produce a reviewable draft they can read in full and correct. Use this whenever the user describes something they want built, changed, migrated or fixed and the shape of the work is not already settled between you. Triggers on: 'plan this', 'planeá esto', 'quiero hacer X', 'necesito implementar', 'how should we approach', 'armemos un plan', 'ayudame a pensar', 'vamos a refactorizar', 'necesito migrar', 'quiero agregar la feature', 'me gustaría que el sistema haga X'. Use it for small work too — a two-file fix gets the same structure, just one turn of the loop instead of many, so never skip it because the task looks small. Do NOT use it to continue an objective that already exists: that is the `work` skill."
---

# plan

Turn a request into an objective that stays in view for the whole job, then
hand off to `work`, which runs one unit of work at a time against it.

## The shape this produces, and why it is not a phase list

There is an **objective** — stable, short, always loaded — and a **loop of work
units**, each defined immediately before it runs.

There is deliberately no numbered list of phases with their contents settled up
front. Writing the detail of unit five during planning means writing it with
the least information you will ever have about it, and once it exists as an
approved artifact, changing it costs enough that people stop changing it and
start following a plan they know is wrong. Defining each unit just before it
runs makes reordering, splitting and dropping cost nothing — they are edits to
a list, not a re-planning ceremony.

What you *do* fix now: the objective, its constraints, what is out of scope,
and a rough orientation of the areas involved. That is enough to judge whether
any given unit is worth doing, which is the only thing the loop needs from you.

## Hard rules

1. **Never implement anything.** This skill writes the objective and stops. If
   the user says "just do it", write the objective first — it takes one turn and
   it is what makes the rest of the work checkable.
2. **Every assumption gets written down.** Not asked about, necessarily —
   written. The draft has a section for them. An assumption you make silently is
   the exact mechanism by which what you build diverges from what was wanted,
   and it is invisible until it is expensive.
3. **The draft is the readback.** Do not summarize the draft in chat and ask
   "does this look right?" — the user needs to read the detail, not your summary
   of it. Point them at the file.
4. **A library version, an SDK shape or a provider API is resolved, never
   recalled.** Your training data lags releases. If the plan turns on an external
   dependency, look it up — `context7` if available, otherwise the pinned source
   in the dependency tree, otherwise the web — and cite what you used.
5. **Conversation in the user's language; everything written to disk in
   English.**
6. **Never invent a path, a name or a number.** If you need one and cannot
   derive it, that is a question or an assumption, and both have a home.

## The memory log

State lives in an append-only log, read and written only through the `memory`
CLI. Never edit the `.jsonl` by hand.

```bash
MEMORY="${CLAUDE_PLUGIN_ROOT}/bin/memory"
[ -x "$MEMORY" ] || ( cd "${CLAUDE_PLUGIN_ROOT}/tools/memory" && go build -o "$MEMORY" ./cmd/memory )
```

`$MEMORY kinds` lists every valid record kind and edge type. Read
`${CLAUDE_PLUGIN_ROOT}/tools/memory/README.md` if you need the retrieval
contract or the scope model in detail; the commands used here are below and
that is usually enough.

## Steps

### 1 — Understand the request, then go look

Read what the user asked for. Then investigate before forming an opinion —
launch the `explore` subagent rather than reading widely in this thread, so the
main context stays free for the objective itself:

```
topic: <the one question that decides the shape of this work>
repo_root: <absolute path>
scope: both
```

Fold its `findings` into your understanding and keep its `gaps` — they belong
in the draft's "What I don't know".

If an objective already exists for this work (`$MEMORY query --kind objective
--current`), this is not the right skill. Point the user at `work`.

### 2 — Ask only what is genuinely ambiguous

Ask when the answer changes what gets built:

- the outcome is clear but the surface is not — which endpoints, which screens
- two or more reasonable approaches lead to materially different work
- an unresolved decision sits underneath everything else — data model,
  ownership boundary, sync vs async
- there is no way to tell when this is done

Ask those in one `AskUserQuestion` call, multiple choice where the options are
discrete. Do not ask a question whose every answer leads to the same next step.

Everything you did **not** ask — every judgment call you made to keep moving —
goes in the draft under Assumptions. That is the trade: you get to keep moving
without interrogating the user, and in exchange nothing you decided stays
invisible.

### 3 — Put the real options on the table

When there is more than one reasonable way to do this, say so with the
tradeoffs and a recommendation. Two or three options, each with what it costs
and what it buys, and a plain statement of which one you would pick and why.
The user chooses; never pick silently and never present a single option as
though it were the only one.

Options that turn on an external dependency get rule 4 applied first — resolve
the version, then present the option.

### 4 — Write the draft

Write `plans/<objective-slug>/draft-objective.md` using exactly this shape.
Derive the slug from the objective, kebab-case.

```markdown
# Objective: <one line>

## What I understood
<The problem in your words, with enough detail that a wrong reading is
obvious to the user. Not a restatement of their sentence.>

## Assumptions I made
- <each judgment call you made without asking, and why you made it>

## What I don't know
- <each open question, including gaps `explore` reported>

## Objective
<What must be true when this is done. One paragraph.>

## Constraints
- <technical, product, or process limits that bind the work>

## Out of scope
- <what this objective explicitly does not cover>

## Success criteria
- <observable statements. Each one must be able to fail.>

## Orientation
<Areas likely involved, known risks, a tentative order of attack.
This is a sketch to steer by, not a contract. Say so in one line.>

## Sources
- <what you read or resolved; library versions with where you checked>
```

Two sections carry most of the value. **Assumptions** is what makes "don't
assume anything" auditable instead of aspirational. **Out of scope** is what
stops the work from quietly growing — it is worth pushing for two or three
entries even when the user shrugs.

### 5 — Hand the draft over and wait

> Escribí el objetivo en `plans/<slug>/draft-objective.md`. Abrilo y leelo
> entero — sobre todo **Assumptions I made** y **What I don't know**, que es
> donde está lo que decidí sin preguntarte. Corregí directo en el archivo lo
> que esté mal y avisame cuando esté listo.

Then stop. Do not proceed on silence, and do not re-explain the draft's
contents in chat — that defeats the point of having written it.

When they come back, re-read the file: they may have edited it, and their edits
are the decision.

### 6 — Commit it to memory

Only once the user approves. The draft is a negotiating surface; memory is the
record.

```bash
# The objective itself.
$MEMORY add --plan <slug> --kind objective \
  --title "<objective one-liner>" \
  --body "<Objective + Constraints + Out of scope + Success criteria>" \
  --about <area>,<area>

# One record per surviving assumption — these are open until something answers
# them, and they are what a later verification pass is allowed to challenge.
$MEMORY add --plan <slug> --kind assumption --title "..." --about <area>

# One record per open question.
$MEMORY add --plan <slug> --kind open-question --title "..." --about <area>

# Any decision the conversation settled, especially a chosen option.
$MEMORY add --plan <slug> --kind decision --title "..." \
  --body "<why this over the alternatives>" --about <area>
```

Tag `--about` with the areas and paths a future session would search by. A
record nobody can find by filtering is a record nobody will read.

Leave the draft file on disk — it is the human-readable form of what was
agreed, and the loop will want to look at it.

### 7 — Hand off

Tell the user what was recorded and point at the next step:

> Objetivo registrado (`<id>`), con N suposiciones y M preguntas abiertas.
> Cuando quieras arrancar, corré `work` — define la primera unidad contra este
> objetivo y te la pasa para aprobar antes de tocar código.

If the whole job plainly fits in one unit, say so and offer to continue into
`work` in this same session. Small work takes the same path, just once.

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
