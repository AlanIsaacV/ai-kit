# Changelog

## Unreleased

### `render` is gone, and a batch names the key whose shape is wrong

`memory render --as KIND` printed markdown for every record of a kind, numbering
them `ADR-0001` and up at read time. It was invoked zero times across the 2,000
transcripts measured for the surface, and nothing in this repository names it —
not a skill, not an agent, not a script, not the Makefile, not the plugin
manifest. Every other command has at least one of the two.

- `memory render --as KIND` is gone, and so are `internal/memory.Render` and the
  two tests that exercised it. A command with no caller and no reader is not a
  surface, it is a claim about one — and it is not free either: every line of
  usage text is read by the agent deciding what this tool can do. The read-time
  numbering goes with it; nothing else computed one. `SupersededIDs` and
  `AnsweredIDs` stay, because `--current` and the objective tree read them.
- **A wrong-shaped `about` or `anchor` is refused by its key on every
  toolchain.** That refusal is a `*json.UnmarshalTypeError` because the decoder
  fills `Field` in on that one type as it unwinds — which Go's `encoding/json`
  does for a plain type mismatch, and does not do, since the v2 decoder, for an
  error handed back from an `UnmarshalJSON` method. So `{"about": 5}` and
  `{"anchor": 7}` had gone back to refusing identically, with `raw` truncated at
  120 characters and the key nowhere in the answer. The key is now read off the
  line itself, which is the one place it is certainly written, and the decoder's
  `Field` is still used where it carries one.

### `anchors` answers which records drifted, which cite a path, and what one record says

`memory anchors` checked every anchor of every record its filters matched and
handed back the whole set. That is the right answer to "how is the log doing" and
the wrong answer to every question anybody actually asks, which is narrower. This
repository's log today: `--kind gotcha` reports `matched 95, checked 95, changed
47, still_holds 48` — forty-seven drifted records, and no invocation that returned
only those. The narrower question was answered by piping the JSON through Python.

```bash
memory anchors --kind gotcha --verdict changed   # only the records that drifted
memory anchors --path tools/memory/anchor.go     # only the records citing that file
memory anchors --id <id>,<id>                    # only these records
```

- **`--verdict` filters the record's verdict, never an individual anchor's.**
  REJECTED: matching per anchor, which would return a record with one rewritten
  citation under `still_holds` — the reading that under-reports drift, and does it
  silently. A record's verdict is its worst anchor's, and that is the only verdict
  this flag looks at. It takes one of `still_holds`, `changed`, `could_not_check`,
  one value and not a comma-separated list, since three values exist and two of
  them are already the complement of the third; a fourth value is refused naming
  what it was handed.
- **`--path` is a case-insensitive substring** over an anchor's `path`, the way
  `--text` already matches title and body on the same command. REJECTED: globs and
  regexes, because one matching idiom per command surface is worth more than
  expressive matching on one flag; and the name `--anchor-path`, because `anchors`
  is the one command where an unqualified `path` has a single referent. A record
  citing the same file twice comes back once, carrying every anchor it was written
  with rather than only the ones that matched.
- **`--id` takes a comma-separated list and reports `not_found`**, the contract
  `query --id` established, because two commands answering the same flag
  differently is what stops an agent trusting either. An id comes back under
  `not_found` when no record carries it, when the record it names carries no
  anchors, and when another bound excluded it — the alternative being an answer
  that is silent about a record it never looked at.
- **The three compose with each other and with every existing filter by
  conjunction.** No precedence rule, and none of them turns the count-before-dump
  guard back on: it stays off for this command, and `--threshold` is still an
  error here rather than an override.
- **`checked` and `counts` still describe what the check did**, over every matched
  record that carried anchors, so `--verdict changed` reads "47 of the 95 I
  checked". REJECTED: narrowing them to the records that came back, which would
  make `checked` mean "carried anchors" on one invocation and "survived the
  filters" on the next, and the gap between `matched` and `checked` stop naming
  the records that had no anchors. `returned` says how many `--verdict` and
  `--path` admitted and appears only when one of them was named; `not_found`
  appears only under `--id`. **An invocation naming none of the three answers
  byte-for-byte what it answered before.**
- **Records carrying no anchors are still skipped**, and this did not become the
  place to report them. `--scope both`, `--count`, `--since`/`--until` and
  `--threshold` are still not flags on this command, and an anchor is still
  checked against the working tree rather than a git ref.

### `query --scope both` reads the repository log and the user log as one answer

The measurement behind `--id`, `--fields` and `edges` counted 23 callers reading
both logs by hand, and 3 of them needed both at once: a record fetched by id from
the user log, not found there, then looked for in both files with a loop that
printed which one held it. With `--id`, `--text` and `--kind` in place that was two
invocations and a merge the caller did itself. It is now one.

```bash
memory query --scope both --id <id>
memory query --scope both --kind preference --about persistence --current
```

- **One `records` list, and every record keeps its own `scope`.** REJECTED: one
  section per log the way `areas` reports, whose per-log `not_found` would call an
  id missing from the repository section while the user section held it. The
  envelope says `scope: "both"` and names the two logs under `paths`, repository
  first, in place of `path`. `plan`, `count`, `returned`, `truncated` and `records`
  are always there, and `hint`, `not_found` and `current_objective` appear under
  exactly the conditions they do on one log.
- **Each log is filtered on its own, and only the matches are merged.** REJECTED:
  concatenating the two logs' lines into one `Query`, which would build one
  objective tree over two unrelated hierarchies. `Query` is now its matching loop
  and its bounding tail as two functions, and `QueryEach` runs the first per log
  and the second once over the union, so `count`, `--count`, `--limit`, the guard
  and `not_found` are the union's with no second copy of any of them. The loop
  itself did not change. `current_objective` is the repository log's, the same
  object `--scope repo` returns.
- **Nothing is deduplicated.** A record `promote` copied into the user log comes
  back twice, once per scope, and counts twice. REJECTED: collapsing by id, which
  would hide that it lives in both.
- **A missing user log adds nothing, at exit 0**, the way `areas` reads it — and
  like `areas`, the read creates `~/.ai-kit/` on a machine that never had one.
  Outside a git repository `both` refuses with exactly the `bad_scope` that
  `--scope repo` gives there, before the user location is touched.
- **`both` is a value of one flag on one command, not a scope.** REJECTED: a
  `ScopeBoth` in `internal/memory`, which would reach `Resolve`, `ValidScope`,
  every write and the `scopes` that `memory kinds` publishes, where widening the
  set is a schema version; and a separate `--both` flag, which could contradict
  `--scope user`. `search`, `edges`, `graph`, `anchors`, `state`, `validate` and
  every write refuse `--scope both` with the same bytes and exit code as before,
  and `kinds` still publishes `repo` and `user`.
- **What an existing invocation now answers differently is the help text, and
  nothing else.** `memory help` and `memory` with no arguments print a usage whose
  `--scope` line names `both` on `query` and which gained a paragraph describing
  it, and `memory query -h` prints the new `--scope` help. `search -h` is
  unchanged, because the help is rewritten on `query`'s flag set alone. Measured
  against a binary built from `f32dfe2` over one scratch repository and throwaway
  HOME: 55 read invocations covering every subcommand the binary dispatches —
  `query` with no `--scope`, with `--scope repo` and with `--scope user` among
  them — and 8 writes (`init`, `add`, `add --batch` from a file and from stdin,
  `link`, `promote`) compared with fresh ids and stamps masked, all byte-identical
  on stdout with the same exit code; so are 14 `--scope both` refusals and `kinds`.

### `edges` answers without a starting id, and `graph` can no longer dump

The same measurement that produced `--id`, `--since`/`--until`, `--count`,
`--fields` and `--body-lines` counted a shape no subcommand had at all: a question
about the edges themselves. Every way into the graph demanded `--from <id>` — the
id the caller was trying to find — so "show me every `supersedes` edge" and "what
got linked last Tuesday" were a `grep` over `.ai-kit/memory.jsonl`. And the walk
that was the only way in had no brake on it: `query` refuses to hand back more
than 25 records without being asked twice, while `graph` would serialize every
node it could reach, each one carrying a complete record body.

```bash
memory edges --type supersedes                 # one relation, across the whole log
memory edges --to <id> --since 2026-09-01      # what pointed at it this month
memory edges --count                           # how many, and no edges
memory graph --from <id> --threshold -1        # the whole walk, however big
```

- **`edges` is a subcommand of its own**, not a mode of `graph` and not a flag on
  `query`. REJECTED: `graph` with an optional `--from`, which is one command
  meaning two things whose result changes shape on an absent flag; and
  `query --edges`, which would put a second `records` shape inside the command
  carrying the most live gotchas in this tool. The price is one duplicated helper —
  `runQuery` is hardwired to `Query`'s `Filter` and `Result` types, so `edges` has
  a parallel one — and it was paid to leave `query`, `search` and `graph` untouched
  while adding a fourth read.
- **It filters on what an `Edge` carries and on nothing else**: `--type`, `--from`,
  `--to`, `--since`, `--until`, with `--count`, `--limit` and `--threshold` for the
  size of the answer. An edge has no plan, no `about` and no body, so `--plan` is
  refused with `plan_not_on_edges` and says why, rather than being accepted and
  ignored the way `graph` and `state` accept it. REJECTED: joining against the
  entry map so that a plan, a kind or an area of the records at either end could
  narrow the list. That join is a real question and it is the next piece of work.
  The refusal fires on the flag alone: `AI_KIT_PLAN` in the environment feeds the
  same field, and refusing on that would refuse every session that exported a plan
  slug and never typed the flag.
- **`--type` refuses a value outside the closed set** and names the eleven it
  takes, where `query --kind` lets an unknown kind quietly match nothing. Here an
  empty list would be indistinguishable from a relation nobody has drawn yet.
- **`graph` now carries the same count-before-dump guard, on by default at 25.**
  Each node embeds an entire record, body included, so bounding `len(nodes)` is
  bounding exactly what the guard on `query` bounds. Above the threshold it returns
  `nodes: []` with `truncated: true` and a hint naming only what `graph` accepts —
  a smaller `--depth`, or `--threshold -1` for the whole walk. REJECTED: a
  `--limit` on nodes. A graph is emitted in traversal order, so the first N nodes
  of one is a partial graph that misreports its own shape, saying a record has no
  children where the cut landed; "the N most recent records" is an answer and "the
  first N nodes" is not.
- **One existing invocation answers differently, and that is the whole of the
  break.** A `graph --from <id>` whose walk reached more than 25 nodes returned all
  of them yesterday and returns none of them today. This was chosen with its cost
  known: shipping the guard dormant behind an explicit `--threshold` would have
  left the unbounded dump as the default, which is the thing it exists to stop.
  `--threshold -1` gets the old answer back — measured byte for byte against a
  binary built before the change, over a 31-node walk, 20,093 bytes identical.
- **Everything else is where it was.** Measured against a binary built from the
  working tree before this change, over 39 invocations covering every subcommand
  the binary dispatches — including `graph` at depths 0, 1 and 3, an unknown id and
  a missing `--from` — every pair is byte-identical on stdout, on stderr and in its
  exit code, with `edges` the only difference, since it did not exist. What did
  move is the usage text: the synopsis gained an `edges` line, `graph`'s gained
  `--threshold`, a paragraph was inserted, and `memory graph --help` prints
  `--threshold` and a `--depth` line that now says what `--depth 0` does. `graph`
  answering with the record at `--from` alone for `--depth 0` is not new behaviour;
  it was already true, undocumented and untested, and is now all three.

### `query` returns the fields you ask for, and the head of a long body

The measurement that produced `--id`, `--since`/`--until` and `--count` counted two
more shapes a caller had to assemble for itself: 270 workarounds for reading part
of a long body and 96 for putting the CLI's output through another program, out of
1,308. Neither is about which records come back, so neither could be a filter —
both are a projection over the records `query` already matched.

```bash
memory query --kind gotcha --fields id,title
memory query --id 01KZ2WFZ2ASGEZFYG12NQ6QHW6 --fields title,body --body-lines 10
memory query --about tools/memory/ --body-lines 0
```

- **`--fields` is a free list of the record's own keys, not a set of named shapes.**
  Any subset of `version`, `id`, `kind`, `slug`, `scope`, `at`, `about`, `title`,
  `body`, `anchors`, `origin` comes back, and nothing else does: `--fields title`
  returns titles with no id, which is what `jq` was being run for. REJECTED: named
  shapes (`--brief`, `--titles`), which would be this tool guessing which four keys
  a caller meant, and a bare `--no-body`, which answers one measured shape and none
  of the others. A name outside the eleven refuses the run and names the field, the
  way `add --batch` names the line and the key — a key dropped in silence is
  indistinguishable from a record that does not carry it. A field that *is* named
  comes back even where the record has nothing under it, so every row of the answer
  has the same shape; a whole record's `anchors` and `origin` still omit themselves
  when empty, because nothing about an unprojected record changed.
- **`--body-lines N` is a leading line count, and its sentinel is load-bearing.**
  `-1`, the default, is the whole body; `0` is no `body` key at all; `N` is the
  first N lines, and a body of N lines or fewer comes back whole. REJECTED: a byte
  range and a window around the `--text` match — lines, because `SplitLines`
  already cuts content the way an editor numbers it and nothing in the read path
  ever indexes a body by byte. The default is `-1` rather than `0` because Go's
  zero value for an unset int flag is `0`: a scheme where `0` meant the whole body
  would have changed the shape of every existing invocation without anyone touching
  one. This log's median body is two lines, which is how much an off-by-one here
  would cost. Named beside a `--fields` list that does not ask for `body`, it does
  nothing rather than refusing.
- **The projection runs after the filter and never inside it.** It is a function in
  `internal/memory` applied to what `Query` already returned, so `Query`'s
  signature, its filtering loop, `Result.Records`'s type and `Areas` are untouched.
  The loop is the part that has to keep returning the same records, and not
  touching it is how that is guaranteed rather than argued.
- **`query` answers a zero match with `[]` and never with `null`.** `Query`
  initialised the slice and then overwrote it with a nil one, so `null` meant
  "nothing matched" and `[]` meant "the guard is withholding" — two answers with no
  documented distinction, which `--id` turned from rare into ordinary. This is the
  one existing answer this change makes different, deliberately. `not_found` is
  exactly what it was: present whenever `--id` was passed, naming every id the
  query did not match, at exit 0.
- **On `query` only.** `search` keeps its flag set, its result map and its hint
  untouched, for the reason `--id` and `--count` are not on it either: every flag
  costs weight in the MCP contract this surface is heading for, and the measurement
  does not show these shapes missing from `search`. One consequence is visible and
  is left visible — `search` still answers a zero match with `null`, because
  changing that would be changing `search`. The count-before-dump hint on `query`
  now names the two new flags among the ways out and `search`'s names neither.
  Three places in all are where an existing invocation's bytes moved, and this is
  the whole list: the `records` key on `query`'s zero-match reads, that hint text,
  and the usage string — `query`'s synopsis line gained the two flags, a paragraph
  describing them was inserted, and `query --help` prints two more entries, so
  `memory` with no arguments and `memory query --help` both answer differently.
  Measured against a binary built from `8fa281a` over 35 `query` and `search`
  invocations covering every pre-existing flag: 28 byte-identical, six differing
  in `records` alone on a zero match, one — `query --threshold` with the guard
  tripped — differing in the hint.

### `query` answers by id, inside a time range, and with a count instead of records

Measured over 2,000 transcripts, an agent went around this tool once per 7.0 uses:
1,308 direct reads of `.ai-kit/memory.jsonl` against 9,122 shell calls carrying a
`memory` subcommand. 80% of those bypasses happened in sessions that had already
used the CLI and only 10% followed an error — the tool answered, and the answer
was the wrong shape. Three of the seven shapes a caller had to assemble for
itself were a field on `Filter` and a check in the loop that was already there.

```bash
memory query --id 01KZ2WFZ2ASGEZFYG12NQ6QHW6,01KZJKE2HPVT5W56VS76JYSXTW
memory query --since 2026-08-01T00:00:00Z --until 2026-08-31T23:59:59Z
memory query --count --kind gotcha --about tools/memory/
```

- **`--id` is a bound on `query`, not a `get` of its own.** Ids are the currency of
  this log — edges point by id, skills pass them between steps, records cite them in
  their bodies — and until now the only way to ask for one was `graph --from <id>
  --depth 0`, which is a consequence of the walk loop rather than a feature. REJECTED:
  a `get` subcommand, which would make one concept two tools the day this surface
  becomes MCP, and would need its own answer for what `--current` and `--plan` mean
  next to it. As a bound it composes with all of them for free. `graph --from <id>
  --depth 0` is left exactly as it was.
- **An id nothing carries is named under `not_found`, at exit 0.** A caller asking by
  id already believes it knows what is coming back, which is exactly the case where a
  silently short answer is expensive. REJECTED: refusing the run, which would make one
  stale id in a list of five hide the four that are there — and the exit code in this
  CLI says whether the command ran, not whether the answer was the one you hoped for.
  The key is present whenever `--id` was passed and absent otherwise, so an empty list
  means "every id you named is in there" rather than "nobody asked". An `--id` that
  parses to no id at all — an unset variable, most often — is refused (`empty_id`)
  rather than read as no bound, which is the one case where reading the whole log
  would answer a question nobody asked.
- **Both time bounds are inclusive, and both are compared as strings.** A record
  stamped exactly at `--since` or exactly at `--until` is returned. `at` is RFC3339
  and RFC3339 sorts lexically, exactly as a ULID does, so the whole filter is two
  string comparisons and there is no date parsing anywhere in the read path.
  REJECTED: an exclusive `--until`, which is the other defensible choice and reads
  the way nobody says a date range out loud. What the string comparison costs is
  that a bound is matched as a prefix: `--until 2026-08-01` excludes everything
  stamped later that day, and the `--help` text and the README say so.
- **`--count` reports the number and returns no records.** It is the same number the
  same query without it reports, which is what makes it an answer to "how many"
  rather than a second filtering path to keep in step. REJECTED: a `count`
  subcommand, for the reason `--id` is not a `get`. The count-before-dump guard is
  neither in its way nor holed by it: `--count` answers with a number whatever the
  guard would have done — under the threshold, over it, with `--limit`, and with the
  guard disabled — and never with records.
- **On `query` only.** `search` shares `Filter` and the same result-building helper
  and keeps exactly the flag set it had. The measurement does not show these bounds
  missing from it — 196 invocations against `query`'s 3,607 — and every flag costs
  weight in the MCP contract this surface is heading for. Eleven existing
  invocations across `query`, `search`, `areas`, `anchors`, `graph` and `state`
  print byte-identical output before and after the change, with one exception: the
  count-before-dump hint, which each command now renders from the flags it actually
  has. `query`'s names `--id`, `--since`/`--until` and `--count` among the ways
  out, since a caller holding zero records is the caller these bounds were built
  for; `search`'s names `--current` and `--limit` and nothing else, which is what
  `search` accepts — it never accepted `--about` or `--kind` either, and advising a
  flag a command does not have is advice that fails silently, since `search` reads
  its text from the first non-flag argument and everything after it.

### `add --batch`, a guard on unsplit arguments, and how a record is retired

Two steps of the workflow write records by the dozen, each one a separate `add`
carrying a multi-paragraph `--body` typed into whatever shell the turn happens to
have. That is a large quoting surface bought for nothing — and the failure that
actually happened was the shell's: an argument arrived as one word looking like
`--kind decision`, which Go's `flag` reports as an undefined flag named
`kind decision` and which reads like a bug in this tool.

```bash
memory add --plan jwt-migration --batch close.jsonl   # 25 entries and 3 edges, one write
memory add --plan jwt-migration --batch -             # or from stdin
```

- **Every entry in one write, or nothing at all.** REJECTED: appending the valid
  lines and reporting the rest, which in an append-only log leaves a caller to work
  out which half landed. A bad line refuses the whole batch, names the line and the
  field, and the log's bytes are what they were. No new store primitive was needed:
  `Store.Append` was already variadic and already takes the advisory lock once for
  the whole call.
- **A line carries the fields the flags carry and never the ones the tool stamps.**
  REJECTED: accepting whole records in their on-disk shape, which would let a caller
  stamp a version or an id — the two fields whose only value is that no caller
  chooses them. An unknown key is refused by name, which is also where the flag
  parser's typo protection went: `titel` in a batch fails the way `--titel` does.
- **A key whose value is the wrong shape is refused by that key too.** `about` and
  `anchor` take a string or an array, and a refusal from the type accepting both used
  to carry no field at all — so `{"about": 5}` and `{"anchor": 7}` produced
  byte-identical refusals, and `raw` truncates at 120 characters, which on a line
  carrying a multi-paragraph body ends long before the key. REJECTED: naming the key
  in the message from inside the unmarshaller, which cannot know it: a method is
  handed the value and nothing else. The refusal is a `*json.UnmarshalTypeError`,
  the one error type the decoder fills a field in on as it unwinds, and the wording
  is rendered from that rather than from json's own, which names a Go type nobody
  reading a JSONL file has in front of them.
- **An edge on a line names exactly one endpoint, by an id already in the log.**
  The entry on that line is the other end, on the side the line left out. REJECTED:
  caller-invented local refs spanning the batch, which buys edges between two new
  records — something neither caller needs — at the price of a ref namespace and a
  dangling-ref failure class. It is also why edges *within* one batch are
  inexpressible rather than forbidden: neither entry has an id until the write.
- **A batched edge passes the same three log-wide checks `link` applies** —
  dangling endpoints, state-edge justification, the objective tree — and any of them
  refuses the whole batch. A second write path for edges that skipped them would
  have been a hole in all three. Each edge is judged against the log plus the batch
  edges *before* it, never against itself.
- **A refusal about a batched edge names the line, never the id the run stamped.**
  The tree check states its refusal in ids, which is right for `link` — both
  endpoints are on disk and either can be looked up. In a batch one of them is the id
  stamped for an entry the refusal means will never be written, so it names nothing
  that exists or ever will, and a reader takes it for a record and searches the log.
  Any such id is now rendered as *the record on line N*; the ids left in a message
  are the ones the log holds.
- **The origin every batched record carries is built once per run, not once per
  line.** Building one spawns two processes — `git remote get-url origin` and
  `git config user.name` — and `firstNonEmpty` evaluates every argument it is given,
  so a set `--author` or `AI_KIT_AUTHOR` does not skip the call. Measured with a
  counting wrapper on `PATH`: 2 spawns for one line, 20 for ten, 400 for two hundred,
  where a 200-line batch spent 2.3s almost entirely waiting on subprocesses; now 2 at
  every size, and 0.04s at 200 lines. REJECTED: memoizing inside the origin builder,
  which leaves a process spawn hiding behind a call that reads as pure. The plan is
  the only part of an origin a line may speak for itself, so the base is copied per
  line by value and only that field is written into the copy.
- **`--batch` and the flags that describe one record are refused together.**
  REJECTED: letting the file win silently, which would put a `--title` nobody sees
  into a log that cannot take it back.
- **An empty batch is a refusal.** At exit 0 it is indistinguishable from a file
  whose lines went somewhere else.
- **Every subcommand refuses an argument in flag position that begins with `-` and
  contains a space**, before parsing a single flag, quoting the argument and asking
  whether an unquoted variable failed to split. REJECTED: a scan over `os.Args` in
  `main`, which would refuse `--body "--about persistence"` — a legitimate body this
  repository's own log contains. Telling a flag from a flag's value needs the
  FlagSet, so the guard is one helper each of the thirteen subcommands calls in
  place of `fs.Parse`. REJECTED: exempting the inline `--flag=value` form, which
  would let a space through in the one shape where it is legitimate; the cost is
  that `--text='key rotation'` must be passed as two arguments, and no command block
  in this repository writes it the other way. The one argument that is not a flag's
  value and is still in flag position is `search`'s text, so `search '--about auth'`
  is refused while `search 'key rotation'` is not — narrower than it sounds, since a
  text beginning with `-` was already refused as an undefined flag at exit 2, and the
  README says so rather than claiming a search text may say anything.
- **A record written by mistake is retired with `supersedes`, and the README now
  says so.** REJECTED: a `retract` subcommand with a tombstone kind, a `retracts`
  edge type and a read that skips it by default — measured against one junk record
  in 806 entries, and a second invisible filter would contradict the one claim this
  log makes, which is that a read returns what was appended. What was missing was
  the sentence, including that `--from <new> --to <old>` drawn backwards is accepted
  at exit 0 and leaves the wrong record live.
- **No schema obligation is triggered.** No kind, no edge type, no scope and no
  record field changed; `CurrentSchemaVersion` stays at 3 and the closed sets are
  untouched. `--batch` and the guard are CLI-level, and a batched entry goes through
  the same write gate a flagged one does — `currentRules`, which never dispatches on
  a record's declared version.

### `areas` enumerates what a log can be asked about

`--about` is the only carrier of a preference's identity and it is freeform, so a
reader who guessed the tag wrong got `count: 0` — the same bytes as the answer
meaning nobody ever wrote one. A preference that existed and a preference that did
not were indistinguishable to anyone who did not already know the tag, and the two
callers about to depend on it both have to check whether an area already has an
answer before they coin a second key for the same thing.

```bash
memory areas                      # every tag in both logs, with counts
memory areas --kind preference    # only the tags preferences are keyed by
```

- **Every distinct tag is a row, not only the first.** REJECTED: enumerating keys
  alone. A preference tagged `migrations,db` has to stay findable under `db`, and
  `gotcha` is not forced to obey a keying convention that belongs to one kind.
  Each row's `live.key` says whether the tag is the first `--about` of the record
  the row names, so the convention is reported and still not enforced.
- **`records` counts the settled too, and `live` is asked separately.** Counting
  only current records would hide that an area has been answered three times,
  which is the history a writer about to answer it a fourth time is looking for.
  `live` is the most recent record carrying the tag that survives `--current` —
  defined generically rather than for `preference`, and coinciding exactly with
  the preference convention under `--kind preference`. It is `null` when every
  record carrying the tag is settled.
- **Both scopes, reported apart, and this is the first command whose default
  reads two logs.** It follows `validate --all`: each scope resolved on its own,
  each carrying the path it resolved to. REJECTED: one merged list, in which no
  row says which log it came from. There is therefore **no `--scope` flag** — the
  command has nothing to choose between, and an accepted-and-ignored flag would
  look like it chose.
- **A missing user log is a user scope with zero areas at exit 0.** REJECTED:
  omitting the section, which would leave "I have never promoted anything"
  indistinguishable from "I did not look there". It resolves through the same
  best-effort read path every other read uses, so nothing new decides this.
- **The plan-less rule is inherited, not restated.** `Areas` calls `Query` twice
  — once for the counts, once with `--current` for the live answers — rather than
  walking the entries itself, so `--plan X` enumerates X's tags *and* the
  repository's exactly as a query would, and there is one place where that rule
  is written down.
- **No threshold.** Rows are aggregates naming one record each and carrying no
  body, and this is the cheap read that tells a caller how to narrow the expensive
  one; withholding it over 25 areas would defeat the point.
- **Tags are folded to lower case and nothing else is merged.** `--about`
  compares case-insensitively, so splitting `Persistence` and `persistence` into
  two rows of one record each would contradict the count `query --about
  persistence` returns. Two tags that merely *mean* the same thing stay two areas:
  making that visible is as far as a read-only enumeration goes.
- **No schema obligation is triggered.** No kind, no edge type, no scope and no
  record shape changed; `CurrentSchemaVersion` stays at 3 and the closed sets are
  untouched. What it owed was the usage string, the README and tests, and it has
  them.

### `promote` selects by class, reaches one hop, and reports what is half-connected

`promote` did one thing safely and nothing else: you named records by id, it
copied them, and it counted the edges it copied. Sending a class of knowledge
upward meant querying for it, extracting the ids and pasting them back; an `adr`
travelled without the `decision` that fed it; and the report counted
`edges_copied` and never mentioned the ones it dropped.

```bash
memory promote --kind preference --to user --dry-run   # what would move
memory promote --kind preference --to user             # move it
memory promote --ids <id> --kind adr --to user         # both, as a union
```

- **`--kind` takes four kinds and no others**: `adr`, `postmortem`,
  `preference`, `dead-end`. `decision` is REJECTED from the set because the loop
  writes one at the close of every objective, so sweeping the class would carry
  hundreds of records about this repository's own code into a log meant to hold
  what outlives it. `gotcha` because it cannot exist without an anchor and an
  anchor is resolved against `origin.repo` — a `path:line` means nothing in the
  next repository. `review-finding` because it names a defect in one
  repository's code at one moment. The set is `promotableKindsV3`, sited beside
  `anchoredKindsV3` where somebody adding a kind will be standing, and published
  by `memory kinds` as `promotable_kinds`. It bounds only what may be named as a
  *class*: any record still promotes by id, and nothing validates it.
- **A kind outside the set is refused by name, with the four listed.** Including
  a kind that is perfectly valid to write, like `decision` — `toKinds` validates
  nothing, so the alternative was a promotion that moved nothing and a caller
  who cannot tell that from an empty class.
- **`--kind` and `--ids` are a union.** REJECTED: making them mutually
  exclusive. Either alone is enough, both together promote everything either one
  named, a record named twice is promoted once, and only naming neither is
  refused — with a message naming both. The refusal's code changed from
  `missing_ids` to `missing_selection`, since it is no longer about ids.
- **Promotion reaches one hop past what it was given.** A record nobody named
  but that an edge joins to one that was named travels too, so an `adr` stops
  arriving without the reasoning that justifies it. REJECTED: the full transitive
  closure, which is the mechanism that would lift a whole repository's
  scaffolding into the user log. The hop is measured from the named set only, and
  it is a pass over the edge list rather than a call to `Graph()` — that function
  is single-source and shadows same-pair edges by walk order
  (`01KZ4ZXEPR07E7VH06JTG8E6J0`).
- **The reach never pulls in an `objective` or a `gotcha`.** Both were a back
  door into the user log for exactly what the four-kind set keeps out of the
  front one. An `adr` very often `derives_from` the objective that produced it,
  and an objective names work that happened in one repository. A `gotcha` cannot
  exist without an anchor, whose `path:line` resolves against `origin.repo` — the
  reason `--kind` refuses the class, and an edge into one would have carried it
  up regardless. It bounds the reach and not the selection: either kind named in
  `--ids` is promoted like anything else. And neither exclusion is silent, which
  was the condition of having them at all — the skipped record is reported in
  `border` and its edge counted in `edges_skipped`, like anything else left
  behind.
- **The report names what sat at the border**: `border` with `border_count`, and
  `edges_skipped`. Both are **about the destination and not about this run** —
  the test is whether a record will be over there when the promotion is done,
  selected by it *or already present*, so promoting an unrelated `preference`
  will name a record some earlier promotion left half-connected. That is the
  honest answer to "what is still half-connected over there", which is the
  question worth asking of a log nobody can tidy; the narrower reading goes stale
  the moment you stop reading the reports. The cost is accepted and stated: the
  border list grows as the user log fills, and there is no flag to narrow it. An
  edge with neither endpoint at the destination is not counted — that number
  would measure the rest of the graph. REJECTED: reporting each skipped edge as a
  triple; REJECTED: refusing when skipped edges outnumber copied ones; and
  REJECTED: narrowing the code to report only what this run moved, which is what
  the wording used to promise.
- **The both-endpoints rule is untouched.** The expansion adds records so that
  more edges qualify; it never admits an edge whose endpoint is absent. Open
  question `01KZ38XQTXFQZRVG6VMPRA82SK` — a state edge promoted without its
  justification — is made less likely and is not closed.
- **`--dry-run` prints the report the real run would print**, plus
  `dry_run: true`, and writes nothing: not the records, not the edges, not the
  destination directory. REJECTED: a preview format of its own, and REJECTED:
  making it a mandatory confirmation, which would break every non-interactive
  caller.
- **`origin.repo_id` tells two repositories with one directory name apart.**
  `origin.repo` is `filepath.Base` of the root, so two checkouts called `api` are
  one string in a log holding records from both. The identity is the `origin`
  remote's URL where there is one and the repository's absolute path where there
  is not, and absent where neither could be read — REJECTED: a required field
  with a sentinel, which would have made "no answer" look different from the
  answer every record written before this field gives. `--repo` overrides the
  label and not the identity.
- **No version 4, and no migration.** `CurrentSchemaVersion` stays at 3: no
  version's rule set has ever inspected `Origin`, so a record lacking the key
  cannot fail and one carrying it cannot either. Nothing is backfilled — the log
  is append-only, and the identity applies to records written from now on.
- **The git shell-out stays in the CLI layer**, beside `gitUserName`, so
  `internal/memory` remains stdlib-only and free of `os/exec`. REJECTED: reading
  `.git/config` directly, and REJECTED: putting the call in the library.
- **Idempotency is now proven rather than asserted.** `TestPromotionIsIdempotent`
  passed `nil` for both edge slices, so the edge-dedup path had never been run by
  a test, and no test invoked the `promote` subcommand twice at all. Both gaps are
  closed: a unit test with real edge slices on both sides, and an end-to-end test
  that runs the built binary twice over the same two logs and compares the
  destination's bytes across the second run.
- **`TestPromotionLeavesHalfEdgesBehind` is deleted.** It seeded an `adr`, the
  `decision` it derives from and nothing else, promoted the `adr` alone, and
  asserted the edge was not copied — exactly the case the one-hop reach exists to
  fix. REJECTED: rewriting it with a longer chain, which would have been a second
  copy of `TestPromotionNamesTheBorderAndCountsSkippedEdges`, where the record
  left behind is genuinely two hops out and the same invariant is proven against
  `EdgesCopied` and `EdgesSkipped` together. The one sentence of its comment that
  was still true and stated nowhere else moved to that test.

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
