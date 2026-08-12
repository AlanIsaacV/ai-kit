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
skills/debug   turn a symptom into a stated cause, with the evidence that established it
skills/work    run one objective: pick, gate, implement, verify, review, record
skills/brief   think a piece of work through, then hand you the map you implement from
skills/debrief read the code you wrote and say what you missed and what would be better
skills/ask     answer a question about a repo from its code and its log, every claim cited
skills/bootstrap  give an unseen repo a README.md and a CLAUDE.md in one invocation
skills/prefer  record why this repo chose what it chose, and carry it to the next one
agents/        explore · implement · verify · review · evaluate · record
tools/memory   the append-only knowledge log everything writes to
```

`plan` runs once per objective, including each one opened underneath another.
`work` runs one objective per turn and is resumable in a fresh session. Small
work takes the same path — one turn of the loop instead of many — so size is
never what picks a door and there is no escalation path to get wrong. What does
pick one is who writes the code: when that is you rather than the agent, `brief`
is the other door — a design conversation that ends in a map you implement from,
with nothing in it for `work` to run.

`debug` is the second front-end onto the same vocabulary, not a second loop. It
runs when what arrived is a symptom rather than a request: it narrows the cause
one hypothesis at a time, writes every hypothesis it tests to the log with the
fidelity of the evidence behind it, never touches the repository it is
investigating, and hands its finding to `plan` — which sizes the resulting work
and writes the objective, exactly as it would from any other starting point.

`ask` is not part of the loop at all: it changes nothing and only a human invokes
it. It answers a question about a repository from the two places an answer can be
checked — mechanism read off the code, rationale read out of the log — in one
narrative where every claim carries a `path:line` or a record id and date. The
answer is ephemeral unless somebody says to keep it, and what gets kept is
anchored, so the next ask over the same ground checks those anchors instead of
reading everything again.

## How the drafts work

Both skills negotiate through a markdown file rather than through chat. Every
objective is written to `.ai-kit/plans/<objective-slug>/` — one directory per
objective, beside the log and inside the directory that keeps itself out of git —
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
make install
```

That builds the log CLI to `bin/memory` and then makes symlinks, all of them
pointing back at the clone it was run from:

```
~/.local/bin/memory              ->  <clone>/bin/memory
~/.claude/skills/ai-kit          ->  <clone>
~/.agents/skills/ai-kit-<name>   ->  <clone>/skills/<name>
```

The first is what lets the skills call `memory` by name instead of carrying a
path to the binary around. The second is what makes Claude Code load this
repository as a skills plugin. The third is one link per directory under
`skills/`, which is how Codex finds them — one at a time rather than as a plugin,
so a skill added to `skills/` gets its own link the next time this runs, while a
skill renamed or removed leaves its old link behind, which no re-run clears and
you delete by hand. Codex reads that directory when a session starts, so the
skills appear in the next session you open and not in one already running.
Nothing else on the machine is touched — no shell profile, no configuration
file — which is also why `~/.local/bin` has to already be on your `PATH` for the
name to resolve; `make install` says so when it is not, and leaves fixing it to
you.

Running it again rebuilds the binary and replaces every link with the same link,
so repeating it costs nothing and it is what to run after changing
`tools/memory/`.

Deleting those symlinks undoes the install, the per-skill ones included; there is
no `make uninstall`, because those paths are the whole footprint and a target for
removing them would only be a second place to keep them written down.

Installing requires Go 1.26+ and nothing else.

## The check

```bash
make check
```

`scripts/check-frontmatter` walks the tree, finds every file that opens with YAML
frontmatter — the eight `skills/*/SKILL.md` and the six `agents/*.md` today, and
whatever is added later, since the set is read off the tree rather than listed
here — and refuses any whose `description` spans more than one physical line. It
names the path and the line and exits non-zero. It repairs nothing, and nothing
runs it for you: no hook, no CI.

That one shape is worth a target because it breaks silently. A description
carrying a raw line break is still valid YAML, and the two hosts disagree about
it: Codex folds the break into a space and lists the skill, Claude Code drops the
skill from its enumeration while leaving it invocable by name. Nothing errors, so
short of reading a live session's skill list there is no witness — which is how
one wrapped line survived four commits, two installs and a verification pass.

It needs Python 3 and nothing else — no Go build, so it runs on a clone that was
never installed.
