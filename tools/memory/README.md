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
| Core | `objective` `unit` `decision` `deviation` | Make the work loop run |
| Learning | `assumption` `open-question` `wont-fix` `dead-end` | Stop verification theater and repeated work |
| Knowledge | `adr` `postmortem` `review-finding` | Outlive the plan, get promoted upward |

`wont-fix` earns its place: once you decide a risk is not worth covering, that
decision is recorded, and no later verification pass raises it again.

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

This rule is written down and nothing computes it yet. There is no `memory
state` command, and `query` and `graph` report edges rather than a resolved
state — deliberately, since a derivation belongs with the read path that needs
it. Whoever builds that reads this section first.

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

- **Never rename a field.** The JSON key *is* the identity here — this is
  protobuf's discipline without protobuf's numbering. A rename is a delete plus
  an add, and every record already written fails it.
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
   `CurrentSchemaVersion`, `entryKinds`, `edgeTypes` and `scopes` to it. The
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
| 1 | `scope` is one of `plan`, `repo`, `user`; six edge types | The original shape |
| 2 | `scope` is one of `repo`, `user`; eleven edge types | The plan scope was removed — that is what the bump was for. The edge-type set widened later, in place, by the five state types |

**Version 2 is the current version, and dropping the scope is what the bump
bought.** The kinds and every field are version 1's — that is why
`validateEntryV2` names `entryKindsV1` rather than declaring a set of its own.
There was no migration: the records already on disk declare version 1, most of
them carrying `"scope": "plan"`, and `validateEntryV1` goes on judging them by
the rules they were written against, where that value is valid. Not a byte of
them was rewritten, and no legacy value entered the current validator.

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

Two things a reader will expect to be wrong, and are not:

- **The Go constant `ScopePlan` stays.** `scopesV1` names it and version 1's
  rules are frozen, so it is version 1 vocabulary rather than dead code. What
  went away is `plan` as a place `Resolve` can find and as a value the CLI
  accepts.
- **`validateEdgeV2` states its own rules even where they match
  `validateEdgeV1`.** It cannot delegate, and this is the one place obligation 2
  above has teeth: the first rule any version states is *which version the record
  declares*, and v1's says 1. An edge stamped 2 judged by v1's rules would be
  refused for the stamp the tool itself put on it — and since the write gate is
  the current version's rules, `link` would refuse every edge it tried to write.
  A version is a whole rule set, and the version rule is one of them. Writing it
  out is also what let the edge types widen without version 1's set moving: one
  validator changed the set it names, the other did not.

## Usage

```bash
memory init                      # creates <repo>/.ai-kit/memory.jsonl

memory add   --plan jwt-migration --kind decision \
             --title "RS256 over HS256" \
             --body "Per-service key rotation requires asymmetric signing." \
             --about auth,pkg/auth/

memory link  --type supersedes --from <new-id> --to <old-id>
memory link  --type met --from <objective-id> --to <decision-id>

memory query --plan jwt-migration --about auth --kind decision --current
memory search --plan jwt-migration "key rotation"
memory graph --from <id> --depth 2

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
`graph` are the operations the collapse exists to let cross plans. With no
`--plan`, `AI_KIT_PLAN` is used.

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
- Promotion preserves ids, does not mutate the source, is idempotent, never
  copies an edge whose other end stayed behind, and never restamps the version a
  record was written under.

```bash
go test ./...
```

## Build

```bash
go build -o bin/memory ./cmd/memory
```

No dependencies beyond the standard library.
