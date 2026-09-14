---
name: brief
description: "Think a piece of work through with the person who is going to write it, and hand them a map instead of a task. An open-ended design conversation — the terrain, the real options with what each one costs, pushback on the choice — that ends, when they say so and not before, in one file at `.ai-kit/plans/<slug>/brief.md` and one `objective` record in the log. The brief is a map and not a design: what we are doing, the terrain with its anchors, what was decided, what that displaced, where it touches, the manual checks they said they will run, and what is still open. Nothing in it is gradeable, because nobody is going to grade it — the person implementing is in the room and reads the code as they write it. Use only when the user says explicitly that they are writing the code themselves. Triggers on: 'lo implemento yo', 'el código lo escribo yo', 'pensemos esto que después lo hago yo', 'dame un brief', 'necesito un brief', 'brief this', 'brief me before I write it', 'I am writing this one myself'. It never writes production code and never produces something a subagent could be handed and graded against: when the agent is the one implementing, that is `plan` and then `work`, the other door and the default one."
---

# brief

Read [the host runtime](references/runtime.md) first for tool invocation,
delegation, repository paths and the memory prerequisite.

Turn a piece of work into a written map for the person who is about to implement
it, and stop there. The detailed design stays in the conversation and gets
rebuilt at the keyboard, which is where the practice is.

This is not a lighter `plan`. `plan` produces criteria a subagent can be graded
against, because a subagent is going to write the code and nobody will be
watching it — the criteria are frozen, the evidence each one demands is declared,
and a separate pass checks them. None of that machinery has anything to do with
this flow. Here the person who implements is in the room, will read every line
because they are writing it, and runs their integration checks by hand. What they
need is orientation: where the thing lives, what was decided and what that cost,
and what they said they would check when it is done.

So the deliverable is a map. Seven short sections, one file, one record. If what
you wrote could be handed to a subagent and executed, you wrote the wrong
document.

## Hard rules

1. **Never write production code.** Not a snippet meant to be pasted, not "here
   is roughly what the function looks like", not a patch offered at the end. The
   whole reason somebody entered this door is to write it themselves; a pasted
   implementation takes the practice away and leaves them reviewing your code
   instead. Reading the repository is not only allowed but the point — writing to
   it is what stops here. The one file you write is the brief.
2. **The brief is a map, not a design.** Anchors, decisions and displaced
   alternatives, in a form somebody skims while typing. Not interfaces, not
   function signatures, not a file-by-file plan. When you find yourself
   specifying, you have started doing their work in a document.
3. **Nothing in the brief is gradeable, and no part of it is frozen.** No
   numbered criteria, no evidence-strength labels, no approval that locks
   something. Those exist in the other door because code gets written unwatched
   and somebody has to check it afterwards against something that cannot move.
   Here the check is a person reading their own diff, and the checks they run are
   the ones section six names. Approval of a brief means the map is right — it
   never means the work is now measured against it.
4. **Nothing is written to disk until they say to write it.** There is no fixed
   number of rounds and no template to fill: the conversation ends when the user
   says it ends. Its counterpart is step 6 — you say out loud when you think it is
   done rather than waiting to be asked, because an open-ended conversation nobody
   offers to close is one that drifts.
5. **Options with their costs come first; your own pick comes last.** State which
   one you would take at the end, or earlier when asked, and never instead of
   presenting the alternatives. A recommendation delivered first is a
   recommendation that already closed the conversation.
6. **A library version, an SDK shape or a provider API is resolved, never
   recalled.** Your training data lags releases. If the design turns on an
   external dependency, look it up — `context7` if available, otherwise the
   pinned source in the dependency tree, otherwise the web — and cite what you
   used.
7. **One record, no edges.** The log gets exactly one `objective` record and
   nothing else: no state edge, no edge saying this was approved for execution,
   no `derives_from`. Nothing downstream reads this trail to decide what to run,
   so an edge here would be bookkeeping with no reader; the `--plan` slug is what
   joins the record to whatever else the tree holds.
8. **Conversation in the user's language; everything written to disk in
   English.**

## The memory log

State lives in an append-only log, read and written only through the `memory`
CLI. Never edit the `.jsonl` by hand.

**The write in step 8 always names its tree with `--plan`**, and so does step 1's
objective read — placing this work in a tree is a question about that tree. The
two lookups that ask what this repository already decided deliberately do not: a
`gotcha` binding the surface you are about to touch was very likely recorded
while somebody worked on a different tree, and a plan-bounded read hides it
behind a `count: 0`.

Omitting the flag is not enough by itself. The CLI then falls back to the
`AI_KIT_PLAN` environment variable — exit 0, no warning, bounded again to
whatever tree an earlier session exported — so those two calls are prefixed with
`env -u AI_KIT_PLAN`, which is the shape `prefer` uses for the same reason.
`memory kinds` lists the valid kinds and edge types if you need them.

## Steps

### 1 — Place the work, and read what is already settled

Find out whether this belongs under something that already exists:

```bash
memory state
memory query --plan <tree-slug> --kind objective --current
```

`state` takes no plan filter — it reports every objective in the log with its
parent and depth. Two names come out of this step and **they are not the same
string**:

- **The tree slug**, which `--plan` takes. Inherit it from whatever this work sits
  under, or coin one when nothing does.
- **This brief's own slug**, which names step 7's directory and step 8's
  `--slug`. Always coined from the brief's own title, never inherited: lowercase,
  every run of characters outside `a-z0-9` collapsed to a single hyphen, no
  leading or trailing hyphen, at most 60 characters, trimming any hyphen the cut
  leaves behind.

Letting the inherited tree slug serve as both is the mistake to avoid, and it
breaks twice. The brief would be written into the directory of the objective this
sits under — which already holds that objective's own draft, one directory per
objective — and the record would take a slug that already belongs to another
objective, in a log where nothing can be taken back. `work` resolves a draft from
the record's slug, so the duplicate would point a later `work` turn at the *other*
objective's draft and the criteria in it.

Then read what somebody already chose on the surface this touches. Ask which tags
exist before filtering on one, because a tag you guessed comes back empty and an
empty result is exactly what the next paragraph must not misreport:

```bash
env -u AI_KIT_PLAN memory areas --kind decision,gotcha
```

That names every area anybody has recorded a decision or a gotcha against, in
this repository's log and the user's, with which record is live in each. Take the
`--about` values from what it names, then read those areas one at a time:

```bash
env -u AI_KIT_PLAN memory query              --kind decision,gotcha --about <area> --current
env -u AI_KIT_PLAN memory query --scope user --kind decision,gotcha --about <area> --current
```

**Both lines, always**: `query` reads one scope per call and defaults to this
repo's, so a choice promoted up out of some other repository is named by `areas`
and then comes back empty from the repo-scope line alone — and it applies here
exactly as much as a local one does.

Say in one line what came back, including "nada" — a filter that ran and found
nothing is different from a read that was skipped, and a tag nobody uses is a
third thing that looks like the first two. A `gotcha` on this surface is the
highest-value thing in the log for somebody about to type: it is a constraint the
code obeys without stating it, so they cannot find it by reading.

**When the work turns on why something is failing and nobody can say why**, say
so in one sentence and stop guessing:

> Ojo: nadie sabe todavía por qué `<X>` falla. Si el diseño depende de eso,
> `debug` lo acota primero y volvemos acá con la causa.

That is a sentence, not a gate. If they want to design against a suspected cause
anyway, that is their call and it goes in the brief's open section.

### 2 — Go look, once

Launch the `explore` subagent rather than opening twenty files in this thread,
so the conversation itself keeps the context:

```
topic: <the one question that decides the shape of this work>
repo_root: <absolute path>
scope: both
```

Keep its `findings` with their `path:line` anchors intact — those anchors are
section two of the brief, and a paraphrase that drops them hands the user a
document they cannot navigate from. Its `gaps` go to the open section.

**Skip this entirely when they already told you where the thing lives.** Read
those files directly instead. Somebody who names the module is somebody who
knows the terrain, and paying for an exploration of it buys a slower first round.

**Work they have already written is read directly, not explored.** `git status
--porcelain` names the files, and the files are what you read:

```bash
git status --porcelain
```

### 3 — Put the terrain and the options on the table

Terrain first, in a handful of lines with anchors: where this lives, what calls
it, what it calls, and the one or two things about it that are not obvious from
the names. This is the half of the conversation the user cannot get faster any
other way.

Then, when there is more than one reasonable way to do it, two or three options,
each with **what it costs and what it buys** — not a list of names. Options that
turn on an external dependency get rule 6 applied before they are presented.

A recorded choice from step 1 goes **beside** the options, as context, never in
place of one:

> Opción A: consultas generadas. Opción B: un ORM. Contexto: hay una decisión
> registrada para este repo que dice consultas generadas (`01KZ…`, 2026-03-14).
> No decide esto — te la muestro para que la tengamos a la vista.

Every option that would have been presented is still presented, including the
one the recorded choice argues against.

### 4 — Push back on the choice

This is the beat that makes the conversation worth more than a careful solo
read, and it is the one an agent skips by default. When they pick, name the way
that choice fails before agreeing with it — the case it handles badly, the thing
it makes hard to change later, the cost that arrives in three months rather than
today.

Do not concede on the first objection and do not manufacture one either. Two
honest moves, and which one applies is decided by whether they answered the
failure or dismissed it:

- **They answered it** — say so plainly and move on. "Vale, con eso el problema
  no aparece" is a real outcome and it is most of them.
- **They did not** — say it once more, in one line, and then let them choose
  anyway. It is their code and their call, and the disagreement goes in the
  brief's open section rather than being relitigated.

Your own pick lands here, at the end, or earlier if they ask for it.

There is no round count. Loop back through step 3 as many times as the design
takes — a new option, a narrower question, a piece of terrain the last round
turned up. What ends this is step 6, not a counter.

### 5 — Two things only they can answer

Ask these once each, in plain words, because neither one is yours to decide.

**Does it fit one sitting?**

> ¿Esto lo escribís de una sentada, o lo partimos? Si lo partimos, te armo el
> brief de la primera parte nada más y el resto queda anotado como pendiente.

If it does not fit, write the brief for the **first slice only**, and name the
rest in one line in the open section. There is no tree here, no child objective
and no descent — that machinery belongs to the other door, and a person who is
about to spend an afternoon typing needs one map, not a hierarchy of them.

**What will they check by hand?**

> ¿Qué vas a chequear a mano cuando lo tengas escrito? Lo que me digas va al
> brief tal cual.

Write down what they answer and nothing else. **Inventing checks is the one way
this section becomes worthless**: a list of plausible-sounding verifications
nobody intends to run reads exactly like a list somebody committed to, and the
person skimming it at the end cannot tell them apart. If they name none, the
section says so in those words.

### 6 — Offer to close, then wait

Once no open decisions remain — every option has an answer, the two questions in
step 5 are answered, and nothing is left that would change what the brief says —
say so, unprompted:

> No me quedan decisiones abiertas de mi lado. ¿Lo escribo? Te dejo el mapa en
> `.ai-kit/plans/<slug>/brief.md` — el diseño en detalle lo rearmás vos en el
> teclado.

Then stop, and do not write anything on silence. They may keep going; an
open-ended conversation is allowed to continue past the point where you thought
it was done, and the offer costs one line to make again later.

### 7 — Write the brief

Only once they say to. One file, `.ai-kit/plans/<slug>/brief.md`, where `<slug>`
is the brief's own slug from step 1 and never the tree slug — under `.ai-kit/`
because that is the directory the `memory` CLI keeps out of git by itself. Seven
sections, in this order, in English:

```markdown
# Brief: <what this work is, in a line>

## What we are doing
<Two or three sentences. The outcome, not the method.>

## The terrain
<Where this lives, with `path:line` anchors. What calls it, what it calls, and
the one or two things that are not obvious from the names.>

## What you decided
<The choice, in one or two lines, and the reason it won.>

## What it displaced
<One line per rejected alternative: what it was, and what made it lose. Not a
record each — this list is the whole trail those alternatives get.>

## Where this touches
<The files and areas the work lands in. A short list, not a file-by-file plan.>

## Manual checks to run
<Exactly what they said they would check, in their terms. Nothing added.>

## What is still open
<Unresolved questions, `explore`'s gaps, a disagreement from step 4 left
standing, and — when the work was sliced — one line naming the rest.>
```

The rejected alternatives live here and nowhere else. So do the open questions.
A `decision` record per displaced option and an `open-question` record per
unknown would be a fuller trail than this flow has any reader for, and rule 7 is
what says how much trail it gets.

Then hand it over and wait:

> Escribí el mapa en `.ai-kit/plans/<slug>/brief.md`. Leelo y corregime lo que
> esté mal — sobre todo **What you decided** y **Manual checks to run**, que es
> lo que vas a usar mientras escribís. El archivo lo reescribo yo con lo que me
> digas, no hace falta que lo edites.

Corrections come back in chat and rewriting is yours. Rewrite the whole file
rather than patching it in place, then name the sections that moved and stop
there — saying what they now say in chat is how the file stops being read.

### 8 — Record it, and name `debrief`

Once the brief is right. One record, and nothing else:

```bash
memory add --plan <tree-slug> --kind objective \
  --slug "<the brief's own slug — the one the directory has, not the tree slug>" \
  --title "<what this work is, in one line>" \
  --body "Brief: .ai-kit/plans/<slug>/brief.md
<what we are doing, the decision and what it displaced, where it touches, and what is still open>" \
  --about <area>,<area>
```

The path goes in the body because the record is the thing a later session finds
by querying, and the map is the thing it then wants to read. `add` prints the
whole record; the id is at `.record.id`. Every command prints JSON on success and
on failure alike, so check the exit code rather than scanning the text.

No edge follows, by rule 7 — not a state edge, not one saying this was approved.
That is not a lock on the record: `work` picks from the open objectives in the log
and this is one of them. What the missing edges buy is an honest state — nobody
has started it — while the person who is going to walks away and writes it.

Then say what happens next, in one line, and stop:

> Queda registrado (`<id>`). Cuando tengas el código escrito, `debrief` lo lee y
> te dice qué te faltó y qué estaría mejor, con el razonamiento, sin tocar nada.

## Key principles

- **The map is the deliverable, and the design is not.** What gets written down
  is what a person cannot reconstruct while typing — anchors, the decision, what
  it cost. What they *can* reconstruct is exactly what they are here to practice.
- **Nothing here is gradeable, on purpose.** The other door's machinery exists
  because nobody watches a subagent write code. Somebody is watching this one.
- **The conversation ends when they say so, and you offer.** No round count, and
  no silent waiting either.
- **Pushback is the value.** An agent that agrees with the first choice has given
  them a slower version of thinking alone.
- **The checks are theirs, quoted, or absent.** An invented check is
  indistinguishable from a committed one, which makes the section a liability.
- **`plan` and `work` are the other door and the default one.** This one is for
  the case where the person at the keyboard is the point.
