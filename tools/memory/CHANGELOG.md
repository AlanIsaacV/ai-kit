# Changelog

## Unreleased

### State is an edge, and an edge has to be justified

Five edge types join the six that were already here: `approved_for_execution`,
`met`, `cancelled`, `separated`, `blocked`. What state a record is in stops
being prose in a body and becomes a relation, which is the only shape an
append-only log can hold honestly — a field would have to be rewritten to
change, and nothing here is ever rewritten. The default state is the absence of
all five, so being in it costs nothing and leaving it costs one appended line.

- **The second endpoint is the evidence.** `link` refuses a state edge whose
  target could not justify it: `approved_for_execution`, `cancelled` and
  `separated` take a `decision`, `met` takes a `decision` or a `review-finding`,
  `blocked` takes an `open-question` or a `decision`. The refusal names the
  allowed kinds and the kind that was actually there, so it can be acted on
  without going back to the log to look the id up by hand.
- **The rule is on the write path and nowhere else.** A version's rule set
  judges one record in isolation and cannot see the target's kind; `link`
  already loads every entry to refuse a dangling edge, so the kind is free at
  the one moment it can still be acted on — the log is append-only, and there is
  no taking a state back afterwards. `validate` does not apply it, so a rule
  that arrived today never condemns an edge written yesterday.
- **No reflexive form.** An edge from a record to itself would be a state
  asserted with nothing behind it, indistinguishable on disk from one that was
  earned, and it would need `validate` to start permitting cycles of length one.
- **The six original types are untouched.** They say how two records relate
  rather than what happened to one of them, and `supersedes` in particular has
  edges on disk that a rule invented afterwards has no business calling invalid.
- **Done inside version 2, and there is no version 3.** Widening a closed set is
  additive: no record already written becomes illegal, and no frozen rule set
  has to change. The cost was taken knowingly and is written into the README's
  compatibility rule — a binary built before this has rules for version 2, so it
  meets a `met` edge stamped 2 and reports `type: unknown`, a false problem,
  where for a version it did not know at all it would have reported an honest
  `unjudged`.
- **`edgeTypesV2` is its own map literal**, naming all eleven types rather than
  aliasing `edgeTypesV1`. A Go map is a reference, and two names for one map are
  both narrowable, so an alias would let a later bump narrow version 1's frozen
  set by accident. `validateEdgeV2` names the new set; `edgeTypesV1`,
  `validateEdgeV1` and `CurrentSchemaVersion` did not move, and an edge
  declaring version 1 with a state type is still refused by version 1's rules.
- **`memory kinds` advertises eleven edge types**, and publishes the allowlist
  itself under a new `justifications` key — an object of state to allowed kinds,
  holding only the five types that carry a rule. A set that widens has to widen
  the advertised list with it, for the same reason a set that narrows has to
  narrow it: otherwise the tool hides a value it accepts, or enforces a rule a
  caller can only discover by tripping over it. The existing `entry_kinds`,
  `edge_types`, `scopes` and `threshold` keys are unchanged.
- **Which state edge wins is decided and written down**, in the README: the most
  recent state edge by `at`, ties broken by later line in the file. Moving
  between states means writing the state you are moving to, with no retraction
  and no `supersedes` — `approved_for_execution` → `blocked` →
  `approved_for_execution` → `met` is a correct trace of a state machine, not a
  contradiction, and a superseded `blocked` is stale rather than wrong. The
  known limit is stated with it: returning to bare `planned` is not expressible,
  because `planned` is the absence of state edges. **Nothing computes this
  yet** — there is no `memory state`, and neither `query` nor `graph` resolves a
  state. The rule is recorded now so the read path that will need it is not
  invented twice.

### One log per repo, and schema version 2

The plan scope is gone. There is one log, at `<repo>/.ai-kit/memory.jsonl`, and
the plan a record belongs to lives in the record as `origin.plan`.

This was not a tidying pass. One file per plan was a smaller graph, not a
neater one: an edge needs both of its endpoints in the file it is written to, so
nothing in one plan's log could point at anything in another's — and "this
objective derives from that one" was inexpressible for exactly the pair of
records that most needed to say it.

- `--plan <slug>` is a filter over `origin.plan` rather than a choice of file. A
  bounded query still bounds, now by objective instead of by directory, and the
  count-before-dump threshold applies to the named plan's own matches.
- `add` stamps `origin.plan` from `--plan` or `AI_KIT_PLAN` and refuses when it
  has neither. A record with no plan is unreachable by every bounded query, and
  the log is append-only, so refusing is the only place to catch it.
- Any two records in a repo can be linked, whatever plan produced them.
- `promote --to repo` is gone: a plan's records are already in the repo log, so
  there is nothing below it to lift them out of. `promote --to user` is the one
  crossing left.
- `validate --all` looks at the repo and user logs and reports their real
  counts. It used to resolve the plan scope first, find several plan directories,
  give up, swallow the failure and report `ok: true` over an empty set — a sweep
  that passed because it checked nothing.
- `init` defaults to the repo scope, and `--scope plan` is refused by name rather
  than as an unknown value, because it was legal until this version and the
  habits that pass it are real.

### Schema version 2

The first real version bump, and the case the versioning machinery was built
for: a value that was legal yesterday is illegal today, and no record written
yesterday became invalid.

- `scopesV2` is `{repo, user}`. `scopesV1` keeps all three and is untouched, so
  the records on disk that declare version 1 with `"scope": "plan"` are still
  judged by version 1's rules and still pass. Nothing was migrated, nothing was
  rewritten, and no legacy value entered the current validator.
- `validateEntryV2` states its whole rule set, naming `entryKindsV1`: version 2
  narrowed where a log lives, not what a record says, so a second copy of the
  kind set would be one more thing to keep in step. `unit` in particular stays a
  kind.
- `validateEdgeV2` was `validateEdgeV1` rule for rule at this bump, and was
  written out again rather than delegated to. That is not ceremony: the first
  rule any version states is which version the record declares, and v1's says
  1 — so a v2 edge judged by v1's rules would be refused for the stamp the tool
  itself applied, and the write gate, being the current version's rules, would
  refuse every edge `link` wrote. A version is a whole rule set, and the version
  rule is one of them. Writing it out is also what let the edge-type set widen
  later without version 1's moving; see the section above.
- `ScopePlan` survives as a Go constant. `scopesV1` names it and version 1's
  rules are frozen, which makes it version 1 vocabulary rather than dead code.

## 0.1.0 — unreleased

First version. Establishes the record schema, which is the durable decision
here — the storage engine behind it is meant to be swappable, so the shape of a
record matters far more than the fact that it currently lives in JSONL. Which is
also why the schema ships already able to move: every record declares the
version it was written against, so narrowing the shape later costs a version
bump instead of a migration.

- Append-only JSONL store with advisory locking, so parallel subagents cannot
  interleave halves of two records on one line.
- Entries and edges as separate line kinds; relations never require rewriting
  the record they relate to.
- ULID identity plus a human `slug`, so promotion between scopes rewrites no
  references.
- Retrieval contract: `query` reports its match count and withholds records
  above a threshold rather than flooding the caller's context.
- Two-directional graph walk, scope promotion, and read-time rendering of
  sequential human numbers.

### Schema versioning

- Every entry and edge carries `"version"`, stamped by the tool, first on the
  line and without `omitempty`. There is no flag to set it. A record with no
  `version` key reads as version 1 and is not rewritten to add one — it renders
  as 1 too, since version 0 names no schema that ever existed.
- Validation is in two halves. At write time `add` and `link` judge against the
  *current* schema and refuse anything it disallows; the log-wide `validate`
  dispatches each record to the complete rule set for the version that record
  declares. Without the split, the first narrowing change would condemn every
  record written before it, leaving only bad answers: rewrite history, or keep
  dead values alive forever.
- Version 1's rule set is frozen. A later version states its own in full rather
  than as a diff, composed from the same check helpers so no rule has two
  implementations. The write path reads its rules out of the same table the
  sweep dispatches through, so a bump cannot register in one place and be
  forgotten in the other.
- A version this binary has no rules for is reported as `unjudged` — counted,
  with its line number, separately from `problems`, and without failing the
  sweep. "Not checked" is not "checked and fine", and an older binary meeting a
  newer binary's records is not a corrupt log.
- `validate` still reports duplicate ids and dangling edges with line numbers,
  and it applies both to records it could not judge: they are properties of the
  log, not of a record's shape.
- The read path accepts keys it does not know, so a field added by a later
  version does not make the line malformed. Typo protection moved to the write
  path, where the CLI refuses an unrecognised flag.
- `memory kinds` sources its scope list from the current schema's set instead of
  a literal, so a version that narrows it narrows the advertised list too.
- The compatibility rule — additive only, never rename, never repurpose,
  narrowing a closed set is a version bump — is written down in the README, with
  what a new version obliges its author to do.
