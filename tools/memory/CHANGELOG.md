# Changelog

## Unreleased

### The `preference` kind, and `--plan` becoming optional on `add`

Two changes that only mean anything together: a kind for *how things are done
here, and it still holds*, and a record that can belong to the repository rather
than to an objective inside it.

```bash
memory add --kind preference --title "Migrations go through Atlas" \
           --about persistence,migrations --body "Hand-written scripts drifted twice."
memory query --kind preference --about persistence --current   # newest first
```

- **What the manifests can be asked is never written.** "This repo uses GORM
  v1.25" is derivable from the repository at the moment of asking, so it is not a
  record; *why* GORM and not sqlc is what has nowhere else to live. That is the
  line the kind is for, and it is the one thing about it a writer can get wrong
  in a way nothing will catch.
- **A kind of its own, not `decision --about stack`.** REJECTED on mechanics:
  `--about` is freeform and unvalidated, so the distinction would be a convention
  that can be forgotten in silence and added by accident, while `--kind` is a
  closed set the validator enforces — and the loop writes a `decision` at every
  objective's close, so a preference would be a needle in that haystack. It is
  not `decision` on meaning either: that kind presupposes a displaced
  alternative, and a preference may have been the only thing anybody tried.
- **No version 4, and not anchored.** `entryKindsV3` gained a third member with
  `CurrentSchemaVersion` left at 3, for the reason the two widenings below state
  in full. `anchoredKindsV3` is untouched and `gotcha` remains the one kind
  refused without an anchor: that set *narrows*, adding to it now would condemn
  records on disk, and a preference is about a choice rather than about code —
  one promoted to the user scope could carry no `path:line` meaning anything
  outside the repository it came from.
- **`--plan` is now optional on `add`.** ~~`add` stamps `origin.plan` from
  `--plan` or `AI_KIT_PLAN` and refuses when it has neither.~~ Reversed. The
  requirement was never in the schema — `Origin` is optional, `Origin.Plan` is
  `omitempty`, and no version's rule set has ever looked at either — it was a
  refusal the CLI added on its own, and three skills had each worked around it by
  inventing a fixed sentinel plan slug. A malformed slug is still refused;
  `link`, `query` and `search` are untouched.
- **A plan-less record is in scope for every plan-bounded read.** `--plan X`
  returns X's records **and** the repository's, and a record that names a plan is
  still absent from a query naming a different one. REJECTED: the smaller change
  where `--plan X` excludes plan-less records, which would have made every
  repo-global record invisible to exactly the reads that most need it. The cost
  is real and there is no flag for it: a bounded read now returns a little more
  than the turn asked for.
- **A preference is keyed by its first `--about` tag**, and the newest one
  carrying that tag is the live answer, read off the result order rather than
  derived. A `supersedes` edge is optional rather than required — it still works,
  and still drops the older record from `--current`, but preferences arrive
  months apart from writers who do not know what they are replacing. Chosen over
  a required `topic` field on the kind, with the weakness known and written down:
  a tag's content is unvalidated, so `persistence` and `database` do not shadow
  each other.
- **Nothing already on disk is re-kinded and no sentinel slug is retired.**
  `skills/bootstrap` and `skills/ask` go on writing under `repo-bootstrap` and
  `repo-questions`; removing the refusal is what makes retiring them possible
  later, and doing both at once would mean changing those skills while changing
  the CLI underneath them.
- **`memory.RequirePlan` and `memory.ErrNoPlan` are gone**, having lost their one
  caller. Both existed only to state a rule that no longer exists, and an
  exported symbol whose doc comment asserts a deleted rule is worse than no
  symbol.

### Anchors, and `memory anchors` to check them

A record may now cite the code its claim rests on as a structured field —
`{path, line, lines, hash}` on `anchors` — and one subcommand says, for each
anchored record, whether the cited code is still there. It is what gives `gotcha`
a reader: the kind had a writer and nothing anywhere read one back.

```bash
memory add --kind gotcha --title "…" --about auth --anchor pkg/auth/token.go:41-43
memory anchors --kind gotcha --about pkg/auth/
```

- **The check is the simplest thing that tells moved from changed.** It hashes
  the cited lines and compares that against every window of the same height in
  the file. No git, no AST, no symbol extraction, no fuzzy matching — so it needs
  no history and works against a shallow clone or a directory that is not a
  repository at all. REJECTED: a normalized fingerprint and a git pre-filter,
  both of which give little more information, guarantee nothing, and would have
  been guessed at before anybody had used the simple one.
- **Nothing is normalized, so a reformat reports `changed`.** Deliberate, and the
  reasoning is the whole design: a false `changed` costs a human one glance, and
  a false `still_holds` launders an unchecked claim into a confirmed one. Whether
  that noise is tolerable in real use is the thing to find out by using it.
- **Three verdicts, the ones `agents/explore.md` already states** —
  `still_holds`, `changed`, `could_not_check`. Text that moved is `still_holds`
  naming the line it is at now. REJECTED: a fourth verdict `moved`, which would
  give the system two vocabularies for one idea and two places to keep in step.
- **Text appearing more than once is `still_holds` with every match named**,
  because choosing one would be the check guessing what the record meant.
- **The check never rewrites a record.** No `--fix`, no auto-updated line. The
  log is append-only and this reports only. `--repo-root` chooses the working
  tree; cross-repo use is out of scope until something asks for it.
- **No language-model call and no network call**, and read-only over the
  repository it is pointed at. Its cost is a process, not tokens: it swept this
  repo's whole log in under 20ms when it landed, and the work is one file read
  per anchor rather than one per record, so the log's length is not what it
  scales with.
- **An unresolvable citation is refused by `add`**, not stored and reported
  later. The log is append-only, so write time is the only place a typo can be
  caught, and a `could_not_check` months later is indistinguishable from a file
  somebody deleted.
- **The field is a widening; the `gotcha` rule is not, and stayed inside version
  3 anyway.** `anchors` is optional, so every record already written stays legal
  and `CurrentSchemaVersion` is still 3. Requiring one on `gotcha` does make a
  record shape that was legal illegal — the direction that normally needs a bump
  — and it stayed because it condemned nothing: **when the rule landed there were
  zero `gotcha` records in either log**, counted rather than assumed. Past tense
  on purpose — gotchas are being written now, and every one of them was written
  under the rule, so the count that justifies this is the one taken the day it
  arrived and not one a later reader can reproduce. The rule is in
  `validateEntryV3` and in `anchoredKindsV3`; `validateEntryV1` and
  `validateEntryV2` were not touched, so a record declaring an older version is
  never asked for an anchor. Adding a second kind to that set is now a bump:
  `hypothesis` and `review-finding` both have records on disk.
- **`memory kinds` publishes `anchored_kinds` and `verdicts`**, for the reason it
  publishes the justification allowlist: a rule the tool enforces but does not
  advertise is a rule a caller only ever meets as a refusal.
- **`memory anchors` is the one read command with the count-before-dump
  threshold off.** It returns a verdict per record and no bodies, and a check
  that withheld its answers above 25 records could not be run over a log.

### The `gotcha` kind, widened into version 3 in place

A *why* that reading the code will never recover: a constraint the code obeys
without stating it, fixed by something the file cannot show — a retry count set by
somebody else's rate limit, a lock order set by a deadlock that already happened.

The gap it fills is a destination. A comment survives only if it says something
the signature and the code do not, which leaves the non-obvious *why* needing
somewhere to go; `agents/implement.md` states which of those go to the log, and
nothing in the log held the shape. `decision` is an option taken and what it
displaced, which presupposes there was a choice, and what arrives here often had
none.

- **No version 4.** `entryKindsV3` gained a member and `CurrentSchemaVersion`
  stayed at 3, for the reason the `hypothesis` widening below states in full.
  `entryKindsV1` did not gain it, so an entry declaring version 1 or 2 with this
  kind is refused by the rules it declares. Two widenings inside one version do
  not add up to a bump: only narrowing takes something back from a record already
  written, and neither of these narrowed anything.
- **No new edge type and no change to the justification map.** A `gotcha`
  justifies no state edge — it is a constraint on the code, not a decision about
  an objective — so `internal/memory/justification.go` is untouched and the eleven
  edge types are the eleven that were there.
- **The anchor is body text, not a field.** ~~The path:line a `gotcha` constrains
  is written into the body. REJECTED, for now: a structured anchor field, which
  presumes an answer to what an anchor check is — a question this repo has open
  and which a schema field written here would settle from outside the objective
  that owns it.~~ Reversed by the anchors work above, in the objective that owned
  the question: it is a field, and a `gotcha` without one is refused. The
  migration this bullet anticipated cost nothing, because no `gotcha` was ever
  written between the two changes.
- **Nothing already on disk is re-kinded.** Not by this widening, and not by `adr`
  becoming a kind the close writes — which needed no schema change at all, since
  `adr` has been in the set since version 1 and only lacked an authored path to it.
  Re-kinding a record would mean a new one with `supersedes` pointing at the old,
  and `supersedes` asserts the old was replaced, which is false for a record that
  was merely filed under the wrong kind. No edge type means "reclassified" and none
  was added.

### The `hypothesis` kind, widened into version 3 in place

A claim under test during an investigation, written with how it was tested, a
verdict, the evidence and that evidence's fidelity. It is what gives the loop's
symptom-first route somewhere to put what it learns, and the rejected ones are
the point as much as the confirmed one: they are the record of what has already
been eliminated, which is what a later session most often lacks.

- **No version 4.** `entryKindsV3` gained a member and `CurrentSchemaVersion`
  stayed at 3, because widening a closed set makes no record already written
  illegal and so has nothing for a bump to protect — the same move version 2 made
  when it gained the five state edges. `entryKindsV1` did not gain it, so an entry
  declaring version 1 or 2 with this kind is refused by the rules it declares, and
  the widening is inside one version rather than everywhere at once. The cost is
  the one the compatibility rule names: a binary built before this reports
  `kind: unknown` — a false problem — where an unknown version would have given an
  honest `unjudged`.
- **A confirmed root cause is not a kind of its own.** It is the hypothesis whose
  verdict is `confirmed`. REJECTED: a separate `root-cause` kind, which stores one
  claim twice under two names and leaves a reader to work out which copy is
  authoritative; and reusing `dead-end`, which means an approach abandoned during
  implementation and would split "what was already tried" across two kinds.
- **No new edge type and no change to the justification map.** A `hypothesis`
  justifies no state edge — it is evidence about the system, not a decision about
  an objective — and the objective a confirmed one produces is tied to it with the
  `caused_by` edge version 1 already shipped.

### Schema version 3

The `unit` kind is gone. An objective may derive from an objective, so the work
under one is another objective, and a separate kind for it named a distinction
the tool no longer draws.

The second narrowing bump, and the first one that had records on disk carrying
the value it takes away — which makes it the first evidence that the version
stamp bought what it was built to buy. It answers open question
`01KZ3472SM2FC6ERY4JV2HYXBX`, which asked what `validate` does with a record
whose version is known but whose kind was removed by a later version, and
deferred on the grounds that there were none of those. There were six.

- **`entryKindsV3` is version 1's eleven without `unit`**, spelled out as its own
  map literal. Not `= entryKindsV1` and not an edit to it: a Go map is a
  reference, so an alias is one map under two names, either of which looks safe to
  narrow — and narrowing version 1's frozen set is precisely what condemns the
  records already written. `entryKinds` *was* that alias until this bump and now
  points at version 3's set, which is what `memory kinds` prints and what the
  write gate enforces.
- **No migration, because version dispatch made migration unnecessary** — not
  because the log happened to be empty, which is what the objective above this one
  assumed when it deferred the removal. Six entries in this repo's log carry kind
  `unit`: one with no `version` key at all, one declaring 1, four declaring 2.
  `validateEntryV1` and `validateEntryV2` both name `entryKindsV1`, where the kind
  is valid, so all six and the 23 edges touching them sweep clean, judged by the
  rules they were written against. Not a byte of them was rewritten. REJECTED:
  deleting the six, which costs 29 of the log's lines and the trace of which piece
  of work produced which decision and closed which question; and a separate
  writable-kinds set governing `add` while the validators stay put, which is the
  two-lists drift the version table exists to prevent — two records both stamped 2
  would disagree about whether `unit` was legal when they were written, with
  nothing on disk saying which.
- **The kind keeps no Go constant.** `entryKindsV1` holds `Kind("unit")` as a bare
  literal instead. The value has to stay in version 1's set or those six records
  stop validating, but a constant in the `const` block is how this package says "a
  kind you may write", and nothing may write this one — so the kind now exists in
  exactly one place with nothing naming it, and that asymmetry is the signal. It is
  the one difference from how `ScopePlan` was handled, and the reason is that
  `scopesV1` is not the only thing naming `ScopePlan`: `Resolve` matches on it to
  refuse `plan` *by name*, which needs the constant.
- **`validateEdgeV3` exists although version 3 changed nothing about an edge.** It
  names `edgeTypesV2` and states every other rule version 2 stated, and the
  duplication is not optional — it is the same bill the version 2 bump paid, come
  round a second time. The first rule any version states is which version the
  record declares; every edge written from now on is stamped 3, and the write gate
  is the current version's rules, so a `ruleSetV3` reusing `validateEdgeV2` makes
  `link` refuse every edge it tries to write. Measured rather than argued: with
  the reuse in place, nine tests failed on an edge with *record declares schema
  version 3 but is being judged as version 2*, the write gate accepting a state
  edge among them. Open question `01KZ3472NSEBRE0C9XE6AP6TCC` asked whether the
  per-version validators stay maintainable past about three versions and said to
  revisit at version 3; this is version 3, and it is noted here rather than
  answered.
- **Only the kinds moved.** `validateEntryV3` names `scopesV2` and there is no
  `edgeTypesV3`, so both scopes and all eleven edge types go on meaning what they
  meant. `implements` in particular stays valid: six edges on disk use it, and
  narrowing an edge type is a bump like any other, which this version did not take.
- **`Origin.Unit` stays**, a field nothing populates. Removing it would change no
  byte on disk — no record sets it and the key is `omitempty` — which is the whole
  argument for deleting it and the reason not to: additive only means never remove
  a field, and a rule that holds except where removal happens to be invisible is a
  rule nobody can lean on. `add --unit` still fills it and still says "unit of
  work" in its help text, which is the last surface where the retired word is
  spoken.
- **The version boundary in this repo's own log falls mid-session, not at a
  release.** No line before 172 declares version 3 and no line from 172 on
  declares anything else, and the two sides sit eight minutes apart inside the
  objective that made version 3: the decision approving the work is stamped 2, the
  deviations recording how it was implemented are stamped 3. The stamps are still
  monotonic, by line and by `at` — what moved is that `CurrentSchemaVersion` in the
  source and the version actually reaching disk were different numbers for part of
  a session, because the skills' bootstrap builds the binary only when the file is
  absent, so every process went on running the old one until it was rebuilt by
  hand. That is open question `01KZ38XQRZXAR81J9MKS33AFTA` observed rather than
  reasoned about, and nothing about it is invalid: each of those lines is judged by
  the version it declares, which is the whole point of stamping per record.

### The objective tree, and everything read off it

The parent relation stops being prose and becomes a relation the tool can
enforce and answer questions about. No new edge type, no new kind, no version
bump: this is a rule and a derivation over records the schema already allows.

- **An objective derives from at most one objective, and never in a circle.**
  The parent relation is `derives_from` with an **objective at both ends** —
  positional rather than typed, so a decision deriving from the objective it was
  taken under, an objective deriving from a decision, and an ADR deriving from
  several decisions all keep meaning what they meant. `link` refuses a second
  parent, naming the parent already on disk, and refuses an edge that would close
  a cycle, naming the whole path. Several roots stay legal; nothing requires a
  parent. REJECTED: a `child_of` type, which would be unambiguous and would widen
  a closed set a second time to leave two edges asserting one relation.
- **On the write path and nowhere else**, beside `CheckJustification` and for the
  same reasons: a version's rule set judges one record in isolation and cannot
  see either endpoint's kind, and the read sweep stays tolerant so a record on
  disk is never condemned by a rule that arrived after it. A log holding a cycle
  or an objective with two parents — hand-edited, or arrived through `promote`,
  which passes no gate — is read rather than refused: the first parent edge in
  the file wins, every walk carries a visited set, and what was met is reported
  under `anomalies`. `validate` is unchanged and still calls none of it a
  problem.
- **The state precedence rule is a function at last.** The README has stated it
  normatively since the state edges shipped and nothing computed it. `memory
  state` resolves a record's state as the most recent state edge by `at`, ties
  broken by the later line in the file, with no state edge at all resolving to
  `planned`. `approved_for_execution` → `blocked` → `approved_for_execution` →
  `met` resolves to `met`, and the stale edges under it stay on disk. Answers
  open question `01KZ4ZY74599MBM6VEHW65GES1`.
- **Open is everything but `met` and `cancelled`**, and `separated` sitting on the
  open side is a decision rather than an omission: it says the work continues
  somewhere other than under the parent it was lifted out of, not that it
  stopped. So a separated objective is still returned by `--current` and is still
  a candidate for the current one, while counting toward no parent and blocking
  none.
- **`query --kind objective --current` returns every open objective.** Depth
  removes nothing, and neither does having an open child. Every other kind's
  `--current` is untouched — the terminal-state drop applies to objectives alone.
- **Which objective is being worked on is one answer, in `current_objective`.**
  The open objective carrying the latest `approved_for_execution` edge, or — when
  none does — the open objective with the most recent activity, where activity is
  the latest `at` among the objective itself and every edge with it at either
  endpoint, **ties broken by the later line in the file** — and, where an edge
  ties two objectives to one line by touching both of them, by the later entry.
  The rule that resolved it is reported with it, so it is never a guess presented
  as a fact. A field of its own, because `hint` means truncation and must go on
  meaning only that. REJECTED: deepest-open as a forced rule, which
  would make every urgent fix and every sideways move a fight with the tool;
  activity alone, which cannot be pinned; the mark alone, which costs a decision
  and an edge on every context switch. **Depth is reported and nothing filters on
  it.**
- **The derivation reads the whole log and the filter applies afterwards.** A
  `--plan` that puts the current objective out of view still gets told which one
  it is, with `outside_filter`, rather than being handed the runner-up.
- **`Query` and `NewTree` take the log's lines rather than the entries and edges
  split out of them.** Both views are still needed and both are still taken —
  but so is a third thing the split throws away, the order of the file itself.
  Two slices each ordered internally cannot say which of one's members came
  before one of the other's, and the current-objective tie-break is stated in
  terms of exactly that. It is not a corner case: `at` is written to the second
  and this loop appends a batch of adds and links at the end of a turn, so most
  lines of a live log share their second with another record, and some of those
  seconds mix an entry with an edge. `runQuery` and `cmdState` both already held
  the lines and were splitting them one step before the order was needed.
- **A parent's completion is computed**: `met` and superseded children counted
  done, `cancelled` and `separated` not counted, everything else outstanding.
  Superseded counting is decision `01KZ30RRADAT4FM5DF3QJRCT55` — absorbed work
  exists, someone else did it.
- **Blocked by an open child is derived from the tree and never written.** No
  edge expresses it and `link --type blocked` pointing at another objective is
  still refused, which is what keeps "blocked on a sibling" and "blocked by my
  own child" from becoming indistinguishable on disk. It is informational:
  nothing filters on it.
- **`memory state` is the command the README said did not exist.** It reports
  every objective's resolved state, parent, children, depth, blocking children
  and completion tally, plus the resolved current objective and any anomalies;
  `--id` narrows to one. A superseded or answered objective is listed under
  `excluded` with its reason rather than dropped, since silence is
  indistinguishable from a record that was never there. `--plan` is accepted and
  ignored, like everywhere but `query` and `search`: narrowing the derivation
  would change the answers rather than the view.
- **State and parentage are resolved from the edge list directly**, never by
  walking `Graph`, which deduplicates by node and reports only the first edge
  that reaches one — so a typed edge can be shadowed by an older `relates_to`
  between the same pair. That is wont-fix `01KZ4ZXEPR07E7VH06JTG8E6J0`, left
  as it is, and reading the edges rather than the walk is how this side of it
  stays correct.

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
  `unjudged`. **There is a version 3 now** — see *Schema version 3* above — and it
  took nothing from the edge types: this widening still needed no bump, still has
  none, and `edgeTypesV2` is still the set the current version names. The sentence
  is left standing because it was true of this change and stays true of it; what
  moved is the world around it.
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
- `add` stamps `origin.plan` from `--plan` or `AI_KIT_PLAN`, and ~~refuses when it
  has neither. A record with no plan is unreachable by every bounded query, and
  the log is append-only, so refusing is the only place to catch it.~~ Reversed by
  the `preference` work above: the premise was true only because `--plan X`
  excluded plan-less records, and that is the half of it that changed. Such a
  record is now in scope for every bounded query rather than for none.
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
  kind — **until version 3, which is the version that removed it**, and which had
  to declare a kind set of its own to do so without reaching back through this
  shared one. The four version 2 records carrying the kind are still judged here,
  and still pass.
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
