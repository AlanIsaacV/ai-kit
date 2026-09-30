# memory

An append-only knowledge log, and the retrieval contract that keeps it from
flooding an agent's context.

`memory` knows nothing about skills, plans, phases or work loops. It stores
typed records and the edges between them, validates them, and answers queries.
Everything about *how* a workflow uses it lives in the skills. That boundary is
deliberate: it is what lets this tool be lifted into its own repository later
without rewriting a line of it.

## Why it is shaped like this

**Append-only, always.** No command rewrites, reorders or deletes a line that
is already on disk. A correction is a new record; an invalidation is a
`supersedes` edge. If the log could be edited it would stop being a record of
what was actually decided, and start being a record of what someone later
wished had been decided.

**Relations are their own lines.** An ADR written today can be fed tomorrow by
a deviation nobody had seen yet. If the relation lived inside the ADR's record,
recording it would mean rewriting that line — so edges are separate records.

**Validity is derived, not stored.** There is no `still_valid` field, because a
mutable field in an append-only log is a lie waiting to happen. `--current`
drops whatever the graph says is settled: a record something `supersedes`, and
a question or assumption something `answers`. Both are equally irrelevant to
"what do I still have to hold in mind", which is the only question `--current`
is for. Without the flag the history is intact, answered questions included.
**The graph it asks is the store's and not one log's**, so an edge written in
another repository settles a record here — a claim about a record is a claim
about that record wherever the line saying so happened to land.

**That last sentence is a reversion.** The settled set was one log's edges until
the `CHANGELOG.md` entry *Reads cross repositories, and so does a settling edge*;
widening it had been recorded there as REJECTED, for now, on the grounds that it
moves what every existing `--current` read in every repository returns and nobody
had taken that decision. The user has since taken it, and that entry is what
reversed it.

**Every record says which schema it was written against.** A record is judged
by the rules of the version it declares, not by the rules in force today. That
is what makes the schema movable at all: without it, the first time a value is
dropped from a closed set, every record that used it becomes invalid, and the
only ways out are rewriting history or keeping dead values alive forever. See
[Changing the schema](#changing-the-schema).

**ULIDs, not sequential numbers.** `ADR-0001` assumes one writer. Two people in
one repo, or two repos merged later, both produce an `ADR-0001` with different
content. A ULID is generated locally, sorts chronologically, and cannot
collide — so promoting a record between scopes changes one field and rewrites
no references anywhere.

**A query reports its count before it hands over records.** This is the point
of the whole tool. An agent that asks a broad question and receives two hundred
records has spent its context on history it did not need. Over the threshold,
`query` returns the count and a hint and *no records*, so the caller either
narrows the question or delegates the reading. Handing back a partial slice
would read as "here is the answer" when it is a fragment of one.

## Scopes

```
<repo>/.ai-kit/memory.jsonl   repo   everything this repo's work has produced
~/.ai-kit/memory.jsonl        user   distilled across repos, promoted deliberately
```

**Those two paths name logs, not files.** Every record on the machine lives in one
SQLite store — `/srv/projects/.ai-kit/memory.db` — and a log is the rows in it
carrying that path as their `source_path`. Nothing there is ever rewritten: the
table is insert-only, so a correction is a new row and an invalidation is a
`supersedes` edge, exactly as it was when a log was a file. `--store PATH` points
any command at a different store, which is what makes a throwaway one possible;
with none given every command uses the default above.

Both ignore themselves. The first command that resolves a location creates the
`.ai-kit/` directory — on a read as much as on a write — and writes a
`.gitignore` containing `*` inside it, which covers everything the loop puts
there. No `memory.jsonl` is written beside it: the path is the log's name and the
rows are in the store. So a repository that has never run this loop needs no
setup, and its own `.gitignore` is never edited. On a read the creation is
best-effort: a repository nobody can write to still answers `query` with what its
log holds, and only a write refuses when the directory cannot be made. Promotion
between the two scopes is always an explicit act.

**One log per repo, and the plan lives in the record.** An entry carries
`origin.plan`, and `--plan <slug>` is a filter over that field — not a choice of
file. An entry written *without* one belongs to the repository rather than to any
plan in it, and every plan-bounded read returns it; see
[records the repository owns](#records-the-repository-owns).

**`--scope` names the one log a command writes to or filters by**, and those two
are its only values: a scope is a value carried on every row, not a second file to
be opened, which is why nothing has to exist at either path above for `user` to
mean something. Where a command's answer spans the store there is nothing left for
the flag to choose, and naming it is [a refusal](#usage). Two commands read both
logs at once: [`areas`](#which-areas-a-log-answers-for) reads them reported
apart, and [`query --scope both`](#asking-both-logs-at-once) reads them as one
answer. [`state --text`](#what-is-waiting-on-you-across-every-log) reaches every log the
store holds, because "what is waiting on me" is the one question whose answer is
not about the repository you are standing in. It creates nothing and writes
nothing.

There used to be a third scope, `plan → <repo>/plans/<slug>/memory.jsonl`, and
removing it is what [schema version 2](#changing-the-schema) is. One file per
plan was not a tidier arrangement of the same graph, it was a smaller graph: an
edge needed both of its endpoints in the log it was written to, as the rule then
was, so nothing in one plan's log could ever point at anything in another's. Any
two records in a repo can now be linked, whatever plan produced them.

## Record kinds

| Group | Kinds | What they are for |
|---|---|---|
| Core | `objective` `decision` `deviation` | Make the work loop run |
| Learning | `assumption` `open-question` `wont-fix` `dead-end` `hypothesis` | Stop verification theater and repeated work |
| Knowledge | `adr` `postmortem` `review-finding` `gotcha` `preference` | Outlive the plan |
| Pending | `idea` | Something you want to do some day, kept until it is taken up or dropped |

Which kinds [promotion](#promotion) will sweep as a class is a set of its own and
is deliberately not this group; any record still promotes by id.

`wont-fix` earns its place: once you decide a risk is not worth covering, that
decision is recorded, and no later verification pass raises it again.

`hypothesis` is a claim under test during an investigation, written with how it
was tested, a verdict, the evidence and that evidence's fidelity. **A confirmed
root cause is not a kind of its own** — it is the hypothesis whose verdict is
`confirmed`, because a second kind would store one claim twice under two names.
The rejected ones are what a later session most often lacks: the record of what
has already been eliminated. It is not `dead-end`, which means an approach
abandoned during implementation, and it is not `postmortem`, which is a closing
artifact and a lesson that generalizes past the bug it came from. It was
[widened into version 3](#the-versions) in place.

`gotcha` is a *why* that reading the code will never recover: a constraint the
code obeys without stating it, fixed by something the file cannot show — a retry
count set by somebody else's rate limit, a lock order set by a deadlock that
already happened. It is not `decision`, which presupposes an option was taken and
says what it displaced; what gets written here often had no alternative. **A
`gotcha` is written because the code cannot be asked the question**, so nothing
regenerates it — which is what separates it from anything stored merely because
asking the code twice is expensive. It was [widened into version 3](#the-versions)
in place, after `hypothesis`.

**It is the one kind that is refused without an [anchor](#anchors).** Nothing
regenerates a `gotcha`, so an unanchored one is a claim about code that no later
session can ever check. `memory anchors` is what checks it, and a `gotcha` whose
code was rewritten is the case that has to reach a human.

Which surface writes one: `implement` does, inline, at the moment the constraint
is discovered, and `bootstrap` does from its interview, for a repository the loop
has not worked in yet and whose traps are only in its owner's head. The rule that
decides when a *why* is written as a `gotcha` at all is stated in
`agents/implement.md`, and `skills/bootstrap/SKILL.md` states it for that one
interview case and for no other.

`preference` is how things are done here, and it still holds: the library, the
tool or the pattern this repository settled on, written with the reasoning that
makes it worth carrying to the next one. **What the manifests can be asked is
never written here.** "This repo uses GORM v1.25" is derivable from the
repository at the moment of asking and is therefore not a record; *why* GORM and
not sqlc is what has nowhere else to live. It is not `decision`, which presupposes
an option was taken and says what it displaced — a preference may have been the
only thing anybody tried — and it is not `adr`, which is a synthesis fed by one
objective's decisions and deviations where this outlives every objective in the
repository. It was [widened into version 3](#the-versions) in place, after
`gotcha`, and it is **not** anchored: it is about a choice rather than about code
that exists, and one promoted to the user scope could carry no `path:line` that
means anything outside the repository it came from.

Filing preferences under `decision` with an `--about stack` tag was considered
and rejected on mechanics. `--about` is freeform and unvalidated, so the
distinction would be a convention that can be forgotten in silence and added by
accident, while `--kind` is a closed set the validator enforces — and the loop
writes a `decision` at every objective's close, so a preference would be a needle
in that haystack.

It is a kind written [without a plan](#records-the-repository-owns), which is
also where the convention for reading two contradictory ones lives.

`idea` is something you want to do some day, noted in one line with no planning
behind it: a title and, if you want one, a body — no priority, no due date, no
draft and no criteria. It lives in the log of the repository it is about, or in
the user log when it is about none, and it is listed under **Ideas** in
[what is waiting on you](#what-is-waiting-on-you-across-every-log) until some
record `answers` it — the objective that takes it up, or a `decision` that drops
it. That is the rule `open-question` already follows, so closing one needed no
new edge type and no new state. It was [widened into version 3](#the-versions) in
place, after `preference`; it is not anchored, because it is about work that does
not exist yet, and it is not in the set `promote --kind` sweeps, because an idea
about no repository in particular is written to the user log directly.

There used to be a `unit` as well, for a piece of work under an objective, and
removing it is what [schema version 3](#changing-the-schema) is. An objective may
derive from an objective, so the work under one is another objective and the kind
named a distinction the tool no longer draws. The records already on disk that
carry it declare version 1 or version 2 and are still judged by the rules those
versions state, where the kind is valid.

## Records the repository owns

Most records come out of one objective and say so. A few are facts about the
repository itself, and they are written with **no `--plan`** and no
`AI_KIT_PLAN`. Three conventions govern them, and none of them is enforced by the
validator — `origin.plan` is `omitempty` and no version's rule set has ever
looked at it, so all three live here rather than in a refusal.

**Which kinds are written without a plan.** `preference` and `idea` are the two,
today. The test is whether the record would still be true after every objective in
the plan directory is closed and forgotten: a preference would, and so would an
idea, which by definition came out of no objective; a decision taken under an
objective would not. Nothing stops another kind from being written this way, and
nothing should — an `open-question` about the repository as a whole is the
obvious next candidate — but a record that *could* name the objective it came out
of and does not is a record that has thrown information away.

**A plan-less record is in scope for every plan-bounded read.** `--plan X`
returns X's records **and** the repository's; it does not return another plan's.
That is what "belongs to the repository" has to mean if it is to mean anything:
one log holds every plan's records, so a record nobody's filter admits is a
record nobody reads. The cost is stated rather than hidden — a bounded read now
returns a little more than the turn asked for, and there is no flag to turn that
off.

**A preference is keyed by its first `--about` tag, and the newest one carrying
that tag is the live answer.** No `supersedes` edge is required, and
`query --kind preference --about persistence --current` puts it first, because
[`query` returns most-recent-first](#usage) and an id is a ULID. The edge still
works and is still worth writing when the writer knows which record they are
replacing — it drops the older one from `--current` outright. But preferences
arrive months apart from writers who do not know, and without this convention two
contradictory ones would both come back current with nothing saying which is
live.

Its known soft spot, written here so the session that hits it does not think it
found a bug: **a tag's content is not validated.** Two writers can key the same
question as `persistence` and as `database`, and neither will shadow the other.
That was chosen over a required `topic` field on the kind, with the weakness
known. Nor is the *position* validated: `--about` matches a tag wherever it sits
in the list, so "first" is a convention for whoever writes the record and is
nothing at all at read time — a preference tagged `orm,persistence` competes for
"newest wins" with one tagged `persistence,migrations`. What makes it stronger
than a free-floating convention is that `--about` is already required with at
least one non-blank tag, so no preference can exist without a topic — only with a
badly chosen one.

## Which areas a log answers for

That soft spot is what `areas` is for. `--about` is freeform, so a reader who
guesses a tag wrong gets `count: 0` — byte-identical to the answer meaning nobody
ever wrote one. `areas` lists the tags that exist, so those two stop being the
same answer:

```bash
memory areas                      # every tag in both logs
memory areas --kind preference    # only the tags preferences are keyed by
```

```json
{"area": "persistence", "records": 2,
 "live": {"id": "01K…", "kind": "preference", "slug": "persistence-is-sqlc",
          "title": "Persistence is sqlc", "at": "…", "key": true}}
```

- **Every distinct tag is a row**, wherever it sits in the record's `--about`.
  A preference tagged `migrations,db` is findable under `db` by a reader who came
  looking for that, and `key` is what says which of the two the record claims to
  be about. So the [keying convention](#records-the-repository-owns) is reported
  and still not enforced.
- **`records` counts every record carrying the tag**, settled ones included,
  because how many times an area has been answered is what a writer about to
  answer it again wants to know. `live` is the separate question: the most recent
  record carrying the tag that survives `--current`, which for a `preference` is
  exactly the convention above. It is `null` when every record carrying the tag
  has been superseded — an area answered and retired is not an area nobody wrote
  about.
- **Both logs, always, reported apart.** This is the one command with no
  `--scope`: it has nothing to choose between. Each scope is resolved on its own
  and carries the path it resolved to, and nothing merges the two — the whole
  point of the user log is that what is in it came from somewhere else. A machine
  that has never promoted anything gets a user scope with zero areas and exit 0,
  never a missing section, so "I never promoted anything" stays distinguishable
  from "I did not look there".
- **`--plan` bounds it exactly as it bounds `query`**, which means a record the
  repository owns is enumerated under every plan. `--kind` narrows it, and the
  counts follow.
- **No bodies, so no threshold.** Every row names one record and returns none, and
  this is the cheap read that tells a caller how to narrow the expensive one —
  withholding it over 25 areas would defeat the point.
- **Tags are folded to lower case**, since that is how `--about` compares them: a
  query for `persistence` returns a record tagged `Persistence`, so listing the
  two as separate areas would contradict the count that query gives back.
  Nothing else is merged — two tags that mean the same thing stay two areas, and
  making that visible is as far as a read-only enumeration should go.
- **A tag carrying leading or trailing spaces is reported with them**, so a row
  name can begin with a space. Only a hand-edited log holds one: `add` trims
  every `--about` value on the way in. Be warned that such a row is **not
  reachable through `query --about` in either spelling** — the flag is trimmed
  before it is compared and the stored tag is not, so the padded form never
  reaches the comparison and the trimmed form is not what is stored. `areas`
  reports what is on disk rather than a tidied spelling that would be no more
  queryable; which side should trim is open question
  `01KZHJ6EG06DVHXX2ZCMW3EWX8`.

## Promotion

`promote` copies records from a repo log up to the user log. It is the only
operation that crosses logs, it is always explicit — nothing is promoted on a
rule, a schedule or an inference — and it only goes upward: `--to repo` is
refused, because since [version 2](#the-versions) a plan's records are already in
the repo log and there is nothing below it to lift them out of.

```bash
memory promote --kind preference --to user --dry-run   # what would move
memory promote --kind preference --to user             # move it
memory promote --ids <id>,<id> --kind adr --to user    # both, as a union
```

**Four kinds may be named as a class**, and the set is published by
`memory kinds` as `promotable_kinds`:

| | |
|---|---|
| `adr` `postmortem` `preference` `dead-end` | what is worth carrying to a repository the work has not reached yet |

It cuts across the groups in [record kinds](#record-kinds) rather than following
them. `dead-end` is a Learning kind and belongs here — what was tried and did not
work is the most expensive thing to rediscover. Three Knowledge kinds are absent,
each for its own reason:

- **`decision`** because the loop writes one at the close of every objective.
  Sweeping the class would carry hundreds of records about *this* repository's
  code into a log meant to hold what outlives it.
- **`gotcha`** because it cannot exist without an [anchor](#anchors), and an
  anchor is resolved against the repository the record's log lives in — or, once
  it is in the user log, against the checkout `origin.repo_id` names. Either way
  it is a claim about one repository's code: a `path:line` means nothing in the
  next repository.
- **`review-finding`** because it names a defect in one repository's code at one
  moment, and what settles it is fixing that code.

Naming a kind outside the set is refused by name, with the four listed in the
refusal — including a kind that is perfectly valid to write, like `decision`.
A promotion that silently moved nothing would be indistinguishable from a class
that happened to be empty. **The set is not a validation rule**: nothing here
stops `--ids` from carrying any record at all upward, including a `gotcha` whose
anchor will not resolve there.

**`--kind` and `--ids` are a union.** Either alone is enough and both together
promote everything either one named; a record named twice is promoted once. Only
naming neither is refused.

**It reaches one hop.** A record nobody named but that an edge joins to one that
was named travels too, because an `adr` that arrives without the `decision` that
fed it arrives as an assertion. One hop and not the transitive closure: the
closure is the mechanism that would lift a whole repository's scaffolding into
the user log. The hop is measured from the named set only — a record pulled in at
one hop does not go on to pull in its own neighbours, whatever order the edges
sit in the log.

**Except two kinds the reach never pulls in**, because the expansion was
otherwise a back door into the user log for exactly what the four-kind set keeps
out of the front one:

- **`objective`.** An `adr` very often derives from the objective that produced
  it, and following that edge would carry a repository's scaffolding upward. An
  objective names work that happened in one repository; the ADR it produced is
  what was meant to outlive it.
- **`gotcha`.** It cannot exist without an [anchor](#anchors), and an anchor's
  `path:line` is resolved against the repository the record's log lives in, or
  against the checkout `origin.repo_id` names once it is in the user log — a claim
  about one repository's code, which means nothing in the next one. `--kind`
  refuses the class for that reason, and an edge into one would have carried it up
  regardless — anchor and all.

This bounds the *reach* and not the *selection*: either kind named in `--ids` is
promoted like anything else, because a caller typing an id has decided. And
neither exclusion is silent — a record the walk skipped is reported in `border`
and the edge to it counted in `edges_skipped`, exactly like any other record left
behind.

**And it says where it stopped.** The report carries `border`, with
`border_count`, and `edges_skipped`.

Read them carefully, because they are **about the destination and not about this
run**. The test is whether a record will be over there when this promotion is
done — selected by it *or already present* — so an edge with one end at the
destination and one end not is counted, and the end that is not is named,
whatever run left it that way. Promoting an unrelated `preference` will report a
record some earlier promotion left half-connected.

That is deliberate, and it is the honest answer to the question worth asking of a
log nobody can tidy: *what is still half-connected over there?* The narrower
reading — what this run in particular left behind — goes stale the moment you
stop reading the reports, and nothing would ever tell you again. **The cost is
that the border list grows as the user log fills**, and there is no flag to
narrow it.

An edge with *neither* endpoint at the destination is not counted: that number
would be a measure of the rest of the graph. And skipped edges outnumbering
copied ones is not a refusal — the command proceeds and reports it, because the
alternative is a promotion that decides for you what you meant.

The [both-endpoints rule](#guarantees-under-test) is untouched by any of this.
The expansion adds *records* so that more edges qualify; it never admits an edge
whose endpoint is absent, and a promoted record never arrives with a dangling
edge behind it.

**`--dry-run` prints the report a real run would print**, plus `dry_run: true`,
and writes nothing — not the records, not the edges, and not the destination
directory the real run would have created. It is optional and never a required
confirmation step: a mandatory one would break every non-interactive caller.

Running the same promotion twice is a no-op. Ids are preserved, so a record
already at the destination is reported as `already_present` and not written
again, and an edge already there is deduplicated by type-from-to. That includes
records pulled in at one hop: whatever the first run moved, the second one names.

### Which repository a record came from

`origin.repo` is a directory name, and two checkouts both called `api` are one
string in a log holding records from both. So a record also carries
`origin.repo_id`:

1. the `origin` remote's URL, where the repository has one — it survives the
   repository being moved, cloned, or checked out twice on one machine;
2. the repository's absolute path otherwise, which survives none of those and is
   the fallback for exactly that reason;
3. absent, where neither could be read.

Absent is a claim and not a hole: it says nothing about which repository, which
is exactly what a record written before the field existed says. A sentinel would
have made those two look like different answers. `--repo` overrides the *label*
and not the identity — a caller may rename what a record calls its repository,
and nothing it can type makes two repositories the same one.

It applies to records written from now on. **Nothing is backfilled**, because the
log is append-only. Which remote is `origin` and only `origin`: that covers a
repository having none, and does not decide between several under other names.

## Anchors

A record may cite the code its claim rests on, as a field rather than as a
`path:line` in the body:

```json
"anchors": [{"path": "pkg/auth/token.go", "line": 41, "lines": 3,
             "hash": "sha256:6afeb2dc…"}]
```

`path` is repo-relative, `line` is 1-based, `lines` counts the line it starts on,
and `hash` is a digest of those lines exactly as they were. It is a field so that
nothing depends on every body being written the same way, and it is optional on
every kind — only [`gotcha`](#record-kinds) is refused without one.

`memory add --anchor pkg/auth/token.go:41-43` builds it, reading the file at that
moment. **A citation that names nothing is refused at write time**, because the
log is append-only and this is the only place a typo can still be caught; left to
the check it would surface months later as a `could_not_check` indistinguishable
from a file somebody deleted. The flag repeats for more than one anchor.

**A path says where in a repository, never which repository.** So `memory anchors`
finds the repository each record came from and reads its anchors there, rather
than in whatever directory the command happens to run in:

- **A record in a repository's log is checked against the repository that log
  lives in** — the directory holding the log's `.ai-kit/`. Never against the
  record's own `origin.repo_id`: a line keeps the identity it was written under,
  and a checkout that has since moved or been cloned under another name is still
  the directory its log sits in now.
- **A record in the user log is checked against the checkout its
  `origin.repo_id` names.** An identity that is itself an absolute path to an
  existing directory is that directory. Otherwise it is looked up among the
  repository logs the store holds, by the identity each one was stored under: one
  match is that log's repository, and where several share the identity — a
  checkout and its git worktrees — it is the one whose `.git` is a directory
  rather than a file.

A record whose repository cannot be located is `could_not_check`, and every one of
its anchors carries a `note` naming which case it is:

| Case | The note names |
|---|---|
| the record carries no `origin.repo_id` | that nothing says which repository its anchors were read out of |
| the identity matches no repository log in the store and is not an existing directory | the identity and the store |
| several repository logs share the identity, and not exactly one of their roots has a `.git` directory | the identity, how many roots qualify, and every candidate |

**It is never checked against the directory the command runs in instead**, not
even when that directory holds a file at the cited path. A verdict read off the
wrong checkout is a false `still_holds` or a false `changed`, and both read as a
fact about the code. A repository that was located but is no longer on disk is a
file that could not be read, and says so the same way.

The answer says where it looked. When one root serves every record — a
repository's log, or any run naming `--repo-root` — it is `repo_root` at the top
of the answer, as it has always been. Under `--scope user` without the flag each
record carries its own `repo_root`, and a record that could not be located carries
none. **`--repo-root`, when named, decides the root for every record** whatever
scope it came from, so a promoted record can still be read against a checkout of
your choosing.

**Which records are checked is this repository's log**, or the user log under
`--scope user` — unless `--all` is named, which checks every record the store
holds, each against the repository its own log lives in:

```bash
memory anchors --all --kind gotcha --current   # every repository on this host, at once
```

`--all` reads each log on its own and merges the matches, the way
[`query --scope both`](#asking-both-logs-at-once) reads its two, so a record's
root is its own log's and never the reader's. It names the logs it read under
`logs` in place of `path` and `scope`, as `state --all` does; every record carries
its own `repo_root`, since no one root served the answer; and `--limit N` takes
the N most recent across every log rather than N from each. Naming `--repo-root`
alongside it still decides the root for every record. A repository whose
directory is no longer on disk makes its own records `could_not_check` rather
than refusing the run. `--scope` beside `--all` is refused: `--all` already
answers for every log, and there is nothing left for the flag to choose.

**It deduplicates nothing, where
[`state --all`](#what-is-waiting-on-you-across-every-log) deduplicates, and the
asymmetry is the two questions rather than an oversight in either.** Two logs can
hold one id — a promotion, or a checkout and one of its git worktrees — and here
each copy is checked against *its own* repository root, so the two rows are two
verdicts about two working trees and both are the answer. Collapsing them would
answer with whichever sorted first and hide that a promoted record is checked in
both places, which is the thing the per-record `repo_root` exists to show. `state
--all` asks what is waiting on you, and an objective is one thing however many
logs hold it, so it names each record once. Neither one moves: `anchors --all`
answers about code, which is per repository, and `state --all` answers about work,
which is not.

At user scope `add` also has no repo to read the cited lines out of, so
`--repo-root` supplies one. The requirement does not bend for the scope: a
`gotcha` with no anchor is refused in every scope, and what the flag changes is
that the root can be given rather than only inferred.

### Checking one

```bash
memory anchors --kind gotcha --about pkg/auth/
```

It reports, for each anchored record the filter matched, **exactly one verdict** —
the same three `agents/explore.md` states for a revalidated finding:

| Verdict | What it means |
|---|---|
| `still_holds` | the cited text is in the file; `found_at` says where, and `moved` says whether that is still the line the record wrote |
| `changed` | the file was read and the cited text is not in it anywhere |
| `could_not_check` | the file could not be read, the anchor is not one this binary can resolve, or the repository the record came from could not be located |

A record's verdict is its worst anchor's — `changed` over `could_not_check` over
`still_holds` — because a record with one rewritten anchor is one to look at
whatever its others say, and a partly checkable one must not read as checked.

**How it works, in full:** it hashes the cited lines and compares that against
every window of the same height in the file. That is the entire mechanism. No
git, no AST, no symbol lookup, no fuzzy matching — so movement is detected
without any history, which matters because the repo it is pointed at may be a
shallow clone or not a git repository at all.

**Nothing is normalized, and a reformat therefore reports `changed`.** Not
whitespace, not case, not indentation. That false positive is deliberate: it
costs a human one glance, and the tolerance that would suppress it also
suppresses the rewrites the check exists to catch. A false `still_holds` is worse
than having no record at all.

Three consequences worth stating, because each looks like an omission:

- **Text that moved is `still_holds`, not a fourth verdict.** The cited content
  is still there; the check names the line it is at now. `moved` as a verdict
  would give the system two vocabularies for one idea.
- **Text that appears more than once is `still_holds` with every match named.**
  Choosing one of them would be the check guessing which occurrence the record
  meant, and guessing is the one thing a mechanical check must not do.
- **The check never rewrites the record.** No `--fix`, no auto-updated line. The
  log is append-only and this reports only. `--repo-root` says which working tree
  to read and defaults to the repository each record came from; a value that is not a
  directory is one refusal rather than `could_not_check` on every record, since a
  typo in the flag is a fact about the command and that verdict reads as a fact
  about the code.

It makes no language-model call and no network call, opens files read-only, and
exits 0 whatever the verdicts are — `changed` is an answer, not a refusal.

### Asking for less than all of it

The whole set with its `counts` roll-up answers "how is the log doing". The three
narrower questions have flags of their own, and they compose with each other and
with every filter above by conjunction:

```bash
memory anchors --kind gotcha --verdict changed   # only the records that drifted
memory anchors --path pkg/auth/token.go          # only the records citing that file
memory anchors --id <id>,<id>                    # only these records
```

- **`--verdict` is the record's verdict, never an anchor's.** It takes one of
  `still_holds`, `changed`, `could_not_check` — one value, not a list — and
  anything else is refused naming the value it was handed. Matching per anchor
  would return a record with one rewritten citation under `still_holds`, which
  under-reports drift silently.
- **`--path` matches a case-insensitive substring** of an anchor's `path`, the way
  `--text` matches title and body. A record citing the same file twice comes back
  once, carrying every anchor it was written with rather than only the ones that
  matched.
- **`--id` takes a comma-separated list** and reports under `not_found` every id it
  did not hand back — one no record carries, one whose record carries no anchors,
  one another bound excluded — exactly as [`query --id`](#usage) does.

`checked` and `counts` stay what they are everywhere else: what the check looked
at and what it found, over every matched record that carried anchors. `returned`
says how many of those `--verdict` and `--path` admitted, and appears only when
one of them is named — so an invocation naming none of the three answers exactly
what it answered before they existed.

## Edge types

**Relations** — how two records stand to each other.

`implements` · `derives_from` · `supersedes` · `answers` · `caused_by` ·
`relates_to`

**States** — what happened to one record.

`approved_for_execution` · `met` · `cancelled` · `separated` · `blocked`

A state is an edge and not a field, for the same reason validity is derived: a
field would have to be rewritten to change, and nothing here is ever rewritten.
The absence of all five is a state too — the default one — so being in it costs
nothing, and leaving it costs one appended line and takes nothing back.

**A state edge points at the record that justifies it.** The second endpoint is
not decoration: it is the evidence, and `link` refuses an edge whose target
could not be it.

| State | May point at |
|---|---|
| `approved_for_execution` | `decision` |
| `met` | `decision`, `review-finding` |
| `cancelled` | `decision` |
| `separated` | `decision` |
| `blocked` | `open-question`, `decision` |

The refusal names both the allowed kinds and the kind that was actually there.
There is no reflexive form — an edge from a record to itself would be a state
asserted with nothing behind it, and on disk it would look exactly like one that
was earned.

The rule runs on the **write path only**, where the target's kind is in hand.
`validate` does not apply it, so a state edge already on disk is never condemned
by a requirement that arrived after it was written. The six relation types carry
no such rule at all: they describe a relation rather than a transition, and
`supersedes` already has edges on disk that no later rule should be able to call
invalid.

The allowlist is published by `memory kinds`, under `justifications`, so it can
be read without provoking a refusal to find out. Only the five states appear
there: a type with no key has no rule, which is a different claim from a type
with an empty list.

### Which state edge wins

A record accumulates state edges; it does not replace them. So a reader needs a
rule, and this is it:

> **A record's state is the most recent state edge by `at`, ties broken by order
> in the file — the later line wins. Moving between states means writing the
> state you are moving to. There is no retraction, and none is needed.**

`approved_for_execution` → `blocked` (something more urgent appeared) →
`approved_for_execution` → `met` is a normal, correct sequence, not a
contradiction to be repaired. The edges are a trace of the state machine, and
every one of them was true when it was written. A `blocked` edge sitting under a
later `met` is *stale*, not *wrong* — and that trace, "this was blocked for
three days in March", is exactly the history a mutable status field would have
destroyed the moment the block cleared.

Two consequences worth stating, because both look like omissions:

- **A state edge needs no `id`.** Nothing ever has to refer back to one to undo
  it. Recency settles it.
- **`supersedes` is not involved.** It replaces a *record* with a better record.
  A later state does not say the earlier one was mistaken, so borrowing
  `supersedes` for it would file a true statement as a correction.

And one honest limit: **returning to bare `planned` is not expressible.**
`planned` is the absence of state edges, and an edge cannot assert an absence.
In practice nothing wants to: a record leaves `approved_for_execution` for
`blocked` or `cancelled`, not for `planned`.

### Two states read off the graph rather than the edge

The rule above is what a record's own edges say. Two shapes of the graph change
the answer, and both are derived on the read path — nothing is written, and the
edges on disk stay exactly as they were:

- **A parent whose children are settled reads `met`.** An objective with at
  least one child that counts as done (`met`, or superseded) and none
  outstanding resolves to `met`, and `memory state` marks it with
  `"state_from": "children"`. Children that are all `cancelled` or `separated`
  derive nothing. A state edge of the parent's own written after its last
  child's closing edge — later by `at`, ties to the later line — wins over the
  derived `met`: that is how a parent with criteria of its own ("two weeks of
  use") stays open. An own edge written before the children closed loses to it.
  The derivation climbs the tree, so a grandparent counts a derived-`met` child
  as done and may derive `met` in turn. An objective on a cycle derives nothing.
- **A block on an answered question stops counting.** A `blocked` edge whose
  target is an `open-question` some record `answers` is left out, and the
  objective resolves from its other state edges — the `approved_for_execution`
  it had before the block, typically, or `planned` when it has none. This is
  the one way a record reads `planned` again: not by an edge asserting it, but
  by the only edge that moved it being released. A block on a `decision` is
  unchanged.

The derivation lives where state is resolved, so every reader follows it: the
report's `state` and `open`, the ledger under `--text`, `--current` (a derived
`met` is terminal and drops out), the current objective, a parent's completion
and blocked-by-an-open-child.

`memory state` is what computes it. `edges` still reports the edges themselves —
deliberately, since listing relations is a different question from resolving one
record's state.

## The objective tree

**An objective may derive from another, and that is the parent relation.** It is
a `derives_from` edge with an **objective at both ends**: positional, not a type
of its own. The same edge type also points a decision at the objective it was
taken under and an ADR at the decisions it came from, and both go on meaning
exactly that — only objective → objective is parentage. A `child_of` type would
have been unambiguous and was rejected anyway: it would widen a closed set a
second time and leave two edges asserting one relation.

**Two rules, enforced by `link`:**

- **One parent.** An objective that already derives from an objective cannot
  derive from a second, and the refusal names the parent it has.
- **No cycles.** The refusal names the whole path.

Both refusals carry the code `not_a_tree`.

Several roots are normal — nothing requires an objective to have a parent.

**An objective's parent may be in another repository.** Both endpoints still have
to exist; the store is where they are looked for, not one `source_path`. So an
objective written into one repository's log derives from an objective written into
another's, and `memory state` reads that as one tree from either end — the child
reports its parent, the parent reports its child, and the parent's completion
counts it. An id no log of the store holds is still refused `dangling_edge`. The
two rules above reach just as far: an objective cannot pick up a second parent by
being linked from another repository.

**A self-edge is refused earlier, and differently.** `derives_from X → X` is a
cycle of length one, but it never reaches the tree rule: an edge pointing at its
own source is refused by the current version's rule set first, so it comes back
as `invalid_edge` — *"an edge must not point at its own source"* — and names
neither a path nor a parent. That is the order on purpose. The refusal is what
mattered; a path reading `X -> X` adds nothing to a message that already says
exactly this; and running a graph rule over an edge that has not been validated
yet would be the worse trade.

So there are three refusals and they do not all read alike:

| Edge | Code | What the message names |
|---|---|---|
| would give an objective a second parent | `not_a_tree` | the parent already on disk |
| would close a cycle | `not_a_tree` | the whole path |
| points at its own source | `invalid_edge` | neither |

Written down because nothing branches on the code today and something might.

Like the justification allowlist, both live on the **write path only**. The read
path stays tolerant, so a log that already holds a cycle or an objective with two
parents — hand-edited, or arrived through `promote`, which passes no gate — is
read rather than condemned: `memory state` terminates, uses the first parent edge
in the file, and reports what it met under `anomalies`. `validate` does not apply
the rules and does not call a malformed tree a problem.

### Open and terminal

| Open | Terminal |
|---|---|
| `planned` · `approved_for_execution` · `blocked` · `separated` | `met` · `cancelled` |

**`separated` is open, and that is the one worth stating.** It says the work
continues but no longer under the parent it was lifted out of — not that it
stopped. So a separated objective is still returned by `--current` and is still a
candidate for the current one, while counting toward no parent's completion and
blocking none. `blocked` is open for the plainer reason that waiting is not
finishing.

`query --kind objective --current` drops the two terminal states and — as it
already does for every kind — whatever is superseded or answered. Nothing else
removes an objective: **not its depth, and not having an open child.**

### Which objective is being worked on

A different question from `--current`, with a single answer. It is reported
beside every query that admits objectives, as `current_objective` — a field of
its own, because `hint` means truncation and nothing else.

> **The open objective whose most recent `approved_for_execution` edge is the
> latest in the log. If no open objective carries one, the open objective with
> the most recent activity — the latest `at` among the objective itself and every
> edge with it at either endpoint — ties broken by the later line in the file,
> and a tie on the line too by the later entry in the file.**

The rule that resolved it comes back with it, as `rule`, so the answer is never a
guess presented as a fact. With nothing open, the field says so rather than
naming one anyway.

**The tie-break is not a corner case, and it is the real line number.** `at` is
written to the second and this loop appends a batch of adds and links at the end
of a turn, so most lines of a live log share their second with another record,
and some of those seconds mix an entry with an edge. That is why the derivation
reads the log's *lines* rather than the entries and edges split out of them: two
slices each ordered internally cannot say which of one's members came before one
of the other's, so the tie would have had to be guessed.

**And the line runs out too, in one place.** An edge touches two records, so it
bumps *both* of its endpoints to the same line — which means a `derives_from`
between two objectives leaves them tied on `at` and tied on the line as well, and
"the later line" has nothing left to separate them with. The tie then falls to
the order the objectives' own entries sit in: **the later entry wins.** It is
deterministic and stable, the same log always giving the same answer, and it
decides anything at all only when those two objectives are the two most recently
touched open ones. It is documented rather than fixed on purpose — any rule that
separated them would have to invent a reason to prefer the parent or the child,
and the edge is one fact about both of them, written once.

Hybrid, because neither half is enough alone. Activity is free but implicit: it
moves on its own and cannot be pinned. The mark is deliberate but costs a
`decision` plus an edge on every context switch, which is the ceremony this loop
exists to stop paying — and it is already available at the moment you are writing
a decision, which is what the allowlist demands behind any state edge. There is
no command for switching: pinning the focus is `add --kind decision` plus
`link --type approved_for_execution`, and both already exist.

**Depth suggests; it does not decide.** It is reported by `memory state` and
nothing filters on it. Every open objective is potentially the current one — an
urgent fix arrives, or a sibling suddenly matters more, or the draft you are
editing turns out to be the real work, and a rule forcing the deepest node would
make every one of those a fight with the tool.

The derivation reads the **whole log**, and `--plan` applies afterwards. When the
resolved objective falls outside the filter it is named anyway, with
`outside_filter`, instead of being replaced by the runner-up.

### How far along a parent is

`memory state --id <parent>` sorts that objective's children into three buckets:

| Bucket | Children |
|---|---|
| counted done | state `met`, or the target of a `supersedes` edge |
| not counted | state `cancelled` or `separated` |
| outstanding | everything else |

**Superseded counts toward the parent.** Absorbed work exists — someone else did
it — and refusing to count it would leave the parent permanently short of a child
nobody is ever going to do again.

**Blocked by an open child is derived, never written.** An objective with at
least one open, unseparated child is reported as blocked by it, with the ids of
the children. No edge says so, and `link --type blocked` pointing at another
objective is still refused — which is what keeps "blocked on a sibling" and
"blocked by my own child" from becoming indistinguishable on disk. It is
informational and nothing acts on it: such an objective is still returned by
`--current` and is still eligible to be the current one.

### What is waiting on you, across every log

`memory state --text` answers one question — *what is waiting on me* — over every
log there is, and prints it instead of emitting it:

```
What is waiting on you — 47 open objectives across 33 logs

Waiting for your approval (34)
  ai-kit             Know what is waiting on you, and that it is true  (01M2CGQ0CVSFTJ0CNB56RYGXCG)
  be-md              Persist LATAM trades from the raw-FIX feed  (01M18F1KD0JRPWXV1H91TEBMMC)

Blocked on a question or a decision (2)
  open-question: tipooperacion = A is still unanswered
    be-profits-stocks  Load the CO trade-history gap from the OyD day listing  (01M03T9F4HWVYETRTBK9RPN9CZ)

Waiting for you to start it (9)
  ai-kit             One readable summary of what is waiting, across every log  (01M2CKDPDRP09ESQ6EG27KTRRJ)

Set aside (2)
  ai-kit             Make the record schema evolvable  (01KZ34725V37YX845RTVPC7N0E)

Ideas (1)
  user               A digest of pending items every morning  (01M3KACFKX3YRCQZB4Q2A72Z8M)

Nothing waiting (19 logs)
  be-bm-fix-orders, be-cashout-co, be-deadletter, be-fix-logs, ...
```

**The default follows the output mode, and the two defaults differ on purpose.**
JSON is what a skill reads about the repository it is standing in; the text is
what you read when you are deciding where to go next, and answering that for one
repository is answering a question you did not ask.

| | JSON | text |
|---|---|---|
| no flags | this repository | every log |
| widened to every log | `--all` | the default |
| narrowed to this repository | the default | `--here` |

`--all` and `--here` together is a refusal — `conflicting_scope` — rather than a
precedence nobody would remember. **`--scope` is refused wherever the answer is
every log's** — `scope_not_on_store_wide_read`, beside `--all` and beside `--text`
without `--here`. It names one log, and neither of those is asking about one.
Where the answer is a single log's it still chooses which, as it always did.

**Which logs.** Every distinct `source_path` the store holds, in path order. There
is no registry of repositories and no scan of the filesystem: a log is in the
answer because a record was written to it, so a repository that has never run this
loop is absent rather than empty, and one whose checkout has since been moved or
deleted still reports its history. The user log joins them when the store holds
one, labelled `user`, since it has no repository to be named after.

**One report per log, and one tree across them.** Reading the logs as a single
report would produce a single "current objective" across every repository, which
answers nothing — so each log is reported on its own and every item names where it
came from. The *derivation* is store-wide: the edges come from every log, so an
objective's parent, children and completion reach across repositories, while the
objectives listed under a log are the ones that log holds and an anomaly is
reported by the log holding the record it is about. In the `--all` JSON every
objective, exclusion, anomaly and current objective carries a `repo`. That field
is `omitempty` and no single-log report ever sets it, so `memory state` with no
flags carries no `repo` anywhere.

**The single-log report says which half came from where**, because both halves are
in it and they do not come from the same place. `objectives_from` is the log the
`objectives` list was filtered out of, and `derived_over` is the store the
anomalies, the current objective and the parentage behind them were derived over.
The lists themselves are what they always were; what is new is that a reader no
longer has to know which half `path` was naming.

**Every objective names the working tree it was recorded in.** Each objective,
each exclusion and each current objective carries `root`: the absolute path of the
working tree holding the log it is listed from, in the single-log report and under
`--all` alike. So `memory state --all --id <id>`, run from any repository, says
where an objective recorded in another one lives on this host. That is all it
says — what a workflow keeps under that directory is the workflow's business, and
nothing here names a file inside it. It is read off the log's path with no look at
the filesystem, so a repository that has since been moved still reports the root
its log was written under. The user log has no working tree, and nothing it lists
carries a `root`.

**Each of the four lists names a record once.** Two `source_path`s can hold the
same id — a promotion, or a worktree of the same repository — so `objectives`,
`excluded` and `current` are each deduplicated by `(id, scope)`. By scope and
never by id alone: a promoted record's two rows genuinely differ in text, and
collapsing them would answer with whichever sorted first. `anomalies` keys on the
message as well: an objective with three parents produces two anomalies carrying
its id, and by id alone the second one — naming the second edge that was ignored —
would be dropped as a duplicate. [`anchors --all`](#anchors) does the opposite,
deliberately: it returns a record once per log that holds it, because there each
copy is a separate verdict read out of a separate working tree. The two commands
answer different questions, and the keys follow the questions.

**The grouping is read off the state and nothing else** — no body is parsed and
no prose is matched. `planned` is waiting for your approval, `approved_for_execution`
is waiting for you to start it, `separated` is set aside, and `blocked` sits under
the record it waits on, named: the title comes from the `blocked` edge's target,
so two objectives held up by the same question read as one wait rather than two.
Terminal objectives are absent, and so is anything superseded or answered.

**Ideas are listed beside the work, and counted apart from it.** Every `idea` no
record `answers` is listed under **Ideas** with the name of the log holding it and
its id, the user log included; one that something answers — wherever that answer
was written — is gone from the list. An idea is named once by its id alone, the
first log holding it owning the line, because a promoted idea is one record in two
logs rather than two ideas. A log whose only open record is an idea is not counted
under *Nothing waiting*, and ideas are not in the closing rate: they are not
questions, and a rate that means something today would stop meaning it. The
`--all` JSON carries the same list as a top-level `ideas` array, each element
`{"root", "repo", "id", "title"}`: `repo` is the log's name as the text prints it,
and `root` is that log's working tree, empty for the user log. The single-log
report gains no key.

**It creates nothing.** Every other read resolves its location and creates
`.ai-kit/` on the way past; this one takes its paths out of the store, and
`--here` locates this repository's log rather than resolving it, so asking what is
waiting leaves no directory behind in any repository it reads.

**Errors stay JSON in both modes**, `--text` included. Only the summary itself is
text.

## Changing the schema

Every record carries `"version"` — first key on the line, no `omitempty`,
because a version nobody can see is a version nobody checks. It is stamped by
the tool, never passed in: a caller that could choose would eventually choose
wrong, and a record that lies about its version is worse than one carrying none
at all. It is per-record rather than per-file, because a record survives
promotion into another log and keeps the rules it was written under. It is an
integer, not semver — there is nothing to be compatible *within* a version.

**A record with no `version` key is version 1.** The stamp arrived after those
records were written, and nothing on disk is ever rewritten to add one. It is
*printed* as 1 as well — `query` shows the version the record is judged at,
because `"version": 0` is on no line and names no schema that ever existed.

**The compatibility rule.** Within a version, changes are additive only.

- **Never rename a field, and never remove one.** The JSON key *is* the identity
  here — this is protobuf's discipline without protobuf's numbering. A rename is a
  delete plus an add, and every record already written fails it. A field that has
  gone quiet stays where it is: `Origin.Unit` is populated by nothing and removed
  by nobody.
- **Never repurpose one.** Same key, new meaning, is the only change no reader
  can detect: nothing looks wrong, everything is.
- **Adding an optional field is free.** Old records simply lack it, and a
  reader ignores keys it does not know rather than calling the line malformed.
- **Widening a closed set is additive, and happens inside the current
  version.** Adding a kind, an edge type or a scope makes no record already
  written illegal, so there is nothing for a bump to protect. It is not free,
  and the price falls on *readers* rather than on records: a binary built before
  the widening has rules for the current version, judges the new value by them,
  and reports `type: unknown` — a problem that is not one — where for a version
  it had never heard of it would have reported an honest `unjudged`. Bump anyway
  when that false alarm would cost more than the new value is worth. The
  edge-type set was widened this way, from six to eleven; see
  [the versions](#the-versions).
- **Narrowing a closed set is a version bump.** Dropping a kind, an edge type
  or a scope — anything that makes a legal record illegal — cannot be done by
  editing the current version's rules, because those rules are still what the
  records on disk are judged by.

**Writing a new version** obliges you to four things:

1. **Declare it.** `SchemaVersionN` in `record.go`, alongside the closed sets it
   changes (`scopesVN` and friends). Leave the old sets untouched — the records
   that used them are still on disk and still have to be readable.
2. **State its whole rule set.** `validateEntryVN` and `validateEdgeVN` in
   `validate.go`, naming every rule, not a diff against the version before it
   and not a table of exceptions. Reading one validator must tell you
   everything that version required, without holding another in your head.
   Compose them from the same `checks` helpers the older validators use, so
   "is this a ULID" never has two implementations that can drift apart.
3. **Repoint the present.** Register the version in `CurrentSchema`, then move
   `CurrentSchemaVersion` and whichever of `entryKinds`, `edgeTypes` and `scopes`
   the new version declares a set for — the rest keep pointing where they point,
   which is what version 3 did with the edge types and the scopes. The
   write gate reads its own rules back out of that same table, so there is no
   second list to keep in step — and if you stamp a version you forgot to
   register, the binary refuses to start instead of quietly reporting every new
   record as unjudged. The gate never dispatches: whatever the read sweep
   tolerates on disk, `add` and `link` go on refusing anything today's schema
   disallows, so a record cannot buy itself lenience on the way in by declaring
   an older version.
4. **Leave the old validator alone, forever.** Deleting it tidies nothing; it
   condemns every record that declares that version. Carrying every version's
   full rule set in the code is the price of never migrating anything, and it
   was chosen deliberately over the cheaper, lossier alternatives.

A binary that meets a version it has no rules for reports the record as
**unjudged**: counted, listed with its line number, kept out of `problems`, and
not a failure. An older binary reading a newer binary's records is a normal
state of an evolving schema, not a corrupt log — but "not checked" must never
be mistaken for "checked and fine", which is why it is reported separately
rather than passed over in silence.

### The versions

| Version | What it says | What changed |
|---|---|---|
| 1 | eleven kinds; `scope` is one of `plan`, `repo`, `user`; six edge types | The original shape |
| 2 | eleven kinds; `scope` is one of `repo`, `user`; eleven edge types | The plan scope was removed — that is what the bump was for. The edge-type set widened later, in place, by the five state types |
| 3 | fourteen kinds; `scope` and the edge types are version 2's; entries may carry `anchors` | `unit` was removed from the entry kinds. Nothing else moved. The kind set widened later, in place, by `hypothesis`, `gotcha`, `preference` and then `idea`, and the optional `anchors` field arrived the same way — with one rule that tightens: a `gotcha` without one is refused |

**Version 3 is the current version, and dropping the `unit` kind is what the bump
bought.** `entryKindsV3` started as version 1's eleven without it — the two kinds
widened in since are further down — and everything else is a set version 3 names
rather than declares: `validateEntryV3` names `scopesV2` and `validateEdgeV3`
names `edgeTypesV2`, so `implements` and both scopes go on meaning what they
meant.

**It is also the first time a narrowing had records on disk to condemn**, which is
the case the version stamp was built for and had not yet been asked to handle. Six
entries in this repo's log carry kind `unit` — two reading as version 1, four
declaring version 2 — and `validateEntryV1` and `validateEntryV2` both name
`entryKindsV1`. So `entryKindsV1` keeps the kind, `entryKindsV3` is its own map
literal that does not, and the read sweep judges each of those six by the version
it declares. There was no migration and no line was rewritten. A record stamped 1
or 2 is still judged by a set that contains `unit`; `add --kind unit` is refused,
because the write gate is the current version's rules and never dispatches.

**Version 2's bump bought dropping the plan scope**, and its kinds and every
field are version 1's — that is why `validateEntryV2` names `entryKindsV1` rather
than declaring a set of its own. There was no migration there either: the records
already on disk declare version 1, most of them carrying `"scope": "plan"`, and
`validateEntryV1` goes on judging them by the rules they were written against,
where that value is valid. Not a byte of them was rewritten, and no legacy value
entered the current validator.

**The kinds widened afterwards, and stayed inside version 3.** `entryKindsV3`
gained `hypothesis`, then `gotcha`, then `preference`, then `idea`, appended to the map
literal, with `CurrentSchemaVersion` left at 3 — the same move version 2 made
with the state edges, for the same reason: widening makes no record already
written illegal, so a bump would protect nothing. `entryKindsV1` gained none of
them, so an entry declaring version 1 or 2 with one of those kinds is refused by
the rules it declares, which is what says the widening happened inside one
version rather than everywhere at once. What it costs is the same false alarm the
rule above names: a binary built before this reports `kind: unknown` where it
would have reported an honest `unjudged`. Four widenings do not add up to a
bump: the direction is what decides, not the count.

`preference` needed nothing else from the schema. It is not added to
`anchoredKindsV3` — that set narrows, and adding to it now is a bump — and
`--plan` becoming optional on `add` took no rule with it, because no version's
rule set ever looked at `origin.plan`. The requirement had been the CLI's own.

**`origin.repo_id` arrived the same way and cost nothing.** No version's rule set
has ever inspected `Origin` at all, so a record lacking the key cannot fail and a
record carrying it cannot either — the optional-field rule above, in its purest
form. There was no migration and nothing was backfilled; see
[which repository a record came from](#which-repository-a-record-came-from).

**The `anchors` field arrived inside version 3, and it brought the one rule here
that takes something back.** The field itself is the easy half: optional, so every
record written before it stays legal, which is what the compatibility rule calls
free. The rule is not — **a `gotcha` with no anchor is refused**, and that makes a
record shape that was legal yesterday illegal today, which is what the rule above
says needs a bump.

It stayed inside version 3 on one ground, and the ground is a fact rather than an
argument: **it condemned no record on disk.** When the rule landed there were zero
`gotcha` records in either log — the kind had been widened into this same version
and never written — so a version 4 would have protected nothing and would have
cost every record a stamp nobody could read. Checked at the time rather than
assumed, and stated in the past tense on purpose: gotchas are being written now,
and every one of them was written under the rule. A later reader should not have
to re-run that count to trust this paragraph, because the count that matters is
the one taken the day the rule arrived.

The rule lives in `validateEntryV3` and in `anchoredKindsV3`, so a record
declaring version 1 or 2 is never asked for an anchor whatever this version does
next. **Adding a second kind to that set now is a bump**, and that is the line:
what made this one affordable is the empty set that was behind it, not the size
of the change. `hypothesis` and `review-finding` are the obvious candidates, and
both have records on disk — so neither can be added the way `gotcha` was.

**The edge types widened afterwards, and stayed inside version 2.**
`edgeTypesV2` holds version 1's six plus the five [state
types](#edge-types). Widening makes no record on disk illegal, so there was
nothing for a version 3 to protect, and the cost — a pre-widening binary
reporting a false `type: unknown` — was taken with open eyes; see the
compatibility rule above. `edgeTypesV1` is a separate map literal spelling out
its own six members, deliberately not `edgeTypesV2` under a second name: a Go
map is a reference, so an alias would be one map wearing two names, either of
which looks safe to narrow. An edge declaring version 1 with a state type is
still refused, by version 1's rules, exactly as it should be.

Five things a reader will expect to be wrong, and are not:

- **The Go constant `ScopePlan` stays.** `scopesV1` names it and version 1's
  rules are frozen, so it is version 1 vocabulary rather than dead code. What
  went away is `plan` as a place `Resolve` can find and as a value the CLI
  accepts.
- **The `unit` kind, by contrast, keeps no Go constant at all.**
  `entryKindsV1` holds `Kind("unit")` as a bare literal instead. The value has to
  be in version 1's frozen set or six records stop validating, so it is spelled
  there, once. But a constant in the `const` block is how this package says "a kind
  you may write", and nothing may write this one — so the kind exists in exactly
  one place with no constant naming it, and that asymmetry is the signal rather
  than an oversight. The difference from `ScopePlan` is that `scopesV1` is not the
  only thing naming `ScopePlan`: `Resolve` matches on it to refuse `plan` *by
  name*, which needs the constant.
- **`Origin.Unit` stays, as a field nothing populates.** Removing it would change
  no byte on disk — no record sets it and the key is `omitempty` — which is the
  whole argument for deleting it and the reason not to. Additive only means never
  remove a field, and a rule that holds except where removal happens to be
  invisible is a rule nobody can lean on.
- **`validateEdgeV2` states its own rules even where they match
  `validateEdgeV1`.** It cannot delegate, and this is the one place obligation 2
  above has teeth: the first rule any version states is *which version the record
  declares*, and v1's says 1. An edge stamped 2 judged by v1's rules would be
  refused for the stamp the tool itself put on it — and since the write gate is
  the current version's rules, `link` would refuse every edge it tried to write.
  A version is a whole rule set, and the version rule is one of them. Writing it
  out is also what let the edge types widen without version 1's set moving: one
  validator changed the set it names, the other did not.
- **`validateEdgeV3` exists although version 3 changed nothing about an edge.**
  Same rule, second time it was billed: every edge written now is stamped 3, so a
  `ruleSetV3` reusing `validateEdgeV2` would send every one of them into rules
  that require 2 and `link` would refuse them all. The three edge validators
  differ only in the stamp they require and the set they name, and whether that
  duplication stays affordable is open question `01KZ3472NSEBRE0C9XE6AP6TCC`.

## Usage

```bash
memory init                      # creates <repo>/.ai-kit/ and this repo's log in the store

memory add   --plan jwt-migration --kind decision \
             --title "RS256 over HS256" \
             --body "Per-service key rotation requires asymmetric signing." \
             --about auth,pkg/auth/

memory link  --type supersedes --from <new-id> --to <old-id>
memory link  --type met --from <objective-id> --to <decision-id>
memory link  --type derives_from --from <child-objective> --to <parent-objective>

memory add   --plan jwt-migration --kind gotcha \
             --title "The lock is taken before the write, not after" \
             --body "A deadlock in March; the order is not derivable from this file." \
             --about auth,pkg/auth/ \
             --anchor pkg/auth/token.go:41-43

memory add   --kind preference \
             --title "Migrations go through Atlas, not hand-written scripts" \
             --body "Hand-written scripts drifted from the schema twice in 2025." \
             --about persistence,migrations     # no --plan: the repo owns this

memory add   --plan jwt-migration --batch close.jsonl   # every line in one write
cat close.jsonl | memory add --plan jwt-migration --batch -

memory query --plan jwt-migration --about auth --kind decision --current
memory query --id <id>,<id>      # exactly those records, and what it could not find
memory query --since 2026-08-01T00:00:00Z --until 2026-08-31T23:59:59Z   # both bounds inclusive
memory query --count --kind gotcha    # how many, and no records
memory query --fields id,title        # those keys per record, and no others
memory query --body-lines 5           # the first five lines of every body
memory query --scope both --id <id>   # from whichever log holds it, and which one that was
memory search --plan jwt-migration "key rotation"
memory areas --kind preference   # what can be asked about, in both logs
memory anchors --kind gotcha --about pkg/auth/   # do those citations still hold
memory anchors --kind gotcha --verdict changed   # only the ones that drifted
memory anchors --path pkg/auth/token.go          # only the ones citing that file
memory anchors --id <id>,<id>    # only those records, and what it could not check
memory anchors --scope user      # each promoted record against the checkout it came from
memory anchors --all --kind gotcha --current   # every log in the store, each against its own repo
memory edges --type supersedes   # every edge of one relation, no starting id
memory edges --from <id> --since 2026-09-01   # what that record linked this month

memory state                     # every objective: state, tree, blocking, tally
memory state --id <objective-id> # just that one
memory state --text              # what is waiting on you, every log, as text
memory state --text --here       # the same, for this repository only
memory state --all               # every log, as JSON
memory state --all --id <objective-id>   # that objective from any repository, with its root

memory promote --kind preference --to user --dry-run   # see Promotion
memory promote --ids <id>,<id> --to user
memory validate --all
```

Every command prints JSON on stdout on success and on failure alike — except
[`state --text`](#what-is-waiting-on-you-across-every-log), which prints the
summary. A failure is JSON in every mode, `--text` included. Exit 0 means the
command ran; exit 1 means it refused, and the JSON says why.

`validate --all` validates every log the store holds, each on its own, but the
known-id set is the whole store's — so an edge pointing at a record that exists
only under another `source_path` is not dangling. An id no log holds still is, and
so does the single-log `validate`, which asks the same question of the same set.

`validate` is the one command where exit 1 is a verdict rather than a refusal. It
exits 1 when a log fails validation (`ok: false`) and when a location it was asked
to sweep could not be created (`init_failed`), and those two stay separate keys: a
directory nobody can write to says nothing about whether the records in it are
well-formed, but a sweep that covered fewer logs than requested has not passed
either, and a caller reading only the exit code has to hear about it. Only the
one-log sweep can report `init_failed`: `--all` takes its set out of the store and
creates nothing, so there is nothing there for it to fall short of.

`--scope` names the one log a command writes to or filters by. It is `repo` by
default and `user` is the only other value a record can live at — a value stamped
on the row, not a second file somebody has to have created: both logs are rows of
the one store, and nothing writes a `memory.jsonl` at either path. `query` alone
also takes [`both`](#asking-both-logs-at-once), which reads the two as one
answer, and every other command refuses it —
[`areas`](#which-areas-a-log-answers-for) is the one command that does not take
`--scope` at all, because it answers for both.

**A command whose answer spans the store refuses a `--scope` that was named.**
`edges` under `--from`/`--to`, `anchors --all`, `state --all`, `state --text`
without `--here`, and `validate --all` each read every log the store holds, so the
flag chose nothing there and was only deciding which single log the envelope would
claim the answer came from. Naming it is a refusal at exit 1 now —
`scope_not_on_store_wide_read`, the shape
[`edges`](#asking-about-edges-and-a-list-that-cannot-dump) already gives `--plan`
— and those answers name every log they read under `logs` in place of the `path`
and `scope` they used to claim. The refusal keys on whether the caller passed the
flag and never on its value, so an invocation that did not name `--scope` answers
exactly what it answered before — the default `repo` included — and `edges` with
no endpoint keeps it live, because "every edge here" is a question about a log.

`--plan` is a *stamp* on `add`, optional there since a record without one belongs
to the [repository](#records-the-repository-owns), and a *bound* on `query` and
`search`; other commands accept it and read the whole log, since `link` is the
operation the collapse exists to let cross plans.
[`edges`](#asking-about-edges-and-a-list-that-cannot-dump) is the one command
that refuses it outright, because an edge carries no plan at all. `state` is in
that second group on purpose — an objective's parent, its children and which
objective is current are properties of the log, so narrowing the derivation would
change the answers rather than the view. With no `--plan`, `AI_KIT_PLAN` is used.
`promote` is in that second group too: it reads the whole source log, so a class
sweep is not silently narrowed by a plan slug somebody passed out of habit.

`--store` is on every subcommand and defaults to `/srv/projects/.ai-kit/memory.db`.
It names the store, not the log: `--dir` and `--scope` still choose which log
inside it is read or written. Like every other flag it goes *after* the
subcommand, and before any positional text — `memory search --store PATH "key
rotation"`, never after the text, which the flag package would swallow as more of
it.

`--about` matches by exact tag or by path prefix: a tag ending in `/` catches
everything under it, so `--about pkg/auth/` finds a record tagged
`pkg/auth/middleware.go`.

### Asking by id, by time, and for a count

`query` takes three bounds beyond the ones that narrow by content, and each of
them composes with every other bound the command has.

`--id` takes a comma-separated list and returns exactly those records, sorted the
way every read here sorts: most recent first. Every id no record in the log
carries comes back under `not_found`, and the command still exits 0 — an id that
matches nothing is an answer, not a refusal. An id excluded by another bound is
reported the same way, so `--id <a>,<b> --current` names the settled one rather
than returning quietly short — `not_found` is measured against what the
invocation matched, not against the whole log.

`--id` carrying no id at all is the one refusal on this bound: `--id ""`, and
`--id "$ID"` against a variable nobody set, parse to nothing, and reading the
whole log there would answer a question nobody asked. The command exits 1 with
`empty_id` instead. Passing no `--id` is unchanged — no id bound, no `not_found`
key, exit 0.

`--since` and `--until` bound a query by the record's `at` stamp, and **both
bounds are inclusive**: a record stamped exactly at either one is returned. The
comparison is against the string as written, which is what makes it free — `at`
is RFC3339 and RFC3339 sorts lexically, exactly as a ULID does. What that costs
is that a bound is matched as a prefix rather than parsed as a date:
`--since 2026-08-01` behaves as the start of that day, while
`--until 2026-08-01` excludes everything stamped later on it. Pass a full
timestamp for a day boundary.

`--count` reports how many records match and returns none of them. It is the same
number the same query without `--count` reports, so the count-before-dump
threshold is neither in its way nor holed by it: `--count` answers with a number
whatever the guard would have done, and never with records.

### Asking for part of a record

Two more flags on `query` change the shape of what comes back rather than which
records do. A caller who wanted forty titles stops receiving forty bodies with
them, and a caller who wants to see how a 13 KB `gotcha` opens stops receiving
13 KB.

```bash
memory query --kind gotcha --fields id,title                 # the cheap listing
memory query --id <id> --fields title,body --body-lines 10   # the top of one long body
memory query --about auth --body-lines 0                     # every record, no bodies
```

`--fields` takes a comma-separated list of the record's own JSON keys — `version`,
`id`, `kind`, `slug`, `scope`, `at`, `about`, `title`, `body`, `anchors`, `origin`
— and returns exactly those and no others. Any subset is legal and nothing is
forced in: `--fields title` returns titles alone, without an id. A name outside
those eleven refuses the invocation with `unknown_field`, names the field it was
handed, and returns no records at all — a key silently dropped would read as a
record that does not carry it. A field you name comes back even where the record
has nothing under it, as `null` or `""`, so every row of the answer has the same
shape; a whole record still omits an empty `anchors` or `origin` exactly as it
always did. What the smaller shape costs is order: a projected record prints its
keys alphabetically rather than in the record's own order.

`--body-lines N` returns the first N lines of every body. `0` returns no `body`
key at all — including where `--fields` named it, which is the one place a named
field does not come back — and the default, `-1`, returns the whole of it, so a
query that does not name the flag answers exactly as it did before the flag
existed. A body of N
lines or fewer comes back whole. Named beside a `--fields` list that does not ask
for `body`, it does nothing rather than refusing.

Neither flag is on `search`, and neither is a way past the count-before-dump
guard: a query whose matches exceed the threshold still withholds them, and asking
for less of each record does not make it hand them over. `--limit` and
`--threshold` are still what get past it, and both compose with both of these.

A `query` that matches nothing answers with an empty `records` list and never with
`null`, including a `--id` that names an id the log does not carry — which is
still listed under `not_found` at exit 0.

### Asking both logs at once

`query --scope both` reads this repository's log and the user log in one
invocation, for the caller holding an id who does not know which of the two it
lives in, and for a kind or a text meant to cover both.

```bash
memory query --scope both --id <id>,<id>
memory query --scope both --kind preference --about persistence --current
```

The answer is one `records` list, most recent first, and every record carries the
`scope` it was written under — `repo` or `user` for anything written or promoted
since version 2. `--fields` still forces nothing in, so a projection that leaves
out `scope` leaves out which log a record came from. The envelope says
`scope: "both"` and names the two logs under `paths`, the repository's first, in
place of `path`.

- **Each log is filtered on its own and the matches are merged.** Their lines are
  never read as one log: the objectives a tree holds are that log's own, and
  joining two would let either one move which objective the other calls current.
  So `current_objective` is the one the repository log resolves, exactly as
  `--scope repo` names it. What the resolution reads off those objectives is the
  store's edges rather than one log's rows — an approval or a state edge written
  in another repository counts, and both passes read the same ones, which is why
  neither log moves the other's answer. What `--current` drops as superseded or
  answered is read off those same store-wide edges, so a record either log holds
  is settled by the edge that settles it wherever it was written. Which records
  come back is still each log's own.
- **Everything about the size of the answer is computed over the union.** `count`
  is the matches across both logs, `--count` returns that number, `--limit N`
  takes the N most recent of them, and the count-before-dump guard trips on the
  union even where neither log alone would trip it. `not_found` names only the ids
  neither log holds.
- **Nothing is deduplicated.** [Promotion](#promotion) copies a record with its id
  untouched, so a promoted record comes back twice, once per scope, and counts
  twice. Hiding one copy would hide that it lives in both.
- **A user log that does not exist adds nothing**, at exit 0, the way `areas`
  reads it. Outside a git repository `both` refuses with `bad_scope`, the refusal
  `--scope repo` gives there, before the user location is touched.
- **`both` is not a scope.** No record can carry it, `memory kinds` does not list
  it, and adding it to the [record scopes](#changing-the-schema) would be a schema
  version. It is a value of one flag on one command: `search`, `edges`, `anchors`,
  `state` and every write refuse it exactly as they did before it existed.

### Asking about edges, and a list that cannot dump

`edges` answers what got linked, and it is the one read that needs no starting id:
"every `supersedes` edge" and "what got linked last Tuesday" are questions about
the edges themselves rather than about a record somebody has already found.

```bash
memory edges                                 # every edge in the log
memory edges --type supersedes               # one relation, across the whole log
memory edges --type met,cancelled            # or several
memory edges --from <id>                     # what that record points at, in every log
memory edges --to <id>                       # what points at it, in every log
memory edges --since 2026-09-01 --until 2026-09-08    # what got linked that week
memory edges --count                         # how many, and no edges
```

**A read addressed by an id reads the store; a read addressed by a filter reads
this log.** `edges --from`/`--to` name a record, and a record is where it is: an
edge drawn in another repository is still an edge that names it, so both answer
the same from either end of it. `edges` with neither — bare, or bounded by
`--type` and the time bounds alone — is this log's edges, as it has always been,
because "every edge here" is a question about a log and not about a record.

So `--scope` is refused beside `--from` or `--to`: the answer is every log's, and
the flag was only deciding which single log the envelope claimed it came from —
`edges --scope user --from <id>` used to return an edge living in a repository's
log while reporting the user log as its source. `edges --from`/`--to` name every
log they read under `logs`, in place of the `path` and `scope` they used to emit,
the way [`anchors --all`](#anchors) and
[`state --all`](#what-is-waiting-on-you-across-every-log) already do. `edges`
naming no endpoint keeps `--scope`, because there the question really is about one
log.

It filters on what an edge carries and on nothing else: its type, its two
endpoints, its `at` stamp. That is the whole of an edge — no body, no `about` and
**no plan** — so `--plan` is refused here (`plan_not_on_edges`) rather than
accepted and ignored the way `state` accepts it. Bounding edges by a plan would
mean joining against the records at their ends, and so would filtering by those
records' kind or areas; none of that is in this command. Only `--plan` on the
command line refuses: `AI_KIT_PLAN` in the environment is not this caller naming a
flag, and it leaves `edges` answering.

`--type` takes the [edge types](#edge-types), comma-separated, and refuses a value
outside that closed set (`unknown_edge_type`) naming the eleven it does take.
`--since` and `--until` are inclusive and compared as written, exactly as on
`query`, so a bare `--until 2026-09-08` stops just before that day. A filter
matching nothing answers `edges: []` and never `null`, at exit 0. Edges come back
most recent first by their stamp, and edges sharing a stamp stay in the order the
log holds them.

**`edges` carries the count-before-dump guard**, the same one `query` carries and
for the same reason: a filter matching most of a log is an answer nobody asked to
be that big. Above the threshold it returns nothing — `edges: []` — with `count`
set to the true number of matches, `truncated: true` and a hint naming the flags
this command accepts and no others: the filters, `--limit N` and `--count`.

### Writing many records at once

`add --batch` takes one JSON object per line — a path, or `-` for stdin — and
appends every entry in it in a single write, or appends nothing at all. One line the
tool refuses refuses the whole batch, names the line and the field, and leaves the
log byte-identical.

A line carries the fields the flags carry and never the ones the tool stamps:

```jsonl
{"kind":"decision","title":"RS256 over HS256","body":"Per-service key rotation.","about":["auth","pkg/auth/"]}
{"kind":"review-finding","title":"Six criteria met, one unverified","about":["auth"],"edges":[{"type":"met","from":"<objective-id>"}]}
{"kind":"gotcha","title":"The lock is taken before the write","about":["auth"],"anchor":["pkg/auth/token.go:41-43"]}
```

- `kind`, `title`, `body`, `slug`, `about`, `anchor`, `plan`, `edges`. Anything else
  is refused by name, which is where the flag parser's typo protection went — and it
  is what refuses `id`, `at`, `version`, `scope` and `origin`, the fields whose whole
  point is that no caller chooses them.
- `about` takes an array or one comma-separated string; `anchor` takes an array or
  one string, and never splits on a comma, because a path may contain one.
- `--plan` stamps the batch and a line's own `plan` wins for that line. A batch that
  names no plan writes records the [repository owns](#records-the-repository-owns).
- **An edge on a line names exactly one endpoint, by an id already in the store.**
  The entry on that line is the other end: `"to"` puts it at the edge's source,
  `"from"` puts it at the target. There is no way to draw an edge between two
  entries of the same batch, because neither of them has an id until the write
  happens. The endpoint may live in another repository's log, exactly as
  [`link`](#the-objective-tree) allows, and the edge still lands in the log the
  batch is written into.
- Each edge passes the same three store-wide checks `link` applies — dangling
  endpoints, [state-edge justification](#edge-types) and the
  [objective tree](#the-objective-tree) — and any of them refuses the whole batch.
  A dangling endpoint is refused with `link`'s message, naming the store rather
  than one log's path.

### An argument in flag position may not contain a space

Every subcommand refuses, before it parses a single flag, an argument in flag
position that begins with `-` and contains a space. It is one argument where the
command line meant two, and an unquoted variable that failed to split is what
produces it: a shell hands over `--kind decision` as one word, Go's `flag` reports an
undefined flag named `kind decision`, and the tool looks like it invented a flag.

A flag's *value* is never inspected, so `--body '--about persistence'` is written
intact.

`search`'s text is not a flag's value — it is the first argument, and therefore in
flag position — so `memory search '--about auth'` is refused like any other unsplit
argument. That is narrower than it sounds: a text beginning with `-` was never
usable, because `flag` reads it as a flag and refuses it as undefined. A text that
begins with anything else may hold whatever it likes, spaces included:
`memory search 'key rotation'`.

The cost that is a real trade-off, and it is deliberate: the inline
`--text='key rotation'` form is refused too, because the guard reads the whole
argument rather than only the part before the `=`. Pass the value as its own
argument.

## Retiring a record written by mistake

A record that should not have been written is **retired, never removed**: nothing in
this log is ever deleted or edited. What retires it is a new record saying what it
got wrong, plus a `supersedes` edge:

```bash
memory add  --kind decision --title "The probe record was a smoke test, not knowledge" \
            --body "What it got wrong, and what is true instead." --about <the mistake's own tags>
memory link --type supersedes --from <the new record> --to <the mistaken record>
```

or, as one batch line, which lands both halves in one write:

```jsonl
{"kind":"decision","title":"<what it got wrong>","body":"<...>","about":["<tags>"],"edges":[{"type":"supersedes","to":"<the mistaken record>"}]}
```

Afterwards a read with `--current` no longer returns the mistake and a read without
it still does — the wrong record stays on disk, and so does the record saying it was
wrong. That is the whole mechanism: there is no `retract`, no tombstone kind and no
read that skips a record by default, because a second invisible filter would
contradict the one claim this log makes, which is that a read returns what was
appended.

**The direction is `--from <new> --to <mistaken>`, and drawn backwards it is not
refused.** The reverse edge is accepted at exit 0 and retires the *replacement*,
leaving the mistake as the live answer, with nothing in the output saying so: both
directions are legal edges and only one of them is what you meant. Read the
direction before writing it, and confirm with `--current` afterwards — the wrong
record still coming back is what a backwards edge looks like.

Tag the new record with the areas the mistaken one carries. A retirement filed
under different tags is invisible to whoever finds the mistake.

## Guarantees under test

- A record with an unknown `kind`, an empty `about`, a non-ULID id or a
  non-kebab slug is refused before it reaches disk.
- An edge whose endpoints do not both exist is refused at write time; `validate`
  also sweeps for dangling edges, because they fail silently — a graph walk
  just returns less than it should.
- A state edge is refused **by `link`** unless it points at a record whose kind
  could justify it, and the refusal names both the allowed kinds and the kind
  that was actually there. The same edge sitting on disk sweeps clean, because
  the rule is on the way in and nowhere else. `link` is the whole of it, and
  that is narrower than "at write time": `promote` copies edges into the target
  log without passing any gate, so a state edge whose justification was left
  behind can arrive there unchecked. That is open question
  `01KZ38XQTXFQZRVG6VMPRA82SK`, and the copy is at
  `internal/memory/promote.go:218`. [Promotion's one-hop reach](#promotion) makes
  it less likely — the justification is exactly the record one edge away — and
  does not close it: two hops out, it stays behind and is reported as border.
- An edge declaring version 1 with a type version 2 added is refused by the
  sweep; the identical edge declaring version 2 passes, and version 1's own set
  is still the six it shipped with.
- Appending never changes bytes already written, including when a record is
  superseded — and including when a record predates the `version` stamp: it is
  read as version 1, not rewritten to say so.
- Two records with identical fields are judged differently when they declare
  different versions; a version the binary does not know is reported as
  unjudged and does not turn `ok` false.
- A key the reader does not know is ignored rather than treated as corruption,
  and a mistyped flag on `add` is still refused before anything is written.
- One malformed line is reported with its line number and does not make the
  lines around it unreadable.
- `query` returns the true count even when it returns no records.
- `--plan` bounds a query to one plan's records out of a log holding several,
  counts only that plan's matches, and still withholds them above the threshold.
- Two records written under different plans can be linked, and `edges --from` on
  one of them returns the edge that reaches the other.
- `edges --plan <slug>` is refused at a non-zero exit, the refusal says an edge
  carries no plan, and it hands back no `edges` key at all.
- `edges` answers a filter that matched nothing with an empty list and never with
  `null` — over a log holding no edges, over a type nothing carries, and over an
  endpoint no edge names — asserted by decoding the output rather than matching
  its text.
- `add` writes a record when no plan is named, and it carries no `plan` key at
  all rather than an empty one; a plan slug that is not kebab-case is still
  refused, and so is a kind outside the closed set, a missing `about` and a blank
  title. `AI_KIT_PLAN` is still the second way to name a plan.
- A plan-less record comes back from `query --kind preference` with no `--plan`
  and from `--plan <slug>` for any slug, including one no record in the log
  carries — while a record that names a plan is still absent from a query naming
  a different one. The widening admits the plan-less case and nothing else.
- Two preferences sharing a first `--about` tag both come back `--current`, most
  recent first, with no edge drawn; a `supersedes` edge from the newer then drops
  the older. Both endpoints carry no plan and the edge is written either way,
  since `link` reads no plan at all.
- `plan` is refused as a scope going in, by no flag and no environment variable
  can it be reopened, and the records already on disk that carry it still pass
  the sweep.
- `unit` is refused as a kind going in, the refusal names the field and the set
  that no longer holds it, `memory kinds` does not advertise it, and a refused
  `add` leaves the log the length it was.
- One entry with kind `unit` is accepted by version 1's and version 2's
  validators and refused by version 3's, on the `kind` field — the narrowing did
  not leak backwards through a map two names shared. Through the sweep, the same
  three records produce exactly one problem, on the line that declares 3, and
  nothing goes unjudged.
- `hypothesis` is accepted by version 3's validator and by the write gate,
  advertised by `memory kinds`, refused by version 1's and version 2's rules on
  the `kind` field, and `CurrentSchemaVersion` is still 3 — a widening does not
  move the version, and it leaves every record already on disk valid.
- `gotcha` clears the same six checks, and the version is still 3 with both
  widenings in: two of them do not add up to a bump.
- `preference` clears them a third time, is absent from `anchored_kinds`, and is
  written with no `--anchor` where an unanchored `gotcha` is still refused. Three
  widenings still do not add up to a bump.
- `idea` is advertised by `memory kinds` and counted in version 3's widenings, so
  four of them still leave the version at 3; an answered idea is gone from the
  Ideas section while an unanswered one in the user log is listed with its log's
  name and id.
- Anchors survive a round trip through the log field for field, on a record
  carrying two of them, and the log still sweeps clean afterwards.
- `add --kind gotcha` with no anchor is refused on the `anchors` field, the
  refusal says how to satisfy it, and nothing reaches the log; the same record
  with an anchor is written. Every other kind is still written without one, and
  every other kind may still carry one.
- An anchor whose file does not exist, whose range runs past the end of it, whose
  lines run backwards, or whose path climbs out of the repo is refused by `add`,
  not recorded and reported later.
- A record written before anchors existed, declaring version 1 or 2, is never
  asked for one — the rule is in version 3's rule set and nowhere else.
- Cited text that moved reports `still_holds` with the line it is at now and
  `moved`, while the stored line and the record's bytes on disk are untouched.
- Cited text that was rewritten reports `changed`; a file that cannot be read
  reports `could_not_check` and the note names the file. Cited text appearing
  twice reports `still_holds` with both lines and a match count, never a choice.
- A record's verdict is its worst anchor's, and a record with no anchors is
  skipped rather than counted as holding.
- `anchors --verdict` filters on that roll-up and not on the individual anchor, so
  a record with one `changed` anchor and one that holds comes back under
  `--verdict changed` and not under `--verdict still_holds`; `--path` returns a
  record whose two anchors both match it once rather than twice; and `--id` names
  an id the log does not carry under `not_found` rather than answering empty.
- `anchors` checks every match rather than withholding above the threshold, over
  a log well past it, in under a second; `--repo-root` moves which working tree
  is read without moving the log.
- A `gotcha` written in one repository and promoted to the user log, checked with
  `anchors --scope user` from a second repository, is `still_holds` while the
  first repository's cited lines are untouched and `changed` once they are
  rewritten there — while the second repository holds the opposite version of the
  file at the same path each time. Named as `--repo-root`, the second repository is
  the one read.
- A user-log record carrying no `origin.repo_id`, one whose identity matches no
  repository log and names no existing directory, and one whose identity two
  checkouts with `.git` directories share are each `could_not_check`, with a note
  naming its own case and no `repo_root` — run from a repository whose file at the
  cited path would have made all three `still_holds`.
- A record promoted out of a checkout whose identity a `git worktree` of it shares
  is checked against the checkout, whose `.git` is a directory, and reports
  `still_holds` read from inside a worktree whose cited lines were rewritten.
- Whitespace, case and indentation all move an anchor's hash — the check
  normalizes nothing, on purpose — and checking writes nothing to the file it
  read.
- Promotion preserves ids, does not mutate the source, is idempotent, never
  copies an edge whose other end stayed behind, and never restamps the version a
  record was written under. Idempotency is proven for edges as well as records —
  with real edge slices on both sides — and proven again through the built binary
  run twice over the same two files, where the destination's bytes are identical
  across the second run and the report classifies everything as already present.
- `--kind preference` promotes every preference and no record of any other kind,
  with no id named. `--kind decision` and `--kind notakind` are both refused by
  name, exit 1, list the four kinds that are accepted, and write nothing.
- `--kind` and `--ids` together put the union of both selections in the
  destination; neither given is refused with a message naming both.
- An `adr` promoted by id alone carries the `decision` it derives from and the
  edge between them, and stops there: a record two hops out is left behind,
  reported in `border` with its count and counted in `edges_skipped`. Both logs
  still validate afterwards, with no dangling edge.
- An `adr` linked to both a `decision` and an `objective` carries the decision
  and leaves the objective, which is reported in `border` and whose edge is
  counted — and naming that objective in `--ids` afterwards promotes it and
  reconnects the edge. The same holds for a `gotcha` in the objective's place.
  The reach is bounded; the selection is not.
- `--dry-run` reports exactly what the run without it goes on to report, leaves
  the destination's bytes untouched, and does not even create the destination
  when there is none.
- Two repositories whose directories are both called `api` carry the same
  `origin.repo` and different `origin.repo_id` in one user log: the one with a
  git remote takes its identity from the remote, the one without takes its
  absolute path.
- A state resolves to the most recent state edge by `at`; a later line carrying
  an older `at` loses, and two edges sharing an `at` are separated by which is
  later in the file. `approved_for_execution` → `blocked` →
  `approved_for_execution` → `met` resolves to `met`, and the stale edges under
  it are still on disk.
- `link` refuses a second parent for an objective and refuses an edge that would
  close a cycle, naming the parent it already has and the whole cycle path
  respectively; a self-edge is refused too, by the edge rules before the tree
  rule is reached. `derives_from` with anything other than an objective at both
  ends is untouched, including an ADR deriving from several decisions.
- A log holding a cycle and an objective with two parents is still read: `state`
  terminates within a deadline the test enforces, reports both anomalies, uses
  the first parent edge in the file, and `validate` still reports `ok`.
- `--current` over objectives returns every open one, a chain of three included,
  and drops one the moment it is `met` — marking the only leaf of that chain
  `met` drops the two parents it leaves with nothing outstanding as well; every
  other kind's `--current` drops what
  a `supersedes` or an `answers` edge settles and nothing else, and gets no
  `current_objective` field at all.
- A `decision` in one repository's log and an `open-question` in another are each
  dropped by `query --current`, `search --current` and `anchors --current` the
  moment a `supersedes` or an `answers` edge naming them is written into a third
  log; without `--current` both come back.
- `edges --from <child>` and `edges --to <parent>` each return the one edge a
  `derives_from` spanning two logs draws, read from either of them, while `edges`
  naming no endpoint still answers with the log it was asked from.
- A `--scope` the caller named is refused at exit 1 by `edges --from`, by
  `anchors --all`, by `state --all`, by `state --text` without `--here` and by
  `validate --all` — `scope_not_on_store_wide_read`, naming the flag and saying
  the answer spans every log in the store; `edges --scope user` naming no endpoint
  is not refused and returns that log's edges at exit 0. `edges --from` names both
  logs under `logs` and carries no top-level `path` or `scope`.
- `state` with no flags reports `objectives_from` and `derived_over`, and its
  `objectives` list is what it was before either key existed. `areas --current`
  drops a record a `supersedes` edge in another log settles, which it used to
  report as its area's live answer.
- `anchors --all` checks a record in each of two repositories against its own
  repository's working tree, reporting `changed` for the one whose cited lines
  were rewritten there and `still_holds` for the other, names both logs under
  `logs`, and puts no `repo_root` at the top; `--repo-root` alongside it reads
  both records against the one root named. `anchors` without the flag returns the
  records it has always returned.
- The current objective is the most recently approved open one, or the most
  recently touched when none is approved, and the reported rule changes with it.
  It is derived from the whole log and says `outside_filter` when `--plan` puts
  it out of view.
- Four records sharing one `at`, laid out as objective, objective, an edge
  touching both, objective, resolve the current objective to the *last line* —
  not to the source of the edge — and moving that edge to the end moves the
  answer with it.
- Two objectives an edge ties to the same line fall back to entry order, and
  swapping their two entries swaps the answer while the edge points the same way.
- A parent's tally counts `met` and superseded children as done, leaves
  `cancelled` and `separated` out, and reports the rest outstanding.
- A parent with one child `met`, another `cancelled` and no state edge of its own
  resolves to `met` with `state_from: children`, and its own parent counts it as
  done and reads `met` too; a parent whose children are all `cancelled` or
  `separated` stays `planned`. An `approved_for_execution` of the parent's own
  written after its child's `met` keeps it `approved_for_execution`, and one
  written before does not.
- An objective approved and then blocked on an open question that something
  `answers` resolves to `approved_for_execution`, the approval edge its winner;
  one blocked on such a question with no earlier edge resolves to `planned`; one
  blocked on a `decision` stays `blocked` even when an `answers` edge names it.
- `state --all --id`, run from one repository for an objective recorded in
  another, reports it with `root` set to the other repository's working tree; each
  current objective under `--all` names the root of its own log, and `state` over
  either log alone carries the same `root` on its objective and its current one.
- `areas` reports both scopes on every run, each carrying the path it resolved
  to. A user log that was never created is a user scope with zero areas and exit
  0, printing an empty list rather than null — never a section left out, which
  would read the same as not having looked.
- A tag sitting second in a record's `--about` is enumerated like any other and
  its row says `key: false`, while the first tag's row says `key: true` and both
  name the same record. A record whose tag is second still answers for that area
  when it is the newest one carrying it.
- Two `areas` runs over one log print byte-identical output, because the rows are
  sorted by area name and not by count — the order a caller diffs must not move
  every time something is written.
- A 25-line batch appends 25 entries and reports every id; the same file with line
  13 carrying an unknown kind is refused naming line 13 and the `kind` field, and
  the log's bytes are identical to before the run.
- A batch line's edge is written with its entry in both directions: `to` an existing
  parent objective writes a `derives_from` out of the new record, `from` an existing
  objective writes a `met` into it, and the tree then reports that objective `met`.
- A batch line whose edge names an endpoint held by another repository's log is
  appended at exit 0, and the edge lands in the log the batch was written into
  while the other log's bytes are untouched.
- A batch edge naming an id no log of the store holds is refused `dangling_edge`
  with the message `link` gives for the same edge, naming the store; two
  `derives_from` on one objective's line are refused `not_a_tree` on the second;
  a `met` whose justification cannot carry it is refused `unjustified_state`. Every
  refusal names the line and leaves the log byte-identical.
- A `not_a_tree` refusal names *the record on line N* where the entry's own id would
  otherwise appear, and every id left in the message is one the log actually holds.
  The id a refused batch stamped is never written, so quoting it would send a reader
  looking through the log for a record that does not exist and never will.
- An edge naming both endpoints or neither is refused on its own `edges[N]` field,
  before the log is consulted at all.
- A batch line carrying `id`, `at`, `version`, `scope` or `origin` is refused by
  name, and so is a mistyped key; what the tool stamps is stamped, and a batched
  entry reads back at the current version and the resolved scope.
- `--plan` stamps every line of a batch, a line's own `plan` wins for itself, and a
  line whose plan is not kebab-case refuses the batch on that line.
- `about` and `anchor` each take a string or an array and produce the same record;
  a comma splits a tag and never a path. A `gotcha` in a batch is still refused
  without an anchor, and an anchor past the end of the file is refused naming the
  anchor rather than asking for a second one.
- A wrong-shaped `about` or `anchor` is refused with that key as the problem's
  `field` and a message saying both what was wanted and what the line carried —
  `{"about": 5}` and `{"anchor": 7}` no longer refuse identically, and an array
  holding a non-string is told what is wrong with the array rather than that it is
  not one. Proven on a line whose body is long enough that `raw` truncates before
  the key, which is the case where the field is the only thing naming it.
- `--batch` together with any of `--kind`, `--title`, `--body`, `--slug`, `--about`
  or `--anchor` is refused naming the flag; an empty batch and an unreadable path
  are each their own refusal, and none of the three writes anything.
- A batch's git work does not grow with its line count: the same number of `git`
  spawns for a 10-line batch as for a 1-line one, counted through the built binary
  with a wrapper ahead of the real `git` on `PATH`. Building an `Origin` shells out
  twice and `firstNonEmpty` evaluates every argument, so a set `AI_KIT_AUTHOR` skips
  nothing — 200 lines cost 400 spawns before the origin was hoisted out of the
  per-line path.
- Every one of the thirteen subcommands refuses `'--kind decision'` in flag position
  at exit 1, quoting the argument and asking about the unquoted variable, before any
  flag is parsed — while `--body '--about persistence is a legitimate body'` is
  written intact, because a flag's value is never inspected.
- `search`'s text is the one argument that is *not* a flag's value and is still in
  flag position, so `search '--about auth'` is refused by the guard while
  `search 'key rotation'` is not. `search -about` is refused too, at exit 2 by the
  flag package as an undefined flag — a text beginning with `-` was never usable, and
  the two refusals stay distinguishable.
- `query --id <a>,<b>` returns exactly those two records and nothing else, most
  recent first. An id nothing carries is absent from the records, named under
  `not_found`, and neither turns the run into a refusal nor invents a record; a
  query that named no id reports no `not_found` at all.
- A record stamped exactly at `--since` and a record stamped exactly at `--until`
  are both returned, and a record one second outside either bound is not — all six
  positions asserted against one pair of bounds.
- `--count` reports the number of records the same filter hands back, across nine
  filter combinations including `--id`, `--since`/`--until` and every bound that
  existed before them.
- The count-before-dump guard still withholds records from a query bounded by
  `--id` or by time, and `--count` is not the way past it: it answers with a
  number and no records under the threshold, over it, with `--limit`, and with the
  guard disabled.
- `query --fields <list>` returns exactly the named keys and no others, asserted
  over the marshaled JSON for a single-field list that omits `id`, for a
  multi-field list, and for all eleven fields at once.
- `--fields` naming something that is not a record field is refused at a non-zero
  exit, names the field it was handed, and prints no `records` key at all.
- `--body-lines 2` returns the first two lines of a five-line body and the whole of
  a two-line one — this log's median body is two lines — while `0` leaves no `body`
  key on any record and naming no flag at all returns the whole body.
- `query` answers a zero match with an empty `records` list rather than `null`,
  both when the filter matched nothing and when `--id` named an id the log does not
  carry, and `not_found` still names that id.
- `query --scope both --id <a>,<b>,<c>`, with `<a>` only in the repository log,
  `<b>` only in the user log and `<c>` in neither, exits 0 with `<a>` carrying
  `scope: "repo"` and `<b>` carrying `scope: "user"` in one `records` list, and
  `not_found` naming `<c>` alone.
- `query --scope both` against a HOME holding no `.ai-kit/` at all exits 0 and
  returns the repository log's records, each carrying `scope: "repo"`.
- `search` and `edges` refuse `--scope both` at exit 1 with `bad_scope`, naming
  the scope they were handed.

```bash
go test ./...
```

## Build

```bash
go build -o bin/memory ./cmd/memory
```

One dependency beyond the standard library: `modernc.org/sqlite`, the store's
driver, and the nine indirect modules `go.mod` names behind it. It is a pure-Go
implementation, so the build above needs no cgo and no SQLite on the machine.
