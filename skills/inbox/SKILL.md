---
name: inbox
description: "Say what is waiting on the user across every repository on this host, and let them act on it from here without opening a session in that repository. Lists it with `memory state --all --text` — the objectives waiting on their approval, the ones verified and waiting on what the user owes, the ones blocked on a question, the ones waiting to be started, the Ideas section and the adjustments the user asked for — exactly as the CLI prints it, inventing no class of its own. Explains any one item from its draft, its `manual-checks.md` and its open records. Closes a `verified` objective — a handed-over check, or a landing that has not happened on an executable objective or on the parent a tree of slices lands through — from the user's own sentence: finds the objective, shows the `owed` record (or, on an objective blocked before `verified` existed, the owed questions) that sentence settles and what state it will write, and on their go writes one `decision` quoting them into that objective's own log — `met` when it worked, a fresh `blocked` on a new question when it failed, `cancelled` when they drop it, withdrawing what it owed, `separated` when it is not for now. Notes an idea in one line into the repository they name, or into the user log when they name none, and drops one with a `decision` that withdraws it. Notes a change the user asks for, or a 'why does it work like this?' they want revisited, as an `adjustment` in the repository it is about, deriving from the objective it touches when there is one, and closes one with a `decision` quoting them that answers or withdraws it; pointing at an existing question is not a response to it. Use when somebody wants to know what they owe, or to report what happened with something they owed. Triggers on: 'qué tengo pendiente', 'qué me está esperando', 'qué hay pendiente', 'qué me falta cerrar', 'lo desplegué y funciona', 'ya lo probé y anda', 'lo corrí y falló', 'eso ya no va', 'eso para más adelante', 'idea:', 'anotá esta idea', 'descartá esa idea', 'what is waiting on me', 'what do I have pending', 'I deployed it and it works', 'I ran the check and it passed', 'it failed', 'drop that one', 'not now', 'note this idea', 'drop that idea', 'quiero que esto cambie', 'anotá este cambio', 'ese ajuste ya está', 'I want this changed', 'note this change', 'that adjustment is done'. It never writes a word the user did not say, never runs a handed-over check itself and never approves an objective for execution — that is `work`'s, in the repository the objective lives in."
---

# inbox

Read [the host runtime](references/runtime.md) first for tool invocation,
delegation, repository paths and the memory prerequisite.

Tell the user what is waiting on them, wherever it is, and take what they say back
about it onto the record in the log it belongs to.

Two things go missing when work spreads over many repositories and many sessions,
and neither is the state of the work — the log already holds that, and
`memory state --all --text` already reads it across every log on this host. What
goes missing is **a cheap place to close what is genuinely theirs**, and **a place
for an idea at all**.

A check `verify` handed over, or the commit, merge or deploy a `## Landing
condition` declares, holds its objective open until somebody reports the result.
The landing is never a criterion and nobody grades it: it waits on an executable
objective that `work` closed `verified` because it had not happened, or on the
parent a tree of slices lands through, which moves to `verified` when the
parent's last open child closes in any state but `verified` or `blocked` — by
`record` when `work` closes that child, by this skill when the user closes it
here. Either way the objective points at an `owed` record naming what the user
owes, and it is listed apart from anything blocked. Reported inside the session
that left it, that is one line. Reported a week later it means finding that
session, which nobody does, so the objective sits `verified` long after the thing
it waits on went fine.
This skill is the other place to say it: one sentence, here, and the close is
written into that objective's own log with the sentence quoted on it.

And a thought about something to do some day has had nowhere to go but an
objective, which means planning it. An `idea` is one line in a log, listed beside
everything else that waits, until an objective takes it up or a decision drops it.

A change the user asks for, or a "why does it work like this?" they want
revisited, is not an idea and not a question to be pointed at: it is an
`adjustment`, listed as pending until a decision says it was done, explained and
accepted, or withdrawn.

There is **no state above `met` here, and no queue of things delivered but not yet
confirmed beyond what a landing condition asked for.** `met` means whatever the
objective's `## Landing condition` declared. Where the user wanted the deploy on
the record, the landing condition already put it there as the `owed` record a
`verified` objective waits on, or a `blocked` one carries beside its wall, which
this skill closes; where they did not, there
is nothing for them to come back and confirm, and a list of such things would
fill with records that are almost all already deployed or already dead.

## Hard rules

1. **Nothing is written without the user's words.** A close, a drop, a "not now",
   an idea and an adjustment are each written because the user said so in this conversation,
   and every `decision` this skill writes quotes them verbatim. Not because the
   listing makes something look finished, not because a commit landed, not because
   a draft reads as delivered.
2. **What a close will write is shown before it is written, and the user says go.**
   The objective, the questions it answers or withdraws, the state edge and the
   quote — in front of them, then the write. Their sentence says what happened;
   the go says this is the objective and these are the questions it happened to.
3. **Never write `approved_for_execution`.** Approving an objective, building it
   and verifying it are `work`'s, run in the repository the objective lives in.
   An item waiting on approval or waiting to be started is explained here and
   pointed at `work`; it is never moved from here.
4. **Never run a handed-over check.** The scripts behind a `manual-checks.md` are
   the user's to run because only they can see what those scripts are pointed at.
   Explaining one is reading its instructions aloud, not executing them.
5. **The listing is the CLI's, and this skill invents no class.** Its sections
   are presented as they come back. No "delivered but unconfirmed", no ranking by
   age, no bucket this skill derived.
6. **Commit counts are not evidence of an item's state on their own.** In a busy
   repository they measure how active the repository is, not how far an objective
   got (`01M3K54KA3RNRPT9V303YNK4ZT`, 2026-09-28).
7. **Every write lands in the log its record belongs to.** A close goes into the
   objective's own log under the objective's own plan; an idea goes into the log
   of the repository it names, or the user log when it names none, with no plan;
   an adjustment goes into the log of the repository it is about, under the plan
   of the objective it touches or with no plan when it touches none.
   Never the log of whatever repository this session happens to be standing in.
8. **A batch file is written with the file-writing tool, never a heredoc, and a
   write is judged by its exit status, then read back.** The user's words go into
   that file verbatim, and a heredoc is a shell reading them first. A refused
   batch still prints JSON, so the text of the output says nothing about whether
   anything landed.
9. **`ask` is for how something works, and only when the user asks that.**
   Explaining an item is its draft, its instructions and its records; the code is
   read only when the question is about the code.
10. **Conversation in the user's language; everything written to disk in
    English** — except the user's own words, which are quoted exactly as they said
    them.

## The memory log

State lives in an append-only log, read and written only through the `memory`
CLI. Never edit the log by hand.

This skill reads across every log and writes into one at a time, so the log a
write goes to is always named on the command: `--dir <root>` for a repository's
log, `--scope user` for the user log. `<root>`, `<objective-id>`, `<plan>` and the
other bracketed names below are placeholders for values a previous command printed
— **paste each one in literally**. Each command runs in its own shell and nothing
carries a value forward, and a placeholder left in is a shell redirect that fails
loudly rather than a `--dir ""` that silently writes into whichever repository the
shell is standing in. Leave the brackets unquoted for that reason.

Where the ids and roots come from:

- **`memory state --all --text`** names every open objective with its log and its
  id, every idea with its log and its id, and every open adjustment the same way.
- **`memory state --all --id <objective-id>`** returns that one objective from any
  log, with its `root` (the working tree its log belongs to), its `slug`, its
  `plan`, its `state` and its `open_records` — the `open-question`, `assumption`,
  `adjustment` and `owed` records that derive from it and that nothing has
  settled. Settled means answered, superseded or withdrawn, wherever this skill
  says it.
- **`memory state --all`** is the same listing as JSON. `current` carries one
  entry per log with its `repo` and `root`, which is how a repository the user
  names becomes a `--dir`; `ideas` carries every unsettled idea and `adjustments`
  every unsettled adjustment, each as `{"root","repo","id","title"}`; the user
  log's carry `"repo": "user"` and `"root": ""`, since that log has no working
  tree. A single-log `memory state`, without `--all`, carries `root`, `id` and
  `title` on each and no `repo`, which is why this skill reads `--all`.

```bash
memory state --all | python3 -c 'import json, sys
d = json.load(sys.stdin)
for c in d["current"]:
    print("log", c["repo"], c.get("root", ""), sep="\t")
for i in d.get("ideas", []):
    print("idea", i["repo"], i.get("root", ""), i["id"], i["title"], sep="\t")
for a in d.get("adjustments", []):
    print("adjustment", a["repo"], a.get("root", ""), a["id"], a["title"], sep="\t")'
```

The surface is twelve commands and this skill uses five of them: `state`, `add`,
`link`, and `edges` and `query` to find and read the `owed` record a `verified`
objective waits on — `query` also reads the `owed` a `blocked` objective lists in
its `open_records`, and an adjustment whole before closing it. `memory kinds` stays worth running for which record kind each
state edge accepts: `met` a `decision` or a `review-finding`, `verified` an
`owed` alone, `blocked` an `open-question` or a `decision`, `cancelled` and
`separated` a `decision` alone. It also lists `withdraws`, the edge this skill
writes from a `decision` to a record it retires without answering — an `owed`,
an `open-question`, an `idea` or an `adjustment`, never an objective — with the
reason on a `Reason: obsolete` or `Reason: no-longer-applies` line of that
decision's body.

## Steps

### 1 — List what is waiting

```bash
memory state --all --text
```

Show it as it came back, in a code block, then one or two lines in the user's
language saying what they can do from here: have any one item explained, report
what happened with one, drop one or set it aside, note an idea, note a change
they want or close one they asked for. Do not re-sort it,
do not collapse **Nothing waiting**, and do not annotate an item with anything the
listing does not carry.

**Verified, waiting on what you owe** is where handed-over checks and landings
show up: each group is headed by the `owed` record the objectives under it wait
on, whose title says on one line what is owed — headed the same way whether the
objective under it is an executable one or a parent whose children have all
closed. Each of them is one the user can close from here, step 3.
**Blocked on a question or a decision** is for what stops the work: a question
nobody has answered, a wall outside it. An objective blocked on a wall can owe a
landing or a handed-over check as well, in an `owed` record `record` wrote beside
the wall and its `open_records` carry; step 2 explains it and step 3 closes it,
and the wall stays what the objective is blocked on. It also still holds objectives closed
before `verified` existed, blocked on an `open-question` titled "Nobody reported a
result for the handed-over criterion: …" or "The landing condition has not
happened: …"; those are owed too, nothing migrated them, and step 3 closes them
the way it always has.
**Ideas** lists every idea no record has settled (answered, superseded or
withdrawn), each with the name of the log it lives in — `user` for the user log —
and its id.
**Adjustments you asked for** lists every `adjustment` nothing has settled, one
line each, the way Ideas does: the changes the user asked for, and the "why does
it work like this?" they wanted revisited, until a decision closes them, step 7.

When the user opened with a sentence about one item instead of a question about
all of them — "lo desplegué y funciona" — this listing is still the first read,
because it is what the objective is resolved against. Go on to step 3 with it.

### 2 — Explain one item

Take the id off the listing, and read the objective whole:

```bash
memory state --all --id <objective-id>
```

Its draft is found exactly the way `work` finds it: under `root`, in the
directory named by the objective's own `slug` — not by `plan`, which names the
tree — in both locations, the new one first:

```bash
sh -c 'for p in <root>/.ai-kit/plans/<objective-slug>/draft-objective.md <root>/plans/<objective-slug>/draft-objective.md; do
  [ -f "$p" ] && { echo "$p"; exit 0; }
done; exit 1'
```

Exit 0 printed the path; **exit 1 is the no-draft answer and exit 2 is not an
answer at all** — it is an unsubstituted placeholder the shell refused. The
handed-over instructions sit beside the draft, looked for the same way:

```bash
sh -c 'for p in <root>/.ai-kit/plans/<objective-slug>/manual-checks.md <root>/plans/<objective-slug>/manual-checks.md; do
  [ -f "$p" ] && { echo "$p"; exit 0; }
done; exit 1'
```

When its `state` is `verified`, what it owes is written in the `owed` record its
latest `verified` edge points at. The edge names it:

```bash
memory edges --type verified --from <objective-id> --limit 1
```

and its `to` is the record, read whole out of the objective's own log:

```bash
memory query --dir <root> --id <owed-id>
```

When its `state` is `blocked`, no `verified` edge names what it owes: a `blocked`
close that also left a landing or a handed-over check unreported wrote the `owed`
beside the wall with no state edge on it. It is the entry of kind `owed` in
`open_records`, and its `id` there is the `<owed-id>` the same `memory query`
reads whole. A `blocked` objective with no `owed` in `open_records` owes nothing
through one — what it owes, if anything, is the old-titled questions step 1
names.

Then explain it from those sources and nothing else, saying which one each part
came from:

- **From the draft**: what it is for (`## What I understood`, `## Deliverable`),
  what `met` means for it (`## Landing condition`, which is never one of the
  criteria and is owed on its own), and which acceptance criteria carry `you-run`
  — those are the ones handed to the user. A parent's draft has `## Success
  criteria` instead, with no fidelity on any of them, so a parent hands nothing
  over through its criteria: its landing is the one thing it can be waiting on.
- **From `manual-checks.md`**, when there is one: what each check proves, how to
  run it, what a pass looks like, and what it touches when it runs. Read out, never
  run (rule 4).
- **From the `owed` record**, on a `verified` objective or a `blocked` one that
  lists one: each debt its body names, one per line — the landing condition, each
  handed-over criterion nobody reported on. That is the whole of what the user
  owes it; on a `blocked` objective the wall is not among it — it is what stops
  the work, not a result the user owes.
- **From `open_records`**: the question the objective is blocked on first, then the
  other open questions and any open adjustment by title. Assumptions are named by
  count and listed only when asked — an objective can carry twenty, and none of
  them is something the user owes. An unsettled `owed` record shows there too; it
  is the one read above, not a second debt.

What cannot be established from those — no draft, no instructions where a
criterion says there should be some — is said as a gap, in one line. An objective
owing a handed-over criterion with no `manual-checks.md` beside its draft is
worth naming, because it means the user was handed a check with nothing written
down about how to run it. One owing its landing alone — an executable objective
or a parent — has no instructions to miss: the `## Landing condition` is the
whole of what it owes.

When the user then asks how something in that repository works — the route the
policy gates, what the harness actually sends — that is `ask`: invoke it through
the Skill tool (`ai-kit:ask`), with the question and `<root>` as the repository
it is pointed at. Not before they ask. When what they say is that it should work
otherwise, or that the reason it works this way should be revisited, that is not
a question to answer or an old question to point at: it is an adjustment, step 7.

### 3 — Close a handover or a landing from the user's words

#### Resolve the objective

Match what they said against the open objectives in step 1's listing — the
repository they named, the words of the title, the check they described. One
match and you have it. **Two or more and you ask**, with one `AskUserQuestion`
naming each candidate by log and title; a guess here puts their words on the wrong
objective's record, and nothing in an append-only log takes that back. None, and
you say so and ask which one they meant.

One sentence that reports on two objectives is two closes, each shown and
confirmed on its own.

An objective that is waiting on approval or waiting to be started has handed
nothing over: its criteria have not been run by anybody. Say so, and point at
`work` in `<root>` — this skill does not close work that never ran. A parent with
a child still open is not waiting on its landing yet: it moves to `verified` on
the landing only when its last open child closes in any state but `verified` or
`blocked`, by `record` or by this skill's own close, so there is no `owed` record
for the user's words to settle. Say so, and write nothing.

#### Read what it still owes

Read it as step 2 does — `memory state --all --id <objective-id>`, then its draft
and its `manual-checks.md` when it has one — and keep `root`, `slug`, `plan` and
`state` in hand.

**A `verified` objective owes what its `owed` record names**, found and read with
the two commands in step 2; keep its id in hand. On a parent it names the landing
alone, and it derives from the parent itself. Any `open-question` in
`open_records` titled the old way below that the same sentence settles is owed
too, and is settled on the same line.

**A `blocked` objective with an `owed` in `open_records` owes what that record
names**, read with the `memory query` in step 2; keep its id in hand. `record`
wrote it beside the wall on a `blocked` close that also left a landing or a
handed-over check unreported. The question the objective is blocked on — the one
step 1's listing shows it under — is that wall: it is not owed, and nothing this
skill writes answers it.

**A `blocked` objective closed before `verified` existed owes its owed
questions**, and has no `owed` in `open_records` — that is how the two are told
apart. They are picked out of `open_records`: the one the objective is blocked on,
and any other `open-question` naming a handed-over criterion or the landing
condition — `record` titled them "Nobody reported a result for the handed-over
criterion: …" and "The landing condition has not happened: …". The landing
question is not a handed-over criterion and stands on its own. On a parent it is
the only owed question there is, and it derives from the parent itself, so the
parent's `open_records` carry it exactly as an executable objective's do.

Assumptions and questions about design are not owed and this skill does not touch
them.

Of those, keep the ones the user's sentence actually speaks to. "Desplegué la
policy de rutas y anda" settles a deploy; it does not settle a browser test the
same objective also handed over. When it is unclear which checks their words
cover, ask rather than rounding up.

Then decide what the write is, from their words and the draft's
`## Landing condition`. On a `verified` objective, or one blocked on owed
questions:

- **It worked, and nothing the objective owes is left.** Every debt its `owed`
  record names, or every owed question, is among what their words settle, and the
  landing condition has happened — in their words, or already when the work was
  delivered. One `decision` quoting them, an `answers` edge on the `owed` record
  or on each question, and `met` after them. On a `verified` objective the `met`
  is what moves it: answering its `owed` record does not release `verified` on its
  own, and without the `met` the objective stays `verified` with its debt settled.
- **It worked, and something is still owed** — another handed-over check they did
  not speak to, or a landing condition above what they reported. On a `verified`
  objective this is not written here: say what its `owed` record still names that
  their words did not settle, and write nothing until they report the rest. On an
  objective blocked on owed questions, the `decision`
  answers what they settled and carries no `met`. When the question the objective
  is blocked on is one this write answers, the objective gets a fresh `blocked`
  edge to what is still owed: to the remaining `open-question` with `memory link`
  after the batch, or, when what is owed is a landing condition no question names
  yet, to a new `open-question` on a second line of the same batch. Answering a
  blocking question releases the block by itself: without the fresh edge the
  objective falls back to the state it had before it was blocked — waiting for
  you to start it if it had been approved, waiting for your approval if it never
  was — not on what is still owed. A parent falls back further: with the block
  released and its children all closed, it reads `met` derived from them, so the
  fresh edge is what keeps a parent waiting on the rest of its landing.
- **It failed.** The `decision` answers the `owed` record or the question with
  the failure, and the objective gets a fresh `blocked` edge to a new
  `open-question` naming what failed,
  on the second line of the same batch. The objective stays open and the listing now
  says what it is really waiting on — a fix, not a run.

On a `blocked` objective that owes through an `owed` record, the wall is still
there whatever the user reports, and no write here takes the objective out of
`blocked`:

- **It worked, and nothing its `owed` names is left.** One `decision` quoting
  them, an `answers` edge on the `owed`, and no state edge: no `met`, because the
  wall still stops the work and a `met` would close the objective over it. The
  `blocked` edge points at the wall question, not at the `owed`, so answering the
  `owed` leaves it standing: the objective stays `blocked` on its wall, owing
  nothing.
- **It worked, and something is still owed** — the same as on a `verified`
  objective: say what the `owed` still names that their words did not settle,
  and write nothing until they report the rest.
- **It failed.** The `decision` answers the `owed` with the failure, as on a
  `verified` objective, and a new `open-question` naming what failed goes on the
  second line of the same batch with a `blocked` edge from the objective: the
  block is re-pointed at the failure. The latest `blocked` edge is the one the
  objective reads, so it is listed blocked on the failure; the wall question stays
  open and unanswered, and its own `blocked` edge stays in the log. When a fix
  answers the failure question, that block is released and the objective falls
  back onto the edge to the wall, still `blocked` while the wall stands. Left
  without the `blocked` edge, the failure would be a question nothing waits on,
  and clearing the wall would let the objective fall back past it.

Dropping the objective and setting it aside are step 4.

#### Show what will be written, and wait

In the user's language, before any file exists:

> Voy a cerrar `<repo>` — `<objective title>` (`<objective-id>`) como `met`.
> Responde a:
> - `<owed-id or question-id>` — `<its title>`
>
> Con tus palabras: "<their sentence, verbatim>". ¿Lo escribo?

For a failure, the line says `blocked` and names the new question's title. For
something still owed, it names what the objective will be left blocked on. On a
`blocked` objective that owes through an `owed` record, it says it stays
`blocked`, and on what:

> Voy a responder, en `<repo>` — `<objective title>` (`<objective-id>`):
> - `<owed-id>` — `<its title>`
>
> Sigue `blocked` en "<the wall question's title>": eso no lo resuelve esto.
> Con tus palabras: "<their sentence, verbatim>". ¿Lo escribo?

For a failure there, the last line before the quote names the new question and
says the objective will be blocked on it, with the wall still open behind it. When
the write is `met`, `cancelled` or `separated` and the objective has a `parent`,
one more line says what follows it:

> Después sigo hacia arriba: si era el último hijo abierto de un padre que aterriza
> con un commit, merge o deploy, lo dejo en `verified`, esperando eso.

Nothing
is written until they say go; a correction — the wrong question, a check they did
not mean — changes the list and gets shown again.

#### Write it as one batch

Write the batch file with the file-writing tool, under the objective's own
directory, at `<root>/.ai-kit/plans/<objective-slug>/inbox-<UTC timestamp>.jsonl`.
That directory is ignored by the repository's git, so the file leaves nothing in
its `git status`; the legacy `<root>/plans/` is not ignored and never gets one. A
new name every time, because an earlier batch is a record of what was written and
is not overwritten.

It worked, nothing left, on a `verified` objective:

```jsonl
{"kind":"decision","title":"Owed reported done: <what they ran or did, in one line>","body":"The user's words: \"<verbatim>\"\nSettles: <the owed id> — <the owed title>.\nLanding condition: <as the draft states it>.\nOutcome: met.","about":["handover","<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"answers","to":"<owed-id>"},{"type":"met","from":"<objective-id>"}]}
```

`answers` comes before `met` in the list, and a batch writes a line's edges in
the order they are listed, so `met` lands on a later line of the log than the
`answers` and than the `verified` it replaces. An old-titled owed question the
same sentence settles takes one more `answers` edge on this line, before `met`.

It worked, nothing left, on an objective blocked on owed questions:

```jsonl
{"kind":"decision","title":"Handover reported done: <what they ran or did, in one line>","body":"The user's words: \"<verbatim>\"\nSettles: <each question id and title>.\nLanding condition: <as the draft states it>.\nOutcome: met.","about":["handover","<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"answers","to":"<question-id>"},{"type":"met","from":"<objective-id>"}]}
```

One `answers` edge per settled question. When something is still owed, the same
line without the `met` edge and with `Outcome:` saying what is left.

The same lines close a parent's landing, with the parent's id as `<objective-id>`
and, as `<owed-id>`, the `owed` record the walk to the ancestor that lands wrote
on it — or, on a parent blocked before `verified` existed, the landing question
`record` wrote on it as `<question-id>`. Its `met` edge is written after the last
child closed, so `memory state` reads it as the parent's own state rather than
one derived from its children.

It worked, nothing left, on a `blocked` objective that owes through an `owed`
record — no state edge, so the objective stays blocked on its wall:

```jsonl
{"kind":"decision","title":"Owed reported done while blocked: <what they ran or did, in one line>","body":"The user's words: \"<verbatim>\"\nSettles: <the owed id> — <the owed title>.\nLanding condition: <as the draft states it>.\nOutcome: still blocked on <the wall question id> — <its title>; nothing is owed.","about":["handover","<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"answers","to":"<owed-id>"}]}
```

It failed:

```jsonl
{"kind":"decision","title":"Handover reported failed: <the check>, <how it failed>","body":"The user's words: \"<verbatim>\"\nSettles: <each question id and title> — it was run, and it failed.\nOutcome: blocked on the open question naming the failure.","about":["handover","<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"answers","to":"<question-id>"}]}
{"kind":"open-question","title":"<the check> failed when the user ran it: <the failure, in one line>","body":"Reported through inbox: \"<verbatim>\"\nThe check: <the criterion or landing condition, as the draft states it>; instructions at <the manual-checks.md path, or none for a landing>.\nWhat it needs is a fix, then a run.","about":["handover","<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"blocked","from":"<objective-id>"}]}
```

On a `verified` objective and on a `blocked` one that owes through an `owed`
record, `<question-id>` in the first line is the `<owed-id>`, and `Settles:`
names it. On the `blocked` one, `Outcome:` also names the wall question, still
open behind the new one.

`answers` and `derives_from` are spelled `"to"` because this line's record is
their source; `met` and `blocked` are spelled `"from"` because the objective is
theirs and this line's record is what justifies them.

Then the one command, into the objective's own log and plan:

```bash
memory add --dir <root> --plan <plan> --batch <root>/.ai-kit/plans/<objective-slug>/inbox-<UTC timestamp>.jsonl
```

A batch is one write or none, so a non-zero exit means nothing landed: show the
CLI's error as it came back and stop, rather than editing the file until something
passes. The first write into another repository's log may raise a permission
prompt; that is the host's call, and it is not routed around.

When the case above needs it — the objective was blocked on a question this batch
answered, and the one still owed already exists — the fresh edge is one more run:

```bash
memory link --dir <root> --type blocked --from <objective-id> --to <the remaining question id>
```

#### Confirm it landed

```bash
memory state --all --id <objective-id>
```

Read `state` out of this, not out of the exit status alone. It should be the state
you showed them: `met`, or `blocked` on the question you named — on a `blocked`
objective that owed through an `owed` record and reported it done, still
`blocked` on its wall. The questions and the `owed` this write answered should be
gone from `open_records`, and on a failure the new question should be on it. A `verified` objective that went `met` is gone from
**Verified, waiting on what you owe** in the next `memory state --all --text`.
Say it in one line — the objective, its state now, and what is still open on it.
The assumptions `plan` recorded stay listed after a `met`; that is what an
objective closes with, not something the close missed.

An objective that still reads as it did before, after an exit 0, is a close that
did not land: say so with what the command printed, and write nothing more.

#### Then the ancestor that lands

A close written here can be the last one a tree was waiting on. When this batch
wrote `met`, `cancelled` or `separated` and the read-back shows it, the objective
may have been the last open child of a parent whose draft declares a landing, and
`memory state` derives that parent `met` the moment it is, reading no draft.
`record` walks up after every close that is neither `verified` nor `blocked` for
exactly this; a close written here goes through no `record`, so this skill runs
the same walk. A
batch that left the objective `blocked` — a failure, or something still owed —
keeps it outstanding on its parent, and there is nothing to walk.

Follow "After the close: the ancestor that lands" in `agents/record.md`, resolved
against the ai-kit clone as the runtime reference says, with the objective this
batch just closed as its `<objective_id>`. It holds here as written: the same
draft lookup, the same reading of the ancestor's `open_records` in
`memory state --all --id <ancestor-id>` for an `owed` nothing has answered,
superseded or withdrawn, the same one `owed` line with its `verified` edge in
`landing-from-<objective-id>.jsonl` beside the ancestor's draft when it has none
— written with the file-writing tool, rule 8 — the same
`memory add --dir <root> --plan <plan> --batch …` with the ancestor's own `root`
and `plan`, and the same rules for an ancestor that already has one: pointed at
it with `memory link --dir <root> --type verified` rather than given a second
when its body carries every line of the draft's `## Landing condition`, and
given a new `owed` superseding it when it does not.
That `owed` record quotes nobody and settles nothing; it is the consequence of
the close the user already said go to, announced in the line shown before it.

Add to the one line of the read-back the ancestor the walk left `verified`, by
log, title and id, and the condition it now waits on, read from its own
`memory state --all --id <ancestor-id>` rather than from the exit status. A walk
that stopped before any ancestor that lands wrote nothing and adds nothing to the
line; one that stopped on an ancestor with no draft says so, naming it.

### 4 — Dropped, or not now

"Eso ya no va" — the objective is not going to happen — closes `cancelled`.
"Eso para más adelante" — the work is real but not under this parent now — closes
`separated`. Both point at a `decision`, both are resolved, shown and confirmed
exactly as step 3 does, and both are one line written to the same place:

```jsonl
{"kind":"decision","title":"Dropped: <objective title>","body":"The user's words: \"<verbatim>\"\nWithdraws: <each owed id and title>.\nReason: no-longer-applies\nOutcome: cancelled.","about":["<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"withdraws","to":"<owed-id or question-id>"},{"type":"cancelled","from":"<objective-id>"}]}
{"kind":"decision","title":"Set aside for now: <objective title>","body":"The user's words: \"<verbatim>\"\nOutcome: separated.","about":["<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"separated","from":"<objective-id>"}]}
```

One of those lines, never both. A dropped objective's `owed` record and owed
questions will never be answered: the work they wait on is not going to happen.
So its decision withdraws each of them, one `withdraws` edge apiece before the
`cancelled` edge, with `Reason: no-longer-applies` in its body — never an
`answers`, which would assert a result nobody reported. An objective that owes
nothing gets neither the `Withdraws:` and `Reason:` lines nor a `withdraws` edge.
The list shown before the write names them under "Retira:" instead of "Responde
a:". A set-aside objective's stay open,
because the work they wait on is still coming. Then the same
`memory add --dir <root> --plan <plan> --batch …`, the same read-back — the
withdrawn records gone from `open_records` — and the same walk to the ancestor
that lands.

### 5 — Note an idea

"Idea: un resumen de lo pendiente cada mañana." Write it as it came: a title of
one line, in English, saying the idea; a body only when they said more than fits
in one. No priority, no due date, no criteria and no plan — an idea that needs
those is an objective, and that is `plan`'s.

It goes into the log of the repository it names. Find that repository's `root` in
the `log` lines of the listing command above; a name that matches two logs is a
question for the user, and a repository that has no log yet is given by its path
and checked with `git -C <the path> rev-parse --show-toplevel` before anything is
written there.

```bash
env -u AI_KIT_PLAN memory add --dir <root> --kind idea --about idea,<area> --title "<the idea, one line>" --body "<what else they said>"
```

An idea that names no repository goes into the user log:

```bash
env -u AI_KIT_PLAN memory add --scope user --kind idea --about idea,<area> --title "<the idea, one line>"
```

`env -u AI_KIT_PLAN` is what keeps it plan-free: `add` stamps `AI_KIT_PLAN` from
the environment when no `--plan` is given, and an idea belongs to no objective
tree. `--about` is required by `add`, and for an idea it is always `idea`, plus
the area when the user names one — `--about idea,<area>`, or `--about idea` alone
when they name none. The sentence was the instruction, so there is no
confirmation round: check the exit status. Exit 0 and you say in one line which
log it went into and its id, and the next `memory state --all --text` lists it
under **Ideas**. Any other exit and nothing landed: show the user the CLI's error
as it came back, and write nothing in its place.

### 6 — Drop or take up an idea

**Dropping** is a `decision` that withdraws it, written into the idea's own log —
the `root` the `idea` line of the listing command printed, or the user log when
its repository is `user` and its `root` is empty. It is the same rule an `open-question` follows: an idea
leaves the Ideas section when something settles it — an answer, a `supersedes`
or a `withdraws` — and nothing else removes it. A dropped idea answered nothing,
so the decision carries a `withdraws` edge and a `Reason:` line, and no
`answers`. Write the line with the file-writing tool into this session's scratch
directory, never into a repository's tracked tree:

```jsonl
{"kind":"decision","title":"Idea dropped: <idea title>","body":"The user's words: \"<verbatim>\"\nReason: obsolete","about":["idea","<area>"],"edges":[{"type":"withdraws","to":"<idea-id>"}]}
```

`Reason:` is `obsolete` when the idea was overtaken or is simply not wanted any
more, and `no-longer-applies` when what it was for is gone; their words say which,
and `obsolete` is the one when they do not. `about` follows the same rule as the
idea's own: `idea` always, and the area only when the user names one — `["idea"]`
when they do not.

```bash
env -u AI_KIT_PLAN memory add --dir <root> --batch <scratch>/inbox-drop-idea-<UTC timestamp>.jsonl
env -u AI_KIT_PLAN memory add --scope user --batch <scratch>/inbox-drop-idea-<UTC timestamp>.jsonl
```

The first for a repository's idea, the second for the user log's; one of them,
never both. Exit status first: any exit but 0 means nothing landed, and the CLI's
error goes to the user as it came back. On 0, the next listing no longer shows it.

**Taking it up** is planning it, and this skill does not plan. Say, in one line,
to run `plan` in `<root>` with the idea as the request and its id beside it, so
the objective `plan` records can carry an `answers` edge to the idea. Until
something settles it, it stays listed — which is correct, since until then nobody
has taken it up.

### 7 — Note an adjustment, and close one

"Quiero que el listado no muestre las asunciones", "¿por qué esto bloquea si es
solo una duda? revisalo" — a change the user asks for, or a "why does it work like
this?" they want revisited, is an `adjustment`. It is not answered by pointing at
an `open-question` already in the log on the same ground: an old question says
somebody once wondered, not that anything is going to change, and citing it
leaves the user's request with no record and nothing listing it. Name the old
question if it bears on the request, and write the adjustment all the same.

Write it as it came: a title of one line, in English, saying the change they want
or the behaviour they want revisited; a body quoting them verbatim; `about` the
areas it touches. It goes into the log of the repository it is about, found the
way step 5 finds a repository's `root`. An adjustment is about something that
works a certain way somewhere, so when neither their words nor the conversation
name the repository, ask which one rather than filing it in the user log.

When it is about an objective — the one being explained, or one their words
match in step 1's listing, resolved as step 3 resolves one, with the same
`AskUserQuestion` when two match — it derives from that objective and goes under
its plan, as one line written with the file-writing tool beside the objective's
draft, at `<root>/.ai-kit/plans/<objective-slug>/inbox-adjustment-<UTC timestamp>.jsonl`:

```jsonl
{"kind":"adjustment","title":"<the change they want, one line>","body":"The user's words: \"<verbatim>\"","about":["<area>","<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"}]}
```

```bash
memory add --dir <root> --plan <plan> --batch <root>/.ai-kit/plans/<objective-slug>/inbox-adjustment-<UTC timestamp>.jsonl
```

When it touches no objective, it is plan-less, kept out of every tree the way an
idea is: the same line without the `derives_from` edge and without the slug in
`about`, written with the file-writing tool into this session's scratch
directory, since the user's words are in it (rule 8):

```bash
env -u AI_KIT_PLAN memory add --dir <root> --batch <scratch>/inbox-adjustment-<UTC timestamp>.jsonl
```

The sentence was the instruction, so there is no confirmation round beyond
resolving the objective: check the exit status. Exit 0 and you say in one line
which log it went into, its id and the objective it derives from, if any; the
next `memory state --all --text` lists it under **Adjustments you asked for**, and
the objective's `open_records` carry it. Any other exit and nothing landed: show
the CLI's error as it came back, and write nothing in its place.

**Closing one** is a `decision` quoting the user, written into the adjustment's
own log — the `root` its `adjustment` line of the listing command printed — and
only on their words about it. Match those words against **Adjustments you asked
for**; two that match are one `AskUserQuestion`, as in step 3. Two closes, one
line each, never both:

- **Done, or explained and accepted** — "ya está cambiado", "ok, entiendo por qué,
  dejalo así": the decision `answers` it, and its body says which.
- **No longer wanted** — "eso ya no hace falta": the decision withdraws it, with a
  `Reason:` line chosen as step 6 chooses one, and no `answers`.

```jsonl
{"kind":"decision","title":"Adjustment closed: <adjustment title>","body":"The user's words: \"<verbatim>\"\nOutcome: <done | explained and accepted>.","about":["<area>"],"edges":[{"type":"answers","to":"<adjustment-id>"}]}
{"kind":"decision","title":"Adjustment withdrawn: <adjustment title>","body":"The user's words: \"<verbatim>\"\nReason: <obsolete | no-longer-applies>","about":["<area>"],"edges":[{"type":"withdraws","to":"<adjustment-id>"}]}
```

Write that line with the file-writing tool into this session's scratch directory.
It goes under the adjustment's own plan, read off the record whole:

```bash
env -u AI_KIT_PLAN memory query --dir <root> --id <adjustment-id>
```

A record whose `origin` carries a `plan` is closed with the first command below,
under that plan; one whose `origin` carries none with the second; one of them,
never both:

```bash
memory add --dir <root> --plan <plan> --batch <scratch>/inbox-close-adjustment-<UTC timestamp>.jsonl
env -u AI_KIT_PLAN memory add --dir <root> --batch <scratch>/inbox-close-adjustment-<UTC timestamp>.jsonl
```

Exit status first, as in step 6. On 0, the next listing no longer shows it under
**Adjustments you asked for**, and the objective it derives from no longer carries
it in `open_records`.

## Key principles

- **The user's sentence is the record.** Every close quotes it, and nothing is
  closed that they did not say.
- **Shown, then written.** The objective and the questions go in front of them
  before the batch exists, because a close on the wrong objective is permanent.
- **Written where it belongs.** An objective's close into its own log and plan, an
  idea into the log it names, an adjustment into the log of the repository it is
  about; this session's repository is not a default.
- **Withdrawn is not answered.** What is dropped — an idea, an adjustment no
  longer wanted, what a cancelled objective owed — is retired with `withdraws` and
  a `Reason:`, never with an `answers` that claims an answer nobody gave.
- **`met` is what the landing condition said.** No state above it, and nothing
  to confirm beyond the `owed` record a `verified` objective waits on or a
  `blocked` one carries beside its wall; a deploy is
  on the record only where the objective, or the parent its slices land through,
  asked for it.
- **Read back, not reported.** An exit status says the write was accepted; `state`
  says what the objective is now.
- **Explaining is reading.** The draft, the instructions and the records answer
  what an item is; the code is `ask`'s, when asked.
