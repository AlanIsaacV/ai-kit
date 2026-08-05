# ai-kit

Skills, agents and tools for working with AI coding agents.

The first thing in here is a planning and execution workflow, built to fix
three specific failures of the one it replaces.

## The three failures it is built around

**The objective got lost.** The previous design stored the goal as one entry in
a growing log, and then — reasonably, to keep context bounded — excluded that
log from every session's start-up read. The subagent that wrote the code never
received the objective at all. So the work satisfied its tickets and drifted
away from the point. Here the objective is loaded first, stays loaded, is
passed to every subagent, and every objective has to state in one line how it
serves the one above it — or, for a root, the request that opened it.

**Phases were rigid.** Writing the full contents of every phase during planning
means writing phase five with the least information you will ever have about
it. Changing it afterwards cost enough that the plan got followed instead of
corrected. Here there is no phase list: there is an objective, and more
specific objectives opened underneath it immediately before each one runs,
against the current state of the repo rather than against a document from three
weeks ago.

**Verification graded its own homework.** Criteria written by the same model
that then checked them, satisfied by tests against mocked dependencies, passing
green while the real thing broke on deploy. Here criteria are approved by the
user before implementation and frozen; each one declares the evidence fidelity
it demands; and a criterion needing a real database cannot be reported as met
on the strength of a mock. Verification runs in a different subagent than
implementation.

## Parts

```
skills/plan    turn a request into a written objective, reviewable in full
skills/work    run one objective: pick, gate, implement, verify, review, record
agents/        explore · implement · verify · review · record
tools/memory   the append-only knowledge log everything writes to
```

`plan` runs once per objective, including each one opened underneath another.
`work` runs one objective per turn and is resumable in a fresh session. Small
work takes the same path — one turn of the loop instead of many — so there is no
second door to pick and no escalation path to get wrong.

## How the drafts work

Both skills negotiate through a markdown file rather than through chat. Every
objective is written to `plans/<objective-slug>/` — one directory per objective —
and the user reads and edits the file directly.

This is deliberate. Being shown a three-bullet summary and asked "look right?"
is how a plan gets approved that nobody read — you end up seeing the headlines
and never the detail. The file carries what was understood, **what was assumed
without asking**, and what is still unknown, in their own sections. Approval
means the file is right, and an edit to it is the decision.

Once approved, the draft is committed to memory as immutable records. A later
correction is a new record with a `supersedes` edge, never a rewrite.

## The memory log

See `tools/memory/README.md`. In short: append-only JSONL, ULID identity,
relations as their own records, validity derived from edges rather than stored,
and a retrieval contract that reports a match count instead of dumping two
hundred records into a context window.

Three scopes — plan, repo, user — all gitignored, with promotion between them
always an explicit act. These are files the assistant writes for itself; the
real work lands in the code, and committing them only adds noise to pull
requests.

## Setup

```bash
cd tools/memory && go build -o ../../bin/memory ./cmd/memory
```

The skills build it on first use if it is missing, so this is optional.

Requires Go 1.26+. No other dependencies.
