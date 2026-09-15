---
name: debug
description: "Take a symptom — something is wrong and nobody yet knows why — and narrow it to a stated cause before any acceptance criterion is written. Tests one hypothesis at a time with a single-variable experiment, records every hypothesis it tests with the evidence that tested it and the fidelity of that evidence, and ends by handing what it found to `plan`, which decides how big the resulting work turns out to be. It never fixes anything and never modifies the repository it is investigating. Use whenever what arrived is a failure rather than a request: a failing test, a panic, a 500, wrong output, a performance regression, a production incident, or a fix that did not work. Triggers on: 'esto está roto', 'se rompió', 'por qué falla', 'investigá este bug', 'el test falla', 'esto tira error', 'no entiendo por qué pasa', 'dejó de andar', 'hay un incidente en prod', 'qué pasó en producción', 'el fix no funcionó', 'why is this failing', 'this used to work', 'debug this', 'find the root cause', 'what is causing this'. When the cause is already established and what is left is to build the fix, that is `plan`'s job and then `work`'s — `debug` is for the case where what to build depends on why."
---

# debug

Read [the host runtime](references/runtime.md) first for tool invocation,
delegation, repository paths and the memory prerequisite.

Turn a symptom into a stated cause, with the evidence that established it, and
stop there. What happens next — one objective or a tree of them — is `plan`'s
decision, taken in the same conversation, from what this skill hands it.

The loop's other front-end starts from a want and reasons forward. A symptom
cannot be reasoned forward from. Forced through `plan` anyway, it produces
acceptance criteria written against a cause nobody has established — and those
criteria are then frozen, which is the whole point of them, so a guess made in
the first ten minutes becomes the thing every later step is graded against.
This is the route that narrows first.

There is no such thing as a "debug objective". An objective is what must be true
when the work is done, with criteria and a fidelity on each, and a fix objective
has exactly that shape and nothing more. So this skill authors none of them: it
investigates, and it hands its finding to the skill that already knows how to
size what it finds.

## Hard rules

1. **One hypothesis at a time, tested by one single-variable experiment.** Two
   changes in one probe produce an outcome that names neither of them, and the
   cheapest way to spend an hour is to confirm something that was true of the
   probe rather than of the system.
2. **A failed hypothesis produces a new hypothesis, never a second attempt
   stacked on the first.** Piling a second theory on top of an unretracted first
   is how an investigation ends up with three half-tested claims and no way to
   tell which of them the evidence was about. The failure is information: form
   the next hypothesis *from* it.
3. **"I don't know" is a valid outcome and it is a result, not a failure.** It
   is only allowed to be said with what was ruled out and what would settle it —
   that is what makes it a finding a later session can start from, rather than an
   admission. A confident wrong cause costs far more, because everything built on
   it is built wrong and looks right.
4. **Three failed hypotheses ends the investigation.** Stop and invoke `plan`
   with what you have. By the third miss the problem is almost never one more
   theory away: the pattern — every probe revealing another coupling, every
   explanation needing "well, unless" — is what a design problem looks like from
   the inside, and that is work to be planned rather than debugged. The fourth
   guess is not this skill's to make.
5. **Never state a cause without its evidence and that evidence's fidelity.**
   "It's the connection pool" is a guess wearing a conclusion's clothes. The
   cause, what you observed, where, and how strongly — `real-dependency`,
   `integration` or `code-read` — travel together or not at all, because `plan`
   is about to write criteria on top of this and it needs to know whether anybody
   actually saw it happen.
6. **Never modify the repository under investigation.** Not code, not tests, not
   a temporary log line you mean to take out afterwards. You read it, you run
   what it already has, you query its logs and traces, and anything you need to
   write goes to scratch outside it. The obligation-to-revert version of this
   rule breaks the one time it matters — a session cut short leaves somebody
   else's repo dirty and nothing saying so. When confirming a hypothesis would
   require touching the repo, that is itself a hypothesis for `plan`, and the
   fidelity stays `code-read` with that said out loud. **The loop's own
   append-only log is the single exception**, and it is not one in practice: the
   log belongs to the repo the loop is running in, not to the repo under
   investigation, and the next section says how to keep those two apart when they
   are different directories.
7. **Never write an `objective` record, and never decide how big the work is.**
   Sizing is a five-part test that lives in `plan` and runs there with the user.
   A cause implying a long sequence of work produces a tree through the same
   machinery as everything else — there is no special case here and no ceiling
   you are allowed to put on it.
8. **A library version, an SDK shape or a provider API is resolved, never
   recalled.** Your training data lags releases, and "that behavior can't have
   changed" is exactly the difference a bug hides in. Look it up — `context7` if
   available, otherwise the pinned source in the dependency tree, otherwise the
   web — and cite what you used in the hypothesis you write down.
9. **Conversation in the user's language; everything written to disk in
   English.**

## The memory log

State lives in an append-only log, read and written only through the `memory`
CLI. Never edit the log by hand.

**Every `memory` call in this skill passes `--dir`, and it points at the repo the
loop is running in** — the one whose `.ai-kit/` holds the log, where `plan` writes
its drafts under `.ai-kit/plans/` and `work` reads them. That repo is usually also
the one under investigation, and then the two are the same path; but this is the
skill that can be pointed at somebody else's checkout, and the difference has to be
settled once, here, rather than per command.

Derive it, before anything has had a chance to change directory. It is not a value
to ask for and it is not one to type from memory — git already knows which
repository this is:

```bash
git -C "<the path the user named, or the session's cwd>" rev-parse --show-toplevel
```

That is the same command `ask` and `bootstrap` derive their target with, and it
replaced a walk upward for `plans/` or `.ai-kit/` because those markers answer a
different question: "has this repo run the loop before?" In a repository being set
up right now the honest answer is no and the walk refuses the one case that works.
Nothing has to exist first — the `memory` CLI creates `.ai-kit/` on the first
command that resolves a location, read or write.

It is written bare, with no `LOOP_REPO=$(…)` around it, and that is not an
oversight: **nothing in this turn says which shell it got**, and an assignment of a
command substitution is one of the shapes a shell may refuse outright — at parse
time, so that a block carrying it runs none of its lines. Nothing is lost by
leaving it off — a variable would not survive to the next command either, and what
this line is for is putting the path on stdout where you can read it and copy it.

**Empty output means the derivation failed, and a failed derivation stops the
skill.** `rev-parse` prints nothing on stdout when the path is missing or is not
inside a git repository. Say so and ask, rather than proceeding:

> No puedo resolver el repo del loop desde `<the path>` — o no existe, o no está
> dentro de un repositorio git. Decime la ruta absoluta del repo donde estás
> llevando los objetivos y sigo desde ahí.

Then take the answer, confirm the path printed, and use it. **What must never
happen is a `memory` call with `--dir` empty**, because that is not an error: it
falls straight back to resolving the log from the current directory — measured,
exit 0. The refusal above covers a path that *is* passed and is not a repo; it
never fires on the empty string, so this stop is the only thing between a `--dir`
that lost its value and an investigation filed in the wrong repository.

That failure is worth stating in full, because it is what stops a later edit from
dropping the flag as noise. Resolved from the current directory, the log is
whatever is above cwd — so a `cd` into the repo under investigation, or step 4's
bisect clone in scratch, redirects the write. And it is not only `add` that lands
there: since every command **creates** the location it resolves, even a `query`
leaves an `.ai-kit/` behind wherever cwd happens to be — a directory appearing in
a tree hard rule 6 promises never to touch, and the hypotheses ending up in a log
nobody will read again.

`<loop-repo>` in the commands below is a placeholder for the path this derivation
prints — the same notation as `<tree-slug>` and `<area>`, and not a variable
anything sets. **Substitute that absolute path into every command**: each command
runs in its own shell, so nothing carries the value forward for you.

The notation is what makes forgetting survivable. `--dir "$LOOP_REPO"` left
unsubstituted is `--dir ""`, which is the silent fallback above — exit 0, in
whatever repository cwd happens to sit in. `--dir <loop-repo>` left unsubstituted
is not a command at all: the shell reads `<` as a redirection, the command fails
with a non-zero exit, and `memory` never runs. Which failure you get differs by
shell and by where the placeholder falls in the line; the measured exits and
messages are in the log. It is the same reasoning step 4's scratch path is
spelled in, one section down. A path you can read in the command is a path you
can see is wrong. Leave the brackets unquoted: `--dir "<loop-repo>"` parses, so
the command runs unsubstituted and the brackets stop nothing.

`memory kinds` lists every valid record kind and edge type. The kind this skill
writes is `hypothesis`, and it is the only one it writes: a confirmed root cause
is not a record of its own — it is the hypothesis whose verdict is `confirmed`,
because one claim stored twice under two names leaves a reader to work out which
copy is authoritative. A rejected hypothesis is not a `dead-end` either; that
kind means an approach abandoned during implementation, and splitting "what was
already tried" across two kinds is how a later session finds half of it.

## Steps

### 1 — Get the symptom stated, and read what the log already ruled out

A symptom is not a diagnosis and it is not a feeling. Restate it as something
observable, in one line, and put it back to the user for correction before
anything else:

> Lo que veo es: **esperábamos X, pasa Y**, en `<dónde>`, desde `<cuándo>`,
> `<siempre / a veces>`. ¿Es eso, o me está faltando algo?

Four things make the difference between an investigation and a wander: what was
expected, what happened instead, where it is observable, and since when. Ask for
the ones you do not have — in one `AskUserQuestion` call, not four turns — and do
not interrogate for the rest. Everything else you are about to find out yourself.

Derive the loop repo here, by the section above, before anything else runs a
command. This is the last moment it is trivially correct: nothing has changed
directory yet, and from step 4 onward something may have. A derivation that comes
back empty stops the skill at this line rather than at the first write.

Then settle which **objective tree** these records belong to, because `--plan`
bounds everything you are about to read and everything you are about to write:

```bash
memory state --dir <loop-repo>
```

When an open objective already covers this surface, take its `origin.plan` —
`state` reports it as `plan` — and use that. When nothing does, which is the
ordinary case for a symptom that arrived on its own, coin one from the symptom,
kebab-case, and keep it: step 8 hands the same slug to `plan`, so the objective
it authors lands in the same tree as the investigation that produced it.

Now read the log, because the most expensive thing an investigation can do is
re-eliminate something a previous session already eliminated:

```bash
memory query --dir <loop-repo> --plan <tree-slug> --kind hypothesis --about <area>
memory query --dir <loop-repo> --plan <tree-slug> --kind wont-fix --about <area>
```

Prior hypotheses are the point of the first read. A `rejected` one is a branch you
do not have to walk again; a `confirmed` one on the same surface may be this bug's
cause returning. The second read is for the risks somebody already looked at and
decided not to cover: a `wont-fix` on this surface names a failure mode that was
accepted on purpose, and the symptom in front of you may be exactly it arriving —
which makes it both a strong first hypothesis and a conversation to have with the
user rather than a bug to fix. Drop `--plan` for a slug you just coined — nothing
is written under it yet, and the areas are what find the earlier work anyway. Say
in one line what came back, including "nada": a filter that ran and found nothing
is different from a step that was skipped.

#### When `plan` invoked you

`plan` routes here from its own step 1, and when it does it arrives holding three
things you would otherwise pay for again: the symptom as the user stated it, the
tree slug it already resolved, and whatever `explore` established for it. Take
all three as given. Do not run the readback above — the user just discussed this
symptom, and asking them to confirm it a second time is how they learn to skim —
and do not re-derive the slug; `plan` will write its objective under the one it
handed you. Carry its findings into step 3, where they go in as `prior_findings`
rather than as a fresh exploration.

Everything else in this step still runs. The log read in particular: `plan`'s own
step 1 reads objectives, not hypotheses, so what a previous session already ruled
out on this surface is exactly what neither of you knows yet.

### 2 — Read the failure whole, then reproduce what can be reproduced

Read the *entire* error — every frame, the wrapped causes, the error code, the
surrounding log lines with their timestamps. The answer is in there more often
than anybody expects, and the top frame is usually where the failure surfaced
rather than where it came from.

Then find out how reliably you can make it happen: on demand, sometimes, or only
in production. That answer is not a gate. **A repro test is not required to
proceed, and demanding one would make the least useful confirmation the mandatory
one** — a bug a test would have caught was already caught, because the suite ran
before deploy. Real bugs arrive from running systems. What a reliable repro buys
is *fidelity*, which is why fidelity is recorded on every hypothesis rather than
required of it.

What "reproduce" means here is bounded by hard rule 6: run the suite that
already exists, run the binary, issue a request against the running system, query
the logs and traces, diff two environments' configuration. It does not mean
writing a test, and it does not mean adding a log line to find out what is in a
variable. When the only way to see the value is to make the code emit it, you
have found something for `plan` — see step 5.

### 3 — Find the boundary where the value first goes bad

A bug in a system with more than one layer hides at a boundary. Find the boundary
before investigating any single component: the question is not "what is wrong
with this function" but "where does the bad value stop being good".

Delegate the reading. Launch the `explore` subagent rather than opening twenty
files in this thread, because the hypothesis loop needs the context that reading
would spend:

```
topic: <the diagnostic question: where does <value> come from, and what is the last place it is still correct?>
repo_root: <absolute path of the repo under investigation>
scope: code
paths_hint: <the frames from the trace, the module the symptom names>
prior_findings: <when `plan` invoked you: the findings it handed over, claim and evidence intact>
```

`prior_findings` is what stops the plan-initiated path from paying twice. Handed
them, `explore` revalidates each one against the repo before exploring anything
new and returns `revalidated` alongside its own `findings`, one verdict per
inherited claim: `still_holds`, `changed` or `could_not_check`. A claim that
still holds is a file nobody reads twice, and the other two verdicts are where
your own looking starts. Leave the field out when the symptom came straight from
the user — there is nothing to revalidate.

Its `findings` come back with `path:line` anchors — keep them, they are the
evidence half of every hypothesis you are about to write. Its `gaps` are as
useful: "I could not establish where this is set" is a candidate hypothesis
rather than a hole to paper over. Launch a second one with a narrower topic when
the first lands you at a boundary you did not know existed; that is cheaper than
widening the first one's budget.

Then trace backward. Once you know where the bad value shows up, walk it to where
it is born — the cause is at the source, and every layer between them is
somewhere a fix would be a patch over a symptom.

### 4 — Ask the history, when there is a known-good point and a check you can script

"It used to work" is a fact with a commit range in it, and `git bisect` finds the
commit for you in log₂ steps instead of you reading them one at a time. It is
worth reaching for whenever two things hold: a revision where the behavior was
correct, and a command that decides good-or-bad by its exit code.

Bisecting moves `HEAD`, so it does not run in the repository under investigation
— hard rule 6 has no exception for "but only the working tree". It runs in a
clone in scratch, which reads that repo and writes nothing to it. The scratch path
is printed and pasted rather than captured, for the reason the loop-repo
derivation above gives, and it is spelled in angle brackets rather than as
`$SCRATCH` notation because an unsubstituted `$SCRATCH` expands to nothing and
points the clone at `/bisect`, while angle brackets cannot run unsubstituted at
all:

```bash
mktemp -d
```

```bash
git clone --no-hardlinks <repo_root> <the path mktemp printed>/bisect && cd <the path mktemp printed>/bisect
git bisect start <known-bad-rev> <known-good-rev>
git bisect run <command that exits 0 when good and non-zero when bad>
```

Three things decide whether this pays: the check has to be **fast**, because it
runs about a dozen times; it has to be **the symptom** rather than something
adjacent, or you will bisect your way to the commit that changed the adjacent
thing; and the range has to actually build at every point in it, which is what
`git bisect skip` is for on the commits that do not.

When a clone is not workable — the build needs credentials, the repo is enormous,
the symptom only appears against production data — say so and do not improvise a
way around hard rule 6. Read `git log` in place instead, narrowed to the files
step 3 pointed at, and treat what you find as `code-read`. The commit that
`bisect` would have named is a hypothesis like any other, and "run a bisect" can
be handed to `plan` as work when it is worth its cost.

### 5 — State one hypothesis, and design the experiment that could kill it

Write the claim before you test it, in one sentence, specific enough to be wrong:

> **X es la causa, porque Y. Si es eso, entonces Z tiene que ser observable.**

That last clause is the whole step. A hypothesis with no observable consequence
cannot be tested, only believed, and an investigation made of those is a story
about the code. The experiment is the smallest observation that separates *this*
claim from the others still standing — one variable, and nothing bundled with it
that changes a second thing.

Three experiment shapes are available to you, and they are exactly the three that
do not touch the repository: **run something that exists** (the suite, the
binary, a request, a query against the real system), **read something the system
already emits** (logs, traces, metrics, a database row, a config diff), or
**follow the value through the source** to the place it is set.

When the experiment you actually want is a fourth kind — add a log line, write a
failing test, instrument the boundary — you do not run it. Two honest moves are
left, and both of them are better than the rule they would break:

- Find the observation the system already gives you. A value you cannot print is
  often visible one layer out, in a response, a metric or an error message.
- Record the hypothesis with verdict `could-not-test` and fidelity `code-read`,
  and carry it to step 8. "Make this observable" is a real objective with real
  criteria, and `plan` can size it — which is a far better outcome than a probe
  left behind in somebody's working tree.

### 6 — Run it, then write the hypothesis down whatever it says

Write the record at the moment you learn the answer, not at the end of the
investigation. A session that gets cut short after the third hypothesis leaves
the next one nothing at all if the writing was saved for a close that never came,
and the rejected ones are precisely what nobody would reconstruct.

```bash
memory add --dir <loop-repo> --plan <tree-slug> --kind hypothesis \
  --title "<the claim in one line, stated so it could be false>" \
  --body "Claim: <what you think is happening, specifically>
Tested by: <the single-variable experiment you ran>
Verdict: <confirmed | rejected | could-not-test>
Evidence: <what you actually observed — path:line, the log query and what it returned, the command and its output>
Fidelity: <real-dependency | integration | code-read>" \
  --about <area>,<path>
```

`--dir` is not optional here, and it carries the absolute path step 1 derived
rather than a variable. Step 4 may have left you standing in a bisect clone and
step 5 may have run commands inside the repo under investigation; the log this
record belongs to is neither of those, this is the one command in the skill that
*writes*, and an empty `--dir` here is what creates `.ai-kit/` in somebody else's
tree.

All six parts are load-bearing and none of them is decoration. The **title** is
the index — kind-plus-title in chronological order is how these are read months
later, so "Retries reuse the pooled connection after it is closed" and not
"Connection pool hypothesis". The **claim** is what a later session compares
against its own symptom. **Tested by** is what tells that session whether the
rejection still stands or was an artifact of how it was probed. The **verdict** is
one of exactly three values. The **evidence** is what makes the verdict checkable
by somebody who was not here. And the **fidelity** is what stops all of it from
being read as stronger than it is:

- **`real-dependency`** — observed in the running system: production or staging
  logs, a trace, the real database, the real queue.
- **`integration`** — reproduced locally against the real pieces, with nothing
  in the path standing in for something else.
- **`code-read`** — followed in the source and never observed. A legitimate
  outcome and a common one; it just has to say so, so `plan` writes criteria
  knowing nobody saw it happen.

**`mocked` is deliberately not on this ladder.** A cause confirmed against a mock
is confirmed against something that is not the system, which is the failure this
whole loop exists to avoid — and a hypothesis has no honest use for it, because
the mock's behavior is the thing you would be diagnosing.

A `rejected` hypothesis gets the same record with the same care. It is the one a
future session most needs and most often lacks.

### 7 — Loop, or stop honestly

**Rejected** — form the next hypothesis from what the failure just told you
(rule 2) and go back to step 5. Count it: this was one.

**Three failed hypotheses** — stop, by rule 4. Say so plainly, name the three and
what each eliminated, and go to step 8 with an investigation that did not
conclude. That is a legitimate hand-off; a fourth guess is not. A
`could-not-test` does not count toward the three: nothing was eliminated and no
theory failed — what happened is that the experiment needed the repo, so it is
work for `plan` rather than a strike, and it does not buy you an extra guess
either.

**No hypothesis worth testing left** — this is rule 3's outcome and it goes to
the user in words, with the two things that make it useful:

> No llegué a la causa. Descarté `<A>`, `<B>` y `<C>` — cada una está en el log
> con la evidencia. Lo que la cerraría es `<lo que haría falta observar>`, que
> hoy no es observable sin tocar el repo.

**Confirmed** — say the cause back in one line *with its fidelity*, before
anything else happens:

> La causa es `<X>`. Lo confirmé `<cómo>` — fidelidad `<real-dependency |
> integration | code-read>`.

Say the fidelity out loud even when it is the strong one, because the whole point
is that "confirmed" means different things at the three levels and the user is
about to approve criteria on top of it.

### 8 — Hand the finding to `plan`

The investigation ends here and the authoring starts, in this same conversation.
**Invoke the `plan` skill** — the Skill tool, skill `plan`, or `ai-kit:plan`
where the plugin namespace is in play — and give it what it needs to write
against:

```
the symptom, as step 1 stated it
the finding: the confirmed hypothesis with its record id, its claim and its fidelity —
  or "no confirmed cause", or "nothing is broken", stated as plainly as that
the hypotheses that were rejected, with what each one eliminated
the tree slug the hypotheses were written under
the areas and paths the investigation touched
```

**All three findings go through this block, and none of them is a degenerate
input.** A confirmed cause, an investigation that ran out of hypotheses, and a
symptom with no bug behind it are three different things for `plan` to author
against, and which of them this was is the first line it needs.

Then let it work. **How big the resulting work is, is `plan`'s decision and not
yours**: it applies its own five-part sizing test with the user and writes either
a single executable objective or a parent objective with a child underneath it,
and it keeps descending until it reaches one a session can deliver. A cause whose
fix implies months of restructuring produces a tree through exactly that
machinery — there is no ceiling here that says "this is too big for a bug", and
saying one would put the shape of the work back in the hands of the thread that
found the cause rather than the one that has to size it.

`plan` links the objective it writes back to the hypothesis with a `caused_by`
edge, so the work and the investigation that produced it are joined on disk
rather than merely sharing a slug. It writes that edge; you write no `objective`
record and no state edge at all.

An investigation that ended without a cause hands over the same way, and what
`plan` sizes is different: usually the objective that makes the thing observable
— the instrumentation, the trace, the log field that the next investigation needs
and hard rule 6 would not let you add here.

**When the finding is that nothing is broken** — a misread output, an expectation
that was never true, a difference between two environments — say that plainly,
and hand it over anyway. Whether anything should be built out of it is a real
question with real answers — a guard, a clearer error, a line of documentation,
or nothing at all — and it is the same question `plan` asks about every finding,
with the user, using the sizing test that lives there. Deciding "no work follows"
from here would be this skill sizing the work after all, which rule 7 gives away
on purpose, and it would do it in the one case where the evidence is thinnest.

So there is no exit from this step that skips the handoff. The investigation ends
by invoking `plan`, whichever of the three findings it reached; the hypotheses are
already on disk with their verdicts, which is what says it concluded rather than
stalled, and `plan` is free to conclude that nothing needs building.

## Key principles

- **The cause is the deliverable.** Not a fix, not a patch, not a theory — a
  stated cause with the evidence that established it and the fidelity of that
  evidence.
- **A rejected hypothesis is worth writing down.** It is the record of what has
  already been eliminated, which is the thing a later session most often lacks
  and the thing nobody would reconstruct.
- **Fidelity is stated, never assumed.** "Confirmed" means three different
  things, and the difference is exactly what somebody writing frozen criteria
  needs to know.
- **The repository under investigation is never touched.** An investigation that
  leaves a probe behind has traded somebody else's clean tree for an hour of
  yours.
- **One variable, or the answer names nothing.**
- **"I don't know" beats a confident wrong cause**, and it costs one honest
  sentence against everything that would have been built on the guess.
- **Sizing belongs to `plan`.** Finding the cause and deciding how much work it
  implies are two different judgments, and the second one is not made by the
  thread that just spent its context on the first.
