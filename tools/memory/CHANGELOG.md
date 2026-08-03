# Changelog

## 0.1.0 — unreleased

First version. Establishes the record schema, which is the durable decision
here — the storage engine behind it is meant to be swappable, so the shape of a
record matters far more than the fact that it currently lives in JSONL.

- Append-only JSONL store with advisory locking, so parallel subagents cannot
  interleave halves of two records on one line.
- Entries and edges as separate line kinds; relations never require rewriting
  the record they relate to.
- ULID identity plus a human `slug`, so promotion between scopes rewrites no
  references.
- Schema validation at write time, plus a log-wide `validate` that reports
  duplicate ids and dangling edges with line numbers.
- Retrieval contract: `query` reports its match count and withholds records
  above a threshold rather than flooding the caller's context.
- Two-directional graph walk, scope promotion, and read-time rendering of
  sequential human numbers.
