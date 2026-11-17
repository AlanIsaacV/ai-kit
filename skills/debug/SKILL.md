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
   guess is not this skill's to make. The three are attempts to explain the
   symptom. A deeper "why" tried in step 8, once the symptom is already
   explained, is not one of them: a miss there ends the descent by that step's
   own limit and leaves this count where it was.
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
kebab-case, and keep it: step 9 hands the same slug to `plan`, so the objective
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
  and carry it to step 9. "Make this observable" is a real objective with real
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
record belongs to is neither of those, this command and step 8's `caused_by` link
are the only ones in the skill that *write*, and an empty `--dir` here is what
creates `.ai-kit/` in somebody else's tree.

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
what each eliminated, and go to step 9 with an investigation that did not
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
anything else happens, and say that it is the first level rather than the last:

> La causa inmediata es `<X>`. Lo confirmé `<cómo>` — fidelidad
> `<real-dependency | integration | code-read>`. Ahora miro si eso, a su vez,
> tiene una causa debajo.

Say the fidelity out loud even when it is the strong one, because the whole point
is that "confirmed" means different things at the three levels and the user is
about to approve criteria on top of it. Then go to step 8: a confirmed hypothesis
is where the descent starts, not where the investigation ends.

### 8 — Descend from the confirmed cause, one "why" at a time

The hypothesis step 7 just confirmed is the **proximate cause**: the first thing
that explains the symptom. It is very often the symptom of something else. A
request times out because the connection pool is exhausted, and the pool is
exhausted because a retry path never returns its connection; confirm the first and
stop, and `plan` writes criteria against the pool — a bigger pool, a shorter
timeout — and what ships is a mitigation named as a fix, with the leak still there
producing the next outage. Step 3's trace backward does not reach this. It walks a
bad value to where it is born, which is data flow inside one cause; the question
here is the cause of a cause, and nothing else in this skill asks it.

So every confirmed level gets asked two things, in this order: why the code that
causes it is written that way, and whether it is itself a symptom of something one
level down.

**Why it is written that way** is a question for the history before it is one for
theory, because the answer that most changes what happens next is "on purpose". Ask
git about the lines this level's evidence anchors — the `path:line` it cites —
and ask the loop's log about the area:

```bash
git -C <repo_root> log -L <first>,<last>:<path>
git -C <repo_root> blame -L <first>,<last> -- <path>
memory query --dir <loop-repo> --kind decision,adr,postmortem,gotcha,wont-fix --about <area>,<path> --current
```

The two `git` commands read the repository under investigation and write nothing
to it, which is what hard rule 6 already allows; narrowing them to the anchored
lines is what keeps the answer about this cause rather than about the file. The
`memory` query drops `--plan` on purpose: a decision about this code was taken
under whatever tree was open when somebody made it, not under the one this
investigation coined. And **its `--dir` is the loop repo, never the repository
under investigation**, even though the question is about that repository's code.
This is the command in the skill most tempting to point there, and it is the one
that does the damage if you do: `memory query --dir` at a repository with no
`.ai-kit/` creates `.ai-kit/` in it — measured, on a read — which is a directory
appearing in a tree hard rule 6 promises never to touch. The loop's log is the only
one this step reads.

What comes back is evidence for the level below. A commit whose message says why
the code is shaped this way, a `decision` or `adr` on the area, a `wont-fix` that
accepted this failure mode — any of those says the cause is a choice, and that is
one of the places the descent stops. A commit that introduced the lines as a quick
fix, a `postmortem` that met this before, a `gotcha` about the constraint: those
are what the next "why" is formed from.

**Whether it is a symptom of something deeper** is a hypothesis like every other,
and it is held to the same standard.

**Split a compound "why" before you test it.** The first answer to "why" is rarely
one claim. It tends to arrive as a mechanism — the pool is exhausted because a
retry path keeps its connection, under a burst of traffic, while a slow query holds
the rest — and every "because", "while" and "and" in it joins a claim that could be
true or false on its own. Tested whole, it needs one experiment that separates all
of its parts at once, which is almost never available, so it tends to come back
`could-not-test` and end the descent above a part that was observable by itself.
So before the deeper hypothesis is formed, write the "why" out and split it at each
joint into single claims: each one thing that is or is not happening, specific
enough to be wrong, carrying no "because" or "and" the others do not. Each single
claim is its own `hypothesis` record, with the six parts, written by step 6 when it
is tested. Two claims never share one record; a claim the descent ends before
testing is named in the evidence of the record that ended it, rather than written
with a verdict it did not earn.

Then order them. **The claim nearest the level above** is the one whose truth would
cause that level directly — the one that joins the rest to the chain. The others sit
beneath it: they explain it, not the level above. The level's one attempt is spent
on that nearest claim. State it by step 5 — the claim, the observable consequence,
one single-variable experiment from the three shapes that do not touch the
repository — and run it and record it by step 6, with the same six parts and the
same command:

```bash
memory add --dir <loop-repo> --plan <tree-slug> --kind hypothesis \
  --title "<the deeper claim in one line, stated so it could be false>" \
  --body "Claim: <what causes the level above, specifically>
Tested by: <the single-variable experiment you ran>
Verdict: <confirmed | rejected | could-not-test>
Evidence: <what you actually observed — path:line, the log query and what it returned, the command and its output, what git log and blame said>
Fidelity: <real-dependency | integration | code-read>" \
  --about <area>,<path>
```

**"Nothing catches X" is never a level's claim.** A claim of the form "nothing
catches, handles or retries X" — no try/except around the write, no guard on the
value, no retry on the call — does not cause X: X happens the same with it or
without it. What it explains is how X reaches the symptom, and that is where it
goes: in the evidence of the level whose claim is X, as what lets the failure
propagate. Split it off a compound "why" like any other joint, but it is not one of
the single claims the split tests, and it gets no record and no level of its own.
The consequence lands in step 9: a fix that only adds that handling — catch and
log, a guard, a retry — removes no level of the chain, so it is a `mitigation`
when it reduces a level and a `patch` when it reduces none, and never `root`.

When the nearest claim comes back `confirmed`, join it to the level above it:

```bash
memory link --dir <loop-repo> --type caused_by --from <proximate-hypothesis-id> --to <deeper-hypothesis-id>
```

The edge runs from the shallower record to the deeper one, read as "this is
caused by that" — the same direction `plan`'s objective will point at the level it
attacks, so the whole chain is walked one way. On the first step down the
proximate cause is the `--from`; on every step after it, the level you just
confirmed takes that place. Say each confirmed level back the way step 7 says the
first one, with its fidelity, because the user is about to choose between fixes
that attack different levels and needs to know how strongly each was seen. The
claims that sat beneath it are where the next "why" starts: order them again
against the level just confirmed, and spend that level's one attempt on the nearest.

**One attempt per level.** The attempt is the nearest claim's test. When it comes
back `rejected`, the descent ends at the last confirmed level. Write its record
anyway, with the same care as any rejected hypothesis, and do not link it:
`caused_by` joins causes, and a rejected claim is not one. Rule 2 does not send you
back for another guess at this level — it forbids stacking a theory on an
unretracted one, and stopping stacks nothing — and rule 4 does not count the miss,
because the symptom is already explained and what failed was a search for
something beneath an established cause. The limit is there because the descent
spends the same context the handoff needs, and every level down is worth less to
the fix than the one above it was.

**When the attempt comes back `could-not-test`, and only then, the claims beneath
it are tested before the descent ends.** `could-not-test` says nothing about
whether the claim is true — only that the observation that would separate it is not
available — and the claims the split put beneath it may each be observable on their
own. Ending there would hand `plan` a chain that stops above a claim the evidence
already shows. So write the `could-not-test` record, unlinked like a rejected one,
and then test each claim beneath it by steps 5 and 6: its own single-variable
experiment and its own record. What counts as `confirmed` is exactly what it is in
steps 5 to 7, and circumstantial evidence still confirms nothing. The claims that
come back `confirmed` form the **conditional level**: each confirmed on its own
evidence, and joined to the chain only through a link nobody could confirm. Join
each of them to the `could-not-test` record, and never with `caused_by` — that edge
joins confirmed causes, and the link between these claims and the chain is exactly
what was not confirmed:

```bash
memory link --dir <loop-repo> --type relates_to --from <could-not-test-hypothesis-id> --to <conditional-hypothesis-id>
```

Its `--dir` is the loop repo, like every other command here. There is at most one
conditional level, and the descent goes no further below it: every claim under it
would rest on a link nobody saw. Write down, for the handoff, **the observation that
would join it to the chain** — the one the `could-not-test` record says was
missing. A claim beneath that comes back `rejected` or `could-not-test` gets its
record and no link, and none of these tests counts toward rule 4's three, for the
same reason the attempt does not. A `rejected` attempt gets none of this: a claim
that was ruled out leaves nothing beneath it to carry.

Before forming the next "why", check whether there should be one. **The descent
stops at the first of three things:**

- **The next "why" no longer changes what would be built.** If fixing the deeper
  cause would be the same change as fixing the level you have, the deeper level
  adds a record and no option. Stop at the level whose fix is the one that would
  ship.
- **It reaches a deliberate decision.** When the history said this code is the way
  it is on purpose, going below it is arguing with a choice somebody made, and that
  is a conversation with the user rather than an investigation. Stop at that level
  and name the commit or record that makes it deliberate.
- **It leaves the system.** When the next "why" is about people, process or a
  provider — why nobody reviewed it, why the team ships on Fridays, why the
  provider throttles — no experiment this skill may run can test it and nothing
  built in this repository fixes it. Stop at the last level that is inside the
  system.

What the descent leaves behind is **the chain**: the confirmed levels in order
from the proximate cause down to the deepest confirmed cause, each a `hypothesis`
record joined to the next by `caused_by`. When the attempt below the deepest of them
came back `could-not-test`, it also leaves **the conditional level**: its confirmed
claims, the untestable record they sit under, and the observation that would join
them. Then write down **the candidate fixes** against the chain — every fix the
investigation surfaced, the cheap ones included, each placed in a layer by step 9's
definitions on the chain alone, and each fix that removes a claim of the
conditional level marked with what would make it root — and go to step 9.

### 9 — Hand the finding to `plan`

The investigation ends here and the authoring starts, in this same conversation.
**Invoke the `plan` skill** — the Skill tool, skill `plan`, or `ai-kit:plan`
where the plugin namespace is in play — and give it what it needs to write
against:

```
the symptom, as step 1 stated it
the finding: a confirmed cause, or "no confirmed cause", or "nothing is broken",
  stated as plainly as that
the chain: the confirmed levels in order, from the proximate cause (the hypothesis
  first confirmed) down to the deepest confirmed cause — each level with its
  hypothesis record id, its claim and its fidelity — and which of step 8's stops
  ended the descent
the conditional levels: none, or the one step 8 recorded beneath a could-not-test
  record — each of its claims with its hypothesis record id, its claim and its
  fidelity, the could-not-test record it sits under, and the observation that
  would join it to the chain
the candidate fixes: each with its layer (root, mitigation or patch) computed on
  the confirmed chain alone, the chain level it attacks (for a patch, the
  proximate cause, the top of the chain), what it removes (a level, or none),
  what it leaves alive, and "would be root if: <observation>" — the observation
  that would join the conditional level it removes a claim of, or none
the hypotheses that were rejected, with what each one eliminated
the tree slug the hypotheses were written under
the areas and paths the investigation touched
```

**All three findings go through this block, and none of them is a degenerate
input.** A confirmed cause, an investigation that ran out of hypotheses, and a
symptom with no bug behind it are three different things for `plan` to author
against, and which of them this was is the first line it needs.

**A fix's layer is defined by what it removes from the chain, not by how
thorough the fix looks.** Every candidate carries two answers that are easy to
conflate: the level it attacks, which is where its change acts and where `plan`'s
`caused_by` will point, and what it removes, which is a level or none. The layer
is read off the second, and the three layers split every fix between them with
none left over:

- **`root`** removes the deepest confirmed level. Nothing in the chain below it is
  left to produce the symptom again.
- **`mitigation`** reduces a level — any level, the deepest included — or removes
  a level above the deepest, and leaves the deepest confirmed level alive. The
  symptom gets rarer, smaller or quieter; the cause underneath keeps producing it.
- **`patch`** removes and reduces no level. It handles the symptom — a retry, a
  guard, a clearer error — and leaves every cause in the chain alive; the level it
  attacks is the proximate cause, the top of the chain, because that is where the
  symptom surfaces.

The test is what the fix removes, so ask it of every candidate in this order:
does it remove the deepest confirmed level? Then it is `root`. If not, does it
remove or reduce any level? Then it is `mitigation`, whichever level it attacks.
If neither, it is `patch`. A fix that attacks the deepest level and only reduces
it is a mitigation, however deep it reaches; a fix that removes the proximate
cause of a two-level chain is a mitigation, whatever it is called in the
conversation that proposed it; and a fix that only makes the failure survivable
removes and reduces nothing, however much code it takes. That includes a fix that
only adds the handling step 8 keeps out of every level's claim — catching, logging
or retrying the failure: it removes no level, so it is never `root`. Naming that
wrongly is exactly how a mitigation ships as the fix and the deeper cause is
forgotten. `plan` presents these candidates to the user as its options, grouped by
layer; which fix gets built is decided there, with them, and not here.

**A conditional level does not move a fix's layer.** The layer is computed on the
confirmed chain, the levels joined by `caused_by`, and the conditional level is not
in it: a fix that removes one of its claims removes no confirmed level, so by the
test above it is a `mitigation` when it reduces a confirmed level and a `patch`
when it reduces none. What it would be if the untestable link were confirmed goes
beside the layer, as `would be root if: <observation>`, carrying the observation
step 8 wrote down for that level. It is a field on the candidate, not a fourth
layer: the three layers still split every fix between them with none left over,
and a candidate whose field is filled reaches `plan` at its computed layer with the
condition that would change it in plain sight.

Then let it work. **How big the resulting work is, is `plan`'s decision and not
yours**: it applies its own five-part sizing test with the user and writes either
a single executable objective or a parent objective with a child underneath it,
and it keeps descending until it reaches one a session can deliver. A cause whose
fix implies months of restructuring produces a tree through exactly that
machinery — there is no ceiling here that says "this is too big for a bug", and
saying one would put the shape of the work back in the hands of the thread that
found the cause rather than the one that has to size it.

`plan` links the objective it writes back to the hypothesis of the chain level
the chosen fix attacks with a `caused_by` edge, so the work and the
investigation that produced it are joined on disk rather than merely sharing a
slug, and the rest of the chain is reachable from there through the edges step 8
wrote. It writes that edge; you write no `objective`
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
