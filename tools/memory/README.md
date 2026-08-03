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
<repo>/plans/<slug>/memory.jsonl   plan   the working noise of one objective
<repo>/.ai-kit/memory.jsonl        repo   distilled, promoted deliberately
~/.ai-kit/memory.jsonl             user   distilled further, across repos
```

All three are gitignored. Promotion between them is always an explicit act.

## Record kinds

| Group | Kinds | What they are for |
|---|---|---|
| Core | `objective` `unit` `decision` `deviation` | Make the work loop run |
| Learning | `assumption` `open-question` `wont-fix` `dead-end` | Stop verification theater and repeated work |
| Knowledge | `adr` `postmortem` `review-finding` | Outlive the plan, get promoted upward |

`wont-fix` earns its place: once you decide a risk is not worth covering, that
decision is recorded, and no later verification pass raises it again.

## Edge types

`implements` · `derives_from` · `supersedes` · `answers` · `caused_by` ·
`relates_to`

## Usage

```bash
memory init  --scope plan --plan jwt-migration

memory add   --plan jwt-migration --kind decision \
             --title "RS256 over HS256" \
             --body "Per-service key rotation requires asymmetric signing." \
             --about auth,pkg/auth/

memory link  --plan jwt-migration --type supersedes --from <new-id> --to <old-id>

memory query --plan jwt-migration --about auth --kind decision --current
memory search --plan jwt-migration "key rotation"
memory graph --plan jwt-migration --from <id> --depth 2

memory promote --plan jwt-migration --ids <id>,<id> --to repo
memory render  --scope repo --as adr
memory validate --all
```

Every command prints JSON on stdout on success and on failure alike — except
`render`, which prints markdown. Exit 0 means the command ran; exit 1 means it
refused, and the JSON says why.

`--about` matches by exact tag or by path prefix: a tag ending in `/` catches
everything under it, so `--about pkg/auth/` finds a record tagged
`pkg/auth/middleware.go`.

## Guarantees under test

- A record with an unknown `kind`, an empty `about`, a non-ULID id or a
  non-kebab slug is refused before it reaches disk.
- An edge whose endpoints do not both exist is refused at write time; `validate`
  also sweeps for dangling edges, because they fail silently — a graph walk
  just returns less than it should.
- Appending never changes bytes already written, including when a record is
  superseded.
- One malformed line is reported with its line number and does not make the
  lines around it unreadable.
- `query` returns the true count even when it returns no records.
- Promotion preserves ids, does not mutate the source, is idempotent, and never
  copies an edge whose other end stayed behind.

```bash
go test ./...
```

## Build

```bash
go build -o bin/memory ./cmd/memory
```

No dependencies beyond the standard library.
