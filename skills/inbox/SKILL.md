---
name: inbox
description: "Say what is waiting on the user across every repository on this host, and let them act on it from here without opening a session in that repository. Lists it with `memory state --all --text` — the objectives waiting on their approval, the ones blocked on a question, the ones waiting to be started, and the Ideas section — exactly as the CLI prints it, inventing no class of its own. Explains any one item from its draft, its `manual-checks.md` and its open records. Closes a handed-over check from the user's own sentence: finds the objective, shows which handed-over questions that sentence settles and what state it will write, and on their go writes one `decision` quoting them into that objective's own log — `met` when it worked, a fresh `blocked` on a new question when it failed, `cancelled` when they drop it, `separated` when it is not for now. Notes an idea in one line into the repository they name, or into the user log when they name none, and drops one with a `decision` that answers it. Use when somebody wants to know what they owe, or to report what happened with something they owed. Triggers on: 'qué tengo pendiente', 'qué me está esperando', 'qué hay pendiente', 'qué me falta cerrar', 'lo desplegué y funciona', 'ya lo probé y anda', 'lo corrí y falló', 'eso ya no va', 'eso para más adelante', 'idea:', 'anotá esta idea', 'descartá esa idea', 'what is waiting on me', 'what do I have pending', 'I deployed it and it works', 'I ran the check and it passed', 'it failed', 'drop that one', 'not now', 'note this idea', 'drop that idea'. It never writes a word the user did not say, never runs a handed-over check itself and never approves an objective for execution — that is `work`'s, in the repository the objective lives in."
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

A check `verify` handed over, or the deploy a `deployed` landing condition asks
for, holds its objective open until somebody reports the result. Reported inside
the session that left it, that is one line. Reported a week later it means finding
that session, which nobody does, so the objective sits `blocked` long after the
thing it waits on went fine. This skill is the other place to say it: one sentence,
here, and the close is written into that objective's own log with the sentence
quoted on it.

And a thought about something to do some day has had nowhere to go but an
objective, which means planning it. An `idea` is one line in a log, listed beside
everything else that waits, until an objective takes it up or a decision drops it.

There is **no state above `met` here, and no queue of things delivered but not yet
confirmed.** `met` means whatever the objective's `## Landing condition` declared.
Where the user wanted the deploy on the record, the landing condition already put
it there as a handover this skill can close; where they did not, there is nothing
for them to come back and confirm, and a list of such things would fill with
records that are almost all already deployed or already dead.

## Hard rules

1. **Nothing is written without the user's words.** A close, a drop, a "not now"
   and an idea are each written because the user said so in this conversation,
   and every `decision` this skill writes quotes them verbatim. Not because the
   listing makes something look finished, not because a commit landed, not because
   a draft reads as delivered.
2. **What a close will write is shown before it is written, and the user says go.**
   The objective, the questions it answers, the state edge and the quote — in
   front of them, then the write. Their sentence says what happened; the go says
   this is the objective and these are the questions it happened to.
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
   of the repository it names, or the user log when it names none, with no plan.
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
  id, and every idea with its log and its id.
- **`memory state --all --id <objective-id>`** returns that one objective from any
  log, with its `root` (the working tree its log belongs to), its `slug`, its
  `plan`, its `state` and its `open_records` — the `open-question` and
  `assumption` records that derive from it and that nothing has answered.
- **`memory state --all`** is the same listing as JSON. `current` carries one
  entry per log with its `repo` and `root`, which is how a repository the user
  names becomes a `--dir`; `ideas` carries every unanswered idea as
  `{"root","repo","id","title"}`; the user log's carry `"repo": "user"` and
  `"root": ""`, since that log has no working tree.

```bash
memory state --all | python3 -c 'import json, sys
d = json.load(sys.stdin)
for c in d["current"]:
    print("log", c["repo"], c.get("root", ""), sep="\t")
for i in d.get("ideas", []):
    print("idea", i["repo"], i.get("root", ""), i["id"], i["title"], sep="\t")'
```

The surface is twelve commands and this skill uses three of them: `state`, `add`
and `link`. `memory kinds` stays worth running for which record kind each
state edge accepts: `met` a `decision` or a `review-finding`, `blocked` an
`open-question` or a `decision`, `cancelled` and `separated` a `decision` alone.

## Steps

### 1 — List what is waiting

```bash
memory state --all --text
```

Show it as it came back, in a code block, then one or two lines in the user's
language saying what they can do from here: have any one item explained, report
what happened with one, drop one or set it aside, note an idea. Do not re-sort it,
do not collapse **Nothing waiting**, and do not annotate an item with anything the
listing does not carry.

**Blocked on a question or a decision** is where handed-over checks show up:
each group is headed by the record the objectives under it wait on, which for a
handover is the `open-question` `record` wrote when nobody reported the result.
**Ideas** lists every idea no record answers, each with the name of the log it
lives in — `user` for the user log — and its id.

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

Then explain it from those three sources and nothing else, saying which one each
part came from:

- **From the draft**: what it is for (`## What I understood`, `## Deliverable`),
  what `met` means for it (`## Landing condition`), and which acceptance criteria
  carry `you-run` — those are the ones handed to the user.
- **From `manual-checks.md`**, when there is one: what each check proves, how to
  run it, what a pass looks like, and what it touches when it runs. Read out, never
  run (rule 4).
- **From `open_records`**: the question the objective is blocked on first, then the
  other open questions by title. Assumptions are named by count and listed only when
  asked — an objective can carry twenty, and none of them is something the user owes.

What cannot be established from those — no draft, no instructions where a
criterion says there should be some — is said as a gap, in one line. A `blocked`
objective with no `manual-checks.md` beside its draft is worth naming, because it
means the user was handed a check with nothing written down about how to run it.

When the user then asks how something in that repository works — the route the
policy gates, what the harness actually sends — that is `ask`: invoke it through
the Skill tool (`ai-kit:ask`), with the question and `<root>` as the repository
it is pointed at. Not before they ask.

### 3 — Close a handover from the user's words

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
`work` in `<root>` — this skill does not close work that never ran.

#### Read what it still owes

Read it as step 2 does — `memory state --all --id <objective-id>`, then its draft
and its `manual-checks.md` — and keep `root`, `slug`, `plan` and `state` in hand.
Then pick out of `open_records` the **handed-over questions**: the one the
objective is blocked on, and any other `open-question` naming a handed-over
criterion or the landing condition — `record` titles them "Nobody reported a
result for the handed-over criterion: …" and "The landing condition has not
happened: …". Assumptions and questions about design are not handovers and this
skill does not touch them.

Of those, keep the ones the user's sentence actually speaks to. "Desplegué la
policy de rutas y anda" settles a deploy; it does not settle a browser test the
same objective also handed over. When it is unclear which checks their words
cover, ask rather than rounding up.

Then decide what the write is, from their words and the draft's
`## Landing condition`:

- **It worked, and nothing the objective owes is left.** Every handed-over question
  is among the ones their words settle, and the landing condition has happened —
  in their words, or already when the work was delivered. One `decision` quoting them, an
  `answers` edge on each question, and `met`.
- **It worked, and something is still owed** — another handed-over check they did
  not speak to, or a landing condition above what they reported. The `decision`
  answers what they settled and carries no `met`. When the question the objective
  is blocked on is one this write answers, the objective gets a fresh `blocked`
  edge to what is still owed: to the remaining `open-question` with `memory link`
  after the batch, or, when what is owed is a landing condition no question names
  yet, to a new `open-question` on a second line of the same batch. Answering a
  blocking question releases the block by itself: without the fresh edge the
  objective falls back to the state it had before it was blocked — waiting for
  you to start it if it had been approved, waiting for your approval if it never
  was — not on what is still owed.
- **It failed.** The `decision` answers the question with the failure, and the
  objective gets a fresh `blocked` edge to a new `open-question` naming what failed,
  on the second line of the same batch. The objective stays open and the listing now
  says what it is really waiting on — a fix, not a run.

Dropping the objective and setting it aside are step 4.

#### Show what will be written, and wait

In the user's language, before any file exists:

> Voy a cerrar `<repo>` — `<objective title>` (`<objective-id>`) como `met`.
> Responde a:
> - `<question-id>` — `<question title>`
>
> Con tus palabras: "<their sentence, verbatim>". ¿Lo escribo?

For a failure, the line says `blocked` and names the new question's title. For
something still owed, it names what the objective will be left blocked on. Nothing
is written until they say go; a correction — the wrong question, a check they did
not mean — changes the list and gets shown again.

#### Write it as one batch

Write the batch file with the file-writing tool, under the objective's own
directory, at `<root>/.ai-kit/plans/<objective-slug>/inbox-<UTC timestamp>.jsonl`.
That directory is ignored by the repository's git, so the file leaves nothing in
its `git status`; the legacy `<root>/plans/` is not ignored and never gets one. A
new name every time, because an earlier batch is a record of what was written and
is not overwritten.

It worked, nothing left:

```jsonl
{"kind":"decision","title":"Handover reported done: <what they ran or did, in one line>","body":"The user's words: \"<verbatim>\"\nSettles: <each question id and title>.\nLanding condition: <as the draft states it>.\nOutcome: met.","about":["handover","<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"answers","to":"<question-id>"},{"type":"met","from":"<objective-id>"}]}
```

One `answers` edge per settled question. When something is still owed, the same
line without the `met` edge and with `Outcome:` saying what is left.

It failed:

```jsonl
{"kind":"decision","title":"Handover reported failed: <the check>, <how it failed>","body":"The user's words: \"<verbatim>\"\nSettles: <each question id and title> — it was run, and it failed.\nOutcome: blocked on the open question naming the failure.","about":["handover","<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"answers","to":"<question-id>"}]}
{"kind":"open-question","title":"<the check> failed when the user ran it: <the failure, in one line>","body":"Reported through inbox: \"<verbatim>\"\nThe check: <the criterion or landing condition, as the draft states it>; instructions at <the manual-checks.md path>.\nWhat it needs is a fix, then a run.","about":["handover","<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"blocked","from":"<objective-id>"}]}
```

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
you showed them: `met`, or `blocked` on the question you named. The questions this
write answered should be gone from `open_records`, and on a failure the new
question should be on it. Say it in one line — the objective, its state now, and
what is still open on it. The assumptions `plan` recorded stay listed after a
`met`; that is what an objective closes with, not something the close missed.

An objective that still reads as it did before, after an exit 0, is a close that
did not land: say so with what the command printed, and write nothing more.

### 4 — Dropped, or not now

"Eso ya no va" — the objective is not going to happen — closes `cancelled`.
"Eso para más adelante" — the work is real but not under this parent now — closes
`separated`. Both point at a `decision`, both are resolved, shown and confirmed
exactly as step 3 does, and both are one line written to the same place:

```jsonl
{"kind":"decision","title":"Dropped: <objective title>","body":"The user's words: \"<verbatim>\"\nOutcome: cancelled.","about":["<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"cancelled","from":"<objective-id>"}]}
{"kind":"decision","title":"Set aside for now: <objective title>","body":"The user's words: \"<verbatim>\"\nOutcome: separated.","about":["<objective-slug>"],"edges":[{"type":"derives_from","to":"<objective-id>"},{"type":"separated","from":"<objective-id>"}]}
```

One of those lines, never both. A dropped objective's handed-over questions will
never be run, and its decision answers each of them, one `answers` edge apiece;
a set-aside objective's questions stay open, because the work they wait on is
still coming. Then the same `memory add --dir <root> --plan <plan> --batch …`,
and the same read-back.

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

**Dropping** is a `decision` that answers it, written into the idea's own log —
the `root` the `idea` line of the listing command printed, or the user log when
its repository is `user` and its `root` is empty. It is the same rule an `open-question` follows: an idea
leaves the Ideas section when something answers it, and nothing else removes it.
Write the line with the file-writing tool into this session's scratch directory,
never into a repository's tracked tree:

```jsonl
{"kind":"decision","title":"Idea dropped: <idea title>","body":"The user's words: \"<verbatim>\"","about":["idea","<area>"],"edges":[{"type":"answers","to":"<idea-id>"}]}
```

`about` follows the same rule as the idea's own: `idea` always, and the area only
when the user names one — `["idea"]` when they do not.

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
something answers it, it stays listed — which is correct, since until then nobody
has taken it up.

## Key principles

- **The user's sentence is the record.** Every close quotes it, and nothing is
  closed that they did not say.
- **Shown, then written.** The objective and the questions go in front of them
  before the batch exists, because a close on the wrong objective is permanent.
- **Written where it belongs.** An objective's close into its own log and plan, an
  idea into the log it names; this session's repository is not a default.
- **`met` is what the landing condition said.** No state above it, no queue of
  things to confirm; a deploy is on the record only where the objective asked for
  it.
- **Read back, not reported.** An exit status says the write was accepted; `state`
  says what the objective is now.
- **Explaining is reading.** The draft, the instructions and the records answer
  what an item is; the code is `ask`'s, when asked.
