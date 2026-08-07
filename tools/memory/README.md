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
no references anywhere. The human-facing number is rendered at read time, per
scope, by `memory render`.

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

Both are gitignored. Promotion between them is always an explicit act.

**One log per repo, and the plan lives in the record.** Every entry carries
`origin.plan`, and `--plan <slug>` is a filter over that field — not a choice of
file. `add` refuses to write a record that cannot name its plan, because
`origin.plan` is the only thing a bounded query has left to bound by.

There used to be a third scope, `plan → <repo>/plans/<slug>/memory.jsonl`, and
removing it is what [schema version 2](#changing-the-schema) is. One file per
plan was not a tidier arrangement of the same graph, it was a smaller graph: an
edge needs both of its endpoints in the file it is written to, so nothing in one
plan's log could ever point at anything in another's. Any two records in a repo
can now be linked, whatever plan produced them.

## Record kinds

| Group | Kinds | What they are for |
|---|---|---|
| Core | `objective` `decision` `deviation` | Make the work loop run |
| Learning | `assumption` `open-question` `wont-fix` `dead-end` `hypothesis` | Stop verification theater and repeated work |
| Knowledge | `adr` `postmortem` `review-finding` `gotcha` | Outlive the plan, get promoted upward |

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
asking the code twice is expensive. What should happen to one whose code has moved
is undecided, and so is whether the path:line it constrains — body text today —
ever becomes a field: both wait on what an anchor check turns out to be. It was
[widened into version 3](#the-versions) in place, after `hypothesis`.

Which surface writes one: `implement` does, inline, at the moment the constraint
is discovered. The rule that decides when a *why* is written as a `gotcha` at all
is stated in `agents/implement.md` and nowhere else.

There used to be a `unit` as well, for a piece of work under an objective, and
removing it is what [schema version 3](#changing-the-schema) is. An objective may
derive from an objective, so the work under one is another objective and the kind
named a distinction the tool no longer draws. The records already on disk that
carry it declare version 1 or version 2 and are still judged by the rules those
versions state, where the kind is valid.

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

`memory state` is what computes it. `query` and `graph` still report edges —
`graph` deliberately, since walking relations is a different question from
resolving one record's state.

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
*printed* as 1 as well — `query` and `graph` show the version the record is
judged at, because `"version": 0` is on no line and names no schema that ever
existed.

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
| 3 | twelve kinds; `scope` and the edge types are version 2's | `unit` was removed from the entry kinds. Nothing else moved. The kind set widened later, in place, by `hypothesis` and then `gotcha` |

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
gained `hypothesis` and then `gotcha`, appended to the map literal, with
`CurrentSchemaVersion` left at 3 — the same move version 2 made with the state
edges, for the same reason: widening makes no record already written illegal, so
a bump would protect nothing. `entryKindsV1` gained neither, so an entry
declaring version 1 or 2 with kind `hypothesis` or `gotcha` is refused by the
rules it declares, which is what says the widening happened inside one version
rather than everywhere at once. What it costs is the same false alarm the rule
above names: a binary built before this reports `kind: unknown` where it would
have reported an honest `unjudged`. Two widenings do not add up to a bump: the
direction is what decides, not the count.

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
memory init                      # creates <repo>/.ai-kit/memory.jsonl

memory add   --plan jwt-migration --kind decision \
             --title "RS256 over HS256" \
             --body "Per-service key rotation requires asymmetric signing." \
             --about auth,pkg/auth/

memory link  --type supersedes --from <new-id> --to <old-id>
memory link  --type met --from <objective-id> --to <decision-id>
memory link  --type derives_from --from <child-objective> --to <parent-objective>

memory query --plan jwt-migration --about auth --kind decision --current
memory search --plan jwt-migration "key rotation"
memory graph --from <id> --depth 2

memory state                     # every objective: state, tree, blocking, tally
memory state --id <objective-id> # just that one

memory promote --ids <id>,<id> --to user
memory render  --as adr
memory validate --all
```

Every command prints JSON on stdout on success and on failure alike — except
`render`, which prints markdown. Exit 0 means the command ran; exit 1 means it
refused, and the JSON says why.

`--scope` is `repo` by default, and `user` is the only other value. `--plan` is a
*stamp* on `add`, which refuses without one, and a *bound* on `query` and
`search`; other commands accept it and read the whole log, since `link` and
`graph` are the operations the collapse exists to let cross plans. `state` is in
that second group on purpose — an objective's parent, its children and which
objective is current are properties of the log, so narrowing the derivation would
change the answers rather than the view. With no `--plan`, `AI_KIT_PLAN` is used.

`--about` matches by exact tag or by path prefix: a tag ending in `/` catches
everything under it, so `--about pkg/auth/` finds a record tagged
`pkg/auth/middleware.go`.

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
  `internal/memory/promote.go:88`.
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
- Two records written under different plans can be linked, and each is reachable
  from the other by a graph walk.
- `add` refuses when no plan is named, because the record it would write is
  unreachable by every bounded query.
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
- Promotion preserves ids, does not mutate the source, is idempotent, never
  copies an edge whose other end stayed behind, and never restamps the version a
  record was written under.
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
  and drops one the moment it is `met`; every other kind's `--current` returns
  what it returned before, and gets no `current_objective` field at all.
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

```bash
go test ./...
```

## Build

```bash
go build -o bin/memory ./cmd/memory
```

No dependencies beyond the standard library.
