---
name: ask
description: "Answer a question about how a repository works, from the code and from the log, in one narrative where every claim carries its citation — `path:line` for anything read off the code, a record id and that record's date for anything that came out of the memory log. Routes by what the question asks for: mechanism is read from the code through the `explore` subagent; a *why*, *what was decided*, *what went wrong* or *what to do* is a question the code can never be asked, so it is read from the log. Says which record kinds it consulted and says plainly when the log holds nothing. The answer is ephemeral: nothing is stored unless the person asking says it is worth keeping, and then only its code claims, with anchors, so the next ask over the same ground is a mechanical check instead of a second exploration. Read-only over the repository it is pointed at, and it can be pointed at a repository other than the one it runs in. Use when somebody wants to understand something rather than change it. Triggers on: 'cómo funciona esto', 'cómo funciona X acá', 'explicame este repo', 'explicame cómo anda', 'dónde está X', 'de dónde sale esto', 'quién llama a esto', 'por qué está hecho así', 'por qué se hizo de esta forma', 'qué se decidió sobre X', 'qué salió mal con X', 'qué pasó con X', 'hay algo escrito sobre esto', 'cómo se hace X en este proyecto', 'how does this work', 'how does X work here', 'explain this repo', 'walk me through X', 'where does this happen', 'who calls this', 'why is it built this way', 'why did we do it like this', 'what was decided about X', 'what went wrong with X', 'is there anything written about this'. It answers and stops: it never edits the repository, never authors an objective and never invokes `plan`, `work` or `debug`."
---

# ask

Answer one question about a repository, from the two places an answer can honestly
come from, and cite every sentence.

There are exactly two, and the split is not stylistic. **Mechanism** — what the
code does, where, in what order, called by whom — is regenerable: it can be read
off the code at the moment of asking, and reading it again is the only way to be
sure it is still true. **Rationale** — why it is like this, what was decided and
what it displaced, what went wrong once, what somebody decided not to fix — is a
question the code cannot be asked at all. Nothing regenerates it. That is what the
memory log is for, and it is where this skill goes for the half of the question
that the code would only let you guess at.

Most real questions are both halves at once. "How does auth work here" is a
mechanism question with a *why is it RS256* sitting inside it, and an answer that
takes only one half is either a wall of call sites with no reason in it, or a
reason with nothing to check it against. So this skill answers both, in one
narrative, and marks where each sentence came from by the shape of its citation
rather than by putting them in separate lists.

The answer is **ephemeral by default**. Nothing is written, nothing is maintained,
no later session pays anything to keep it current — which is the only honest
default for prose about code, because prose nobody rechecked is prose of unknown
truth. When the person asking says an answer is worth keeping, its *code* claims
are written to the log with the anchors they were read off. That turns the next ask
over the same ground into a check that costs one process and no tokens:
`memory anchors` says which of those claims still point at the code they were taken
from, and only the ones that moved get explored again.

Nothing here runs on its own. No step of `plan`, `work` or `debug` calls this
skill; a human invokes it, and it answers and stops.

## Hard rules

1. **Read-only over the repository it is pointed at.** Not code, not tests, not a
   file written "just to check something". The single exception is that
   repository's memory log, written only under rule 5 and only through the
   `memory` CLI. Anything you need to write for yourself goes to scratch outside
   the target repo.
2. **Every claim carries a citation, and no claim is uncited.** A code claim
   names at least one `path:line`; a log claim names the record's id **and that
   record's date**. The date is not decoration: "the code says this today" and
   "somebody decided this fourteen months ago" are different kinds of true, and
   in one narrative the citation is the only thing carrying that difference.
3. **A sentence you cannot cite does not go in the answer.** Not softened with
   "probably", not folded into a summary. It goes in the honest list of what you
   could not establish, which is worth more than a confident hole.
4. **When the log holds nothing on the subject, say so.** Never fill a *why* from
   the code: what you would produce is a mechanism dressed up as a rationale — the
   code's current shape narrated as if somebody had chosen it — and it is
   indistinguishable, to the reader, from a decision that was actually recorded.
   "El log no tiene nada sobre esto" is a complete answer to the rationale half.
5. **Nothing is written unless the person asking asks for it**, in that
   conversation, in words. Not because the claim looks durable, not because the
   answer took a while to produce. And when they do ask: **only code claims are
   written.** A claim that came out of the log is already in the log, and writing
   it again stores one fact twice under two ids with nothing saying which copy is
   authoritative.
6. **Every claim this skill writes carries at least one `--anchor`.** The rule
   lives here and the CLI does not enforce it: `memory kinds` reports
   `anchored_kinds` as `gotcha` alone, so an unanchored `review-finding` is
   accepted. It cannot be enforced without a schema version bump — requiring the
   anchor would condemn every `review-finding` already on disk — so nobody later
   should assume the CLI is guarding this. A claim you cannot anchor is a claim
   that stays in the chat.
7. **Never write a `gotcha`.** A `gotcha` exists *because the code cannot be asked
   the question* (`tools/memory/README.md:94-104`), which is exactly the opposite
   of a claim read off the code — "which is what separates it from anything stored
   merely because asking the code twice is expensive", and asking the code twice
   being expensive is the only reason anything here gets stored. `gotcha` is a
   **read** surface for this skill and never a write one.
8. **The revalidation check runs before any exploration, never after.** Step 2's
   pair is the check whose whole purpose is to decide what does not need
   exploring, so it has to run while its answer can still cancel work; run after
   `explore` it is a receipt for work already paid for. Step 6 runs `anchors`
   again after the write, and that is a different call with a different job —
   confirming what landed — which could not have run earlier because the records
   did not exist yet.
9. **There are exactly three verdicts — `still_holds`, `changed`,
   `could_not_check` — and this skill invents no fourth.** They are the same three
   `agents/explore.md` states for a revalidated finding, which is what lets the
   CLI's output be handed straight into `explore`'s contract. A claim whose lines
   moved is `still_holds` reporting the line it sits at now; "moved" is not a
   verdict and calling it one would give the system two words for one idea.
10. **A `changed` verdict is presented as suspect and never repeated as true.**
    Nobody can tell from a hash that the claim is false — the rewrite may have
    respected it — but nobody can tell that it is true either. Name the anchor
    that moved, and re-explore that claim.
11. **A diagram is ephemeral.** Where a picture genuinely helps, it is offered in
    the chat as mermaid and it is written nowhere: not into a record body, not
    into a file in the target repo, not into a scratch file that anyone would
    then be tempted to commit.
12. **This skill never checks whether a memory log exists, never runs `init`, and
    never asks permission to create one.** Any `memory` command that resolves a
    location creates it where it lands, at exit 0 and silently — a `query` as much
    as an `add` (`tools/memory/internal/memory/store.go:37`). So the first read of
    this run leaves `<target>/.ai-kit/` behind even when nothing is persisted,
    holding an empty log and a `.gitignore` containing `*` that keeps the whole
    directory out of `git status`. That cost is known and accepted: the request to
    persist is the consent for what gets *written*, and a question about an
    invisible directory is ceremony charged to the user.
13. **It answers and stops.** No objective, no criteria, no fix, no invocation of
    `plan`, `work` or `debug`. When the answer turns up work worth doing, name it
    in one line and leave the next move to the user.
14. **Conversation in the user's language; everything written to disk in
    English.**

## The memory log

State lives in an append-only log, read and written only through the `memory`
CLI. Never edit the `.jsonl` by hand.

```bash
MEMORY="${CLAUDE_PLUGIN_ROOT}/bin/memory"
[ -x "$MEMORY" ] || ( cd "${CLAUDE_PLUGIN_ROOT}/tools/memory" && go build -o "$MEMORY" ./cmd/memory )
```

### The one directory that decides everything: the target repository

This skill can be pointed at a repository other than the one it is running in,
and when it is, **the log it reads and writes is the target repository's log** —
not the log of the repo this session happens to have started in. A claim about how
some code works belongs next to that code; filed anywhere else it is a fact nobody
will ever query for.

So settle the target once, at step 1, as an absolute path:

```bash
git -C "<the path the user named, or the session's cwd>" rev-parse --show-toplevel
```

It is written bare, with no `TARGET=$(…)` around it, and that is not an oversight:
`fish` does not parse a POSIX assignment, and the session's shell may be `fish`.
Nothing is lost by leaving it off — a variable would not survive to the next
command either, and what this line is for is putting the path on stdout where you
can read it and copy it. **No block in this skill captures one command's output
for a later command**, for that same reason: run verbatim under `fish` 4.8.1 a
block containing that shape fails with `Unsupported use of '='` at exit 127, and
it fails before any of its lines have run, while under a shell that accepts the
shape the same block runs — so which of the two a turn gets is decided by
whichever shell it happens to have. Step 7's pair, where a write and the edge
that qualifies it are two runs, is where that matters most.

**Empty output means the derivation failed, and a failed derivation stops the
skill.** `rev-parse` prints nothing on stdout when the path is missing or is not
inside a git repository. Say so and ask, rather than proceeding:

> No puedo resolver el repo desde `<the path>` — o no existe, o no está dentro de
> un repositorio git. Decime la ruta absoluta del repo sobre el que querés la
> respuesta y sigo desde ahí.

Then take the answer, confirm the path printed, and use it. **What must never
happen is a `memory` call with `--dir` empty**, because an empty `--dir` is not a
refusal — it resolves the log from the current directory instead, which on step 6
means `add` creating or appending to `.ai-kit/memory.jsonl` in a different
repository, at exit 0, with nothing in the output saying which one. And not only on
step 6: every command creates the location it resolves, so a read with the flag
dropped leaves an `.ai-kit/` in that repository too. The refusal
below covers a path that *is* passed and is not a repo; it never fires on the empty
string, so this stop is the only thing standing between a mistyped path and a claim
about one repository filed in another.

`$TARGET` below is notation for the path this derivation prints, and nothing
anywhere assigns it. **Substitute that absolute path literally into every
command**: each command runs in its own shell, `$TARGET` pasted as it is written
expands to nothing, and `--dir ""` is exactly the silent fallback above. A path you
can read in the command is a path you can see is wrong.

Two flags carry the target, and both appear on every command that accepts them:

- **`--dir "$TARGET"`** resolves *which log* — `<repo>/.ai-kit/memory.jsonl`.
  Accepted by `query`, `search`, `anchors`, `add`, `link`, `graph`.
- **`--repo-root "$TARGET"`** resolves *which working tree* the anchors are
  relative to and read from. Accepted by `anchors` and `add`, and by nothing else
  — do not put it on `query` or `link`, which refuse an undefined flag.

`--repo-root` defaults to the repo the log lives in, so on the same-repo case the
two coincide and writing both looks redundant. Write both anyway. The pair then
reads identically whether the target is this repo or somebody else's checkout,
which means the cross-repo case is not a different command that somebody has to
remember to assemble under pressure — and neither value is ever left to be
resolved from wherever the shell happens to be standing.

Two measured behaviours of the target directory, so neither surprises you:

- **A directory that is not inside a git repository is refused**: exit 1 with
  `code: bad_scope`, "not inside a git repository". That is a stop, and the honest
  response is to say the path is not a repo and ask for the right one — not to
  retry from somewhere else.
- **A git repository with no log yet returns `count: 0` at exit 0, and creates the
  log while answering** — `<repo>/.ai-kit/`, an empty `memory.jsonl` and the
  `.gitignore` that hides them both. Which means `count: 0` still cannot
  distinguish *nobody wrote anything about this* from *nothing was ever written
  here at all*. Never report the second: what you know is that the query came back
  empty, and that is what to say.

### What this skill writes, and under what

```
--plan repo-questions   --kind review-finding   --anchor <path>:<first>-<last>
```

`--plan repo-questions` is **fixed**, and it is not whatever objective tree is in
flight. A claim about how the code works belongs to no objective and outlives every
one of them; filed under a tree's slug it would disappear the moment that tree was
finished, and it would pollute that tree's findings while it lasted.

`--kind review-finding` is the existing kind, reused deliberately. None of the
kinds means "a fact about how the code works, derived by reading it", and adding
one for a single writer of unknown frequency was weighed and declined. What that
costs is real and worth knowing: these records share a kind with the findings
`review` produces, so a `--kind review-finding` query that omits `--plan` will mix
them. Always pass `--plan repo-questions` on both the read and the write.

`$MEMORY kinds` is the authority on which kinds and edge types exist; `$MEMORY
--help` on which flags do. A command this file does not show is one of those two
reads away.

## Steps

### 1 — Settle the question, the repository, and what the question is actually asking for

Derive the target path first, by the section above, and keep it where you can read
it — it is the one value everything downstream depends on and this is the cheapest
moment to get it right.

Then read the question for its **halves**, because the routing is per clause and
not per question. The test is the one this tree already uses: ask whether the code
could be asked this at all.

- **Mechanism** — how does it work, where does it happen, what calls what, in what
  order, where does this value come from. A carrier can be named: a file, a
  function, a line. It goes to the code, through `explore`, in step 4.
- **Rationale** — *why* is it like this, *what was decided* and what did it
  displace, *what went wrong*, *what to do* about this surface. No carrier exists.
  It goes to the log, in step 3.

Say back in one line what you understood the question to be and which halves you
read in it, then go — this is a readback, not an interview. A question that is
genuinely ambiguous about *which repository* or *which of two surfaces* is worth
one `AskUserQuestion`; anything else you are about to find out yourself, and asking
costs the user a turn to tell you what the code would have.

A question with no mechanism half at all — "why did we choose X" — is legitimate
and common, and it does not get a mechanism half added for symmetry. It is
answered from the log, and step 4 does not run.

### 2 — Check what is already answered, before exploring anything

Anything persisted from an earlier ask over this ground is either still true or
worth knowing about, and finding out costs one Go process, no model call and no
tokens. It runs before `explore`, by hard rule 8.

```bash
$MEMORY query   --dir "$TARGET" --plan repo-questions --kind review-finding --about <area> --current
$MEMORY anchors --dir "$TARGET" --repo-root "$TARGET" --plan repo-questions --kind review-finding --about <area> --current
```

**The two lines carry the same filter, and that identity is the point.** A query
narrower than the check verdicts records nobody loaded; a check narrower than the
query loads records nobody verified. Either way the round describes two sets while
reading as one. Change one line's filter and change the other with it.

You need both commands because each returns half of what a reused claim is made
of. `query` returns the record whole — the title, the body and the `at` date, which
is the date the claim was established and the thing rule 2 makes you cite.
`anchors` returns the verdict and the anchor it checked, and no date and no body.
Neither alone lets you present a claim honestly.

On `--about`: the tag is what makes this pair worth running, so write claims in
step 6 tagged with both the area and the path prefix, and read here with the path
prefix of the surface the question touches (a tag ending in `/` matches by path
prefix). When you cannot guess the tagging, drop `--about` from **both** lines and
let `--plan repo-questions` bound them. If the query then withholds its records
above the count threshold, **narrow it** rather than raising the threshold: a
withheld query leaves you holding verdicts for records you cannot read.

`--current` drops what something else supersedes, and for a claim about code that
is not tidiness — step 7 supersedes a claim whose code moved, and the stale
citation is meant to stop being checked the moment its replacement exists.

Then partition what came back, by verdict:

- **`still_holds`** — reuse the claim exactly as it is, and cite it with the `at`
  date from the query, not with today's. Nothing re-explores it and nothing
  rewrites it. An anchor whose `found_at` differs from its stored `line` is text
  that moved; that is `still_holds`, it needs nothing from you, and what you cite
  is the line in `found_at` — the one a reader opening the file today will land on.
- **`changed`** — the cited lines were rewritten. This claim goes to `explore` in
  step 4 as a prior finding, and until something settles it, it is suspect: name it
  with the anchor that moved and never state it as true. A pure reformat of the
  file lands here too — nothing is normalized before hashing, by design — and that
  is not a reason to wave it through, because the check cannot tell a reformat from
  a rewrite and neither can you without looking.
- **`could_not_check`** — the file is gone, or the anchor cannot be resolved. Same
  weight as `changed`, never rounded up to `still_holds`: an unchecked claim
  reported as a confirmed one is the one failure that makes the whole record
  worthless.

Read `matched` and `checked` side by side before you say anything about coverage.
`checked` counts the records that carried an anchor at all, and a `counts` block
full of `still_holds` says nothing about the ones that were never checkable.

Say in one line what this pair returned, including nothing: a filter that ran and
found nothing is different from a step that was skipped, and on a first ask over a
repository nothing is the ordinary answer.

### 3 — Ask the log for what the code cannot be asked

Choose the kinds per question. **There is deliberately no fixed set**, because a
fixed one is either too wide — every kind in every query, most of them returning
noise the user pays for — or too narrow, and quietly missing the one record that
answers the question. Kinds worth weighing, with what each actually holds:

- **`adr`** — a decision with consequences that outlive the objective that made
  it. First stop for "why is it built this way".
- **`decision`** — a choice taken, with what it displaced. That displacement is
  usually the real answer to "why not X".
- **`gotcha`** — a constraint the code obeys without stating it, written because
  reading the code will never recover it. The highest-value record in the log for a
  *why*, and by rule 7 read-only here.
- **`postmortem`** — a lesson that generalized past the bug it came from. First
  stop for "what went wrong".
- **`wont-fix`** — a risk somebody looked at and decided not to cover, in their
  own words. The answer to "why doesn't this handle X" is often here and nowhere
  else.
- **`dead-end`** — an approach tried during implementation and abandoned. The
  answer to "why don't we just…".
- **`hypothesis`** — what a `debug` investigation tested and what the verdict was.
  A rejected one is a branch somebody already walked.
- **`open-question`** and **`assumption`** — what is knowingly unsettled on this
  surface. Worth consulting whenever the question is *what to do*.

```bash
$MEMORY query  --dir "$TARGET" --kind <the kinds you chose> --about <area> --current
$MEMORY search --dir "$TARGET" "<a distinctive phrase from the question>"
```

**No `--plan` on these**, and that is a judgment worth stating: rationale about a
surface may have been written under any objective tree, and an ADR in particular
outlives the objective that wrote it. Bounding by tree here would hide exactly the
record that answers the question. `--plan repo-questions` bounds step 2's pair,
which is this skill's own writing, and nothing else.

**A rationale read breaches the count threshold easily, and the response is to
narrow rather than to raise it** — the same discipline step 2 applies to its pair.
Three kinds over a surface with any history is enough: measured against this
repository's own log, `--kind adr,decision,gotcha --about tools/memory/ --current`
returns `count: 29`, `returned: 0`, `truncated: true`, which is no records at all at
exit 0. So narrow — one kind at a time, a tighter `--about`, or the phrase through
`search` — because the alternative reads worse than it sounds: `--limit N` hands
back the N most recent and reports only *how many* older matches it dropped, so a
decision taken two years ago is absent from the answer with nothing naming it, and
an ADR is exactly the record whose age is no argument against it. A wide
`--threshold` is the same trade paid in context instead: every body loaded into this
thread so that one of them can be cited.

`search` is the fallback when `--about` tagging does not line up with how you would
name the surface. Reach for it before concluding the log is empty, because a tag
you guessed wrong and an absence look identical.

Then **say which kinds you consulted**, in the answer, whatever they returned. That
sentence is what makes an empty result meaningful: "consulté `adr`, `decision` y
`gotcha` sobre esta superficie y no hay nada" is a finding the reader can act on,
where silence about it leaves them unable to tell a gap in the log from a step you
skipped.

Keep each record's **id and `at` date** with its content from the moment you read
it. Reconstructing them at write-up time is how a citation ends up pointing at the
wrong record — and going back for one is more awkward than it looks:

```bash
$MEMORY graph --dir "$TARGET" --from <id> --depth 1
```

`graph` is the only command that resolves an arbitrary record by id. `query` takes
no `--id`, and `state --id` answers for objectives alone — pointed at any other
record it refuses with `unknown_objective`. So the id you cite is worth capturing
when you have the record in hand rather than looking up afterwards.

### 4 — Ask the code, through `explore`

Delegate the reading. Twenty files read in this thread is twenty files of context
spent where the answer needed three, and this thread's job is the narrative.

```
topic: <the mechanism half, as one question>
repo_root: <$TARGET, absolute>
scope: code
paths_hint: <what step 2 and step 3 already pointed at — the anchors of reused claims, the paths named in records>
prior_findings: <only the step 2 claims whose verdict was `changed` or `could_not_check`, each with its claim and its evidence intact>
```

`prior_findings` carries **only** the claims that failed their check. Handing it a
`still_holds` claim asks a subagent to pay for what a hash already settled, which
is the entire saving this shape exists for. It comes back in `revalidated`, one of
the three verdicts per claim, alongside the subagent's own `findings` and `gaps`.

**When step 2 answered the mechanism half completely and every verdict was
`still_holds`, `explore` does not run at all.** That is the shape working, not a
step skipped, and it is worth one line in the answer: the claims were established
on `<date>`, their anchors still hold today, and nothing needed re-reading.

Keep the `findings` with their `path:line` evidence exactly as returned. Those
anchors are the citations rule 2 requires and the anchors step 6 would write; a
paraphrase that drops them turns a checkable answer into prose.

Its `gaps` go into the answer as gaps. "I could not establish where this is set" is
information the reader needs, and rounding it into a confident sentence is how this
skill would start producing the thing it was built to replace. A second, narrower
`explore` is cheaper than widening the first one's budget when the first lands you
at a boundary you did not know existed.

### 5 — Write one answer, and cite every sentence in it

One narrative, ordered by the question rather than by where the material came from.
Not a "from the code" section and a "from the log" section: nobody reads a system
that way, and the reason the mechanism is what it is usually belongs in the same
paragraph as the mechanism.

The citation is what carries the origin, and its shape is the whole distinction:

- Code: `` `pkg/auth/token.go:88-94` `` — at least one per claim, and the range is
  the lines the claim actually rests on.
- Log: `` `01KZ… (2026-03-14)` `` — the record id and that record's `at` date.
- A claim reused from step 2: its `path:line` **and** the date it was established,
  said as what it is — established then, anchor checked today.
- A claim whose anchor came back `changed` and which nothing settled: named as
  suspect, with the anchor that moved, in the sentence itself.

Close the answer with what it does not cover: the `gaps` `explore` returned, the
kinds you consulted that came back empty, and anything the question asked that
neither surface could answer. That paragraph is not an apology — it is the part a
reader would otherwise have to discover by acting on a hole.

**A mermaid diagram is offered when the answer's structure is a graph.** The test:
more than two components, with the edges between them carrying meaning the prose is
reduced to enumerating — "A calls B and C, B and D both write to E" is a picture
being spelled out. A linear sequence, a list, or a claim about one file does not
earn one; a diagram there is decoration that costs the reader a second pass. Every
node in it corresponds to something the answer cites, and by rule 11 it is written
nowhere.

Then stop. If the answer surfaced work worth doing — a bug, a contradiction between
the log and the code, a decision nobody recorded — say it in one line and leave the
next move to the user. Turning an answer into an objective is `plan`'s job, invoked
by them.

### 6 — Persist, only if asked, and only the code claims

The trigger is the person asking saying this answer is worth keeping. Nothing else
is a trigger: not the claim looking durable, not the exploration having been
expensive, not a helpful offer of your own that they said "sí" to in passing.

**One record per claim, never one record per answer.** The verdict is per record:
three claims bundled into one body get one verdict between them, so a rewrite that
touches the first condemns the other two, and the two that still hold can never be
reused on their own. One claim, one record, its own anchors.

**And one record per claim means *every* claim** — not the three that felt most
important, not the ones whose anchors were easiest to draw. The enumeration is not
a matter of taste: by rule 2 every code claim in the answer already carries at
least one `path:line`, so the claims to write are exactly the prose claims that
carry one, less the two already accounted for elsewhere. A claim reused unchanged
from step 2 is on disk already, and rewriting it would leave two ids for one fact.
A claim still suspect from a `changed` verdict is not yet true enough to store, and
step 7 is where it lands once the user settles it. Everything else in the answer
that pointed at a line gets its own record.

Leaving one out is the failure this rule exists for, and it is invisible by
construction. The next ask over the same ground re-explores that claim from
scratch, at full price, having been told nothing — and nothing anywhere will ever
say it was skipped, because the anchors check can only report on records that
exist. A partial persist and a complete one print the same block of `still_holds`.
There is no verdict for the claim you never wrote.

**A diagram's node labels are presentation, not claims, and are not separately
persisted**: a label reading `clock.py:6-7` is the picture pointing at a claim the
prose already made and already cited, so it earns no record of its own. If a label
names code the prose never claimed, the answer was under-cited against step 5 —
every node corresponds to something the answer cites — and the fix is a sentence in
the prose, not an extra record.

```bash
$MEMORY add --dir "$TARGET" --repo-root "$TARGET" --plan repo-questions --kind review-finding \
  --title "<the claim in one line, stated so it could be false>" \
  --body "Claim: <what the code does, specifically>
Question: <the question this was answering>" \
  --about <area>,<path-prefix>/ \
  --anchor <path>:<first>-<last>
```

The body carries no "read from `path:line`" line, and leaving it out is deliberate:
the anchor is a structured field and every `query` returns it with the record, so a
prose copy is one more thing to drift out of step with the anchor that is actually
checked. What the body adds is the **question**, which nothing else on the record
holds — it is what tells a later reader at what altitude the claim was made and
therefore whether it answers theirs.

Four things about that write are load-bearing:

**The title is the index.** Kind plus title in chronological order is how these are
read months later, so "Token validation rejects an unexpired token whose kid is
unknown" and not "Notes on token validation". Stated so it could be false is what
makes a later reader able to check it at all.

**The anchor is required by rule 6 and by nothing in the CLI**, and how tightly you
draw it decides whether this record is worth having. Cite the lines the claim
actually rests on — the range is what a later `changed` verdict is measured
against. Too wide and every unrelated edit inside it reports `changed`, and a
record that cries wolf gets ignored, which is worse than not writing it. Too narrow
and the rewrite that breaks the claim happens on the line after the range. Repeat
`--anchor` for a claim that rests on two places rather than stretching one range
over both.

**`--about` gets both the area and the path prefix**, ending the prefix in `/`.
That is what step 2's pair will filter by on the next ask, and a claim nobody's
filter reaches is a claim nobody reuses.

**Nothing log-derived is written**, by rule 5, and no diagram goes into a body, by
rule 11.

Then confirm what landed, rather than trusting that `add` printed JSON. Count the
answer's code claims first — that number is what this command has to reconcile
against, and counting it afterwards from the records you wrote proves nothing:

```bash
$MEMORY anchors --dir "$TARGET" --repo-root "$TARGET" --plan repo-questions --kind review-finding --about <exactly what step 2's pair carried> --current
```

**This is not a new filter — it is step 2's, copied.** If step 2 read with a path
prefix, this line carries that path prefix; if step 2 dropped `--about` from both
its lines, this line drops it too. It is the same rule that binds step 2's own two
lines, one step later and for the same reason: the number this command is compared
against was produced over that filter, and a check narrower or wider than the query
that set the baseline compares two sets while reading as one.

`checked` equal to `matched` says every record under the filter is anchored. A
`matched` above `checked` means something went in unanchored — say so and fix it by
writing the anchored version, since nothing already written is ever edited.

`counts.still_holds` equal to `checked` is the ordinary case and not an invariant.
A claim step 2 returned `changed` that the user never settled is still on disk and
still `--current` — step 7 supersedes it only once they confirm a revision — so it
reports `changed` here too, and it belongs in `counts`. It is expected, not a
missing claim. What this block has to show is that the records *this answer wrote*
are anchored and resolve against the code right now, not that every record under
the filter does.

`matched` is what makes the completeness rule checkable instead of merely asserted.
Step 2 reported a `matched` over this same filter before anything was written, so
`matched` here must equal that number plus one for each code claim this answer newly
made. (Step 7's revision is the one case where the arithmetic does not move —
`--current` drops the superseded record as its replacement enters, one in and one
out.)

Short by one is a claim that never landed — but **name the absent claim before
writing anything.** Set the records you wrote against the answer's cited claims and
identify which cited claim has no record. A shortfall you cannot pin to a named
claim is not an omission, and writing "the missing one" on that basis appends a
second record for a claim already on disk, which an append-only log cannot take
back: two ids for one fact, with nothing saying which is authoritative. That is a
worse outcome than the omission this reconciliation exists to catch.

Then say the reconciliation in the confirmation line — "six code claims, six
records, all `still_holds`" — because it is the only part of the persist a reader
can check after the fact, and a count one short reads exactly like a complete one
until somebody states it.

### 7 — When a repeat ask revises a claim

**A repeat ask over ground where every verdict came back `still_holds` writes
nothing at all.** The records already describe the code; appending a copy to carry
today's date would spend a record to say that nothing changed, and it would leave
two ids for one fact — the same failure rule 5 avoids on the log-derived side.

The one case that writes is a claim whose anchor came back `changed`, which step 4
re-explored, and whose revision **the user confirms**. Then it is a new record
superseding the old, exactly the way a re-anchored `gotcha` is:

```bash
$MEMORY add --dir "$TARGET" --repo-root "$TARGET" --plan repo-questions --kind review-finding \
  --title "<the claim as it is true now>" \
  --body "Claim: <what the code does now>
Question: <the question this was answering>
Revised because: the previous claim's anchor came back changed at <path:line>" \
  --about <area>,<path-prefix>/ \
  --anchor <path>:<first>-<last>
```

```bash
$MEMORY link --dir "$TARGET" --type supersedes \
  --from <the id the add above printed, at .record.id, pasted literally> \
  --to <old-id>
```

Two runs, with nothing captured between them, for the reason the target-path
derivation above gives — and here the stakes are higher than a failed
derivation. The edge runs from the new record to the one it replaces. Without it
both records answer the same `--current` query and a later ask cannot tell which
one describes the code — with it, the stale citation stops being checked the
moment the replacement exists. The old id is not repeated in the body for the
same reason the anchor is not: the edge is what carries it, and
`$MEMORY graph --from <new-id>` walks to it. What the body adds is *why* the revision happened, which no edge holds.

The user's confirmation is not a formality. A `changed` verdict says the lines
moved, not what is true now, and writing a revision they never agreed to would put
this skill's own reading on disk as though somebody had checked it.

## Key principles

- **The citation is the claim.** A sentence about code with no `path:line`, and a
  sentence about a decision with no record id and date, are both just prose — and
  prose of unknown truth is the thing this skill exists to stop producing.
- **The code is asked for mechanism; the log is asked for everything the code
  cannot be asked.** A *why* answered from the code is a guess wearing a citation.
- **An empty log is an answer.** Say the kinds you consulted and say they came back
  with nothing. The gap is information; filling it is not.
- **Ephemeral by default.** Nothing stored is nothing to go stale, and the only
  claims worth the maintenance are the ones somebody asked to keep.
- **A stored claim is a checkable claim or it is not stored.** The anchor is what
  makes the second ask cheap and what makes a stale answer say so instead of
  repeating itself as true.
- **The revalidation check runs before the exploring.** That order is the entire
  saving; the other order is a receipt. Step 6's confirming run is a different
  call and is not what this is about.
- **`changed` means suspect, never false and never true.** The hash knows the lines
  moved and nothing more, and a human is the one who gets to decide.
- **This repository is read, not touched.** The only thing written is its log, only
  on request, and only through the CLI.
