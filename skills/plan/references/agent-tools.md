# Agent tools — answering a call and recording it

What a tool must do when an agent calls it, and what it must leave behind, so that
whether it works can be measured instead of guessed. Read by `plan` when an
objective builds or changes a CLI or an MCP tool whose caller is an agent; its
checklist becomes that objective's criteria. The premise: **a tool consumed by
agents is only known to be broken when an agent says so, and agents mostly do
not.** They get an empty or wrong answer at exit 0, conclude there is nothing
there, and move on — or they route around the tool by reading its store directly.
An error count sees neither.

## Why a classified record, and not an error count

The failure this exists for is the call the tool misread and answered anyway.
`memory search --dir X "detector" --fields id` answered `count: 0` at exit 0: every
flag after the term had been folded into the search text, and the same search
written with its flags first finds records. Across both transcript roots a large
share of `memory search` invocations put a flag after the term, and every one of
them searched for text the agent never wrote, at exit 0. No error was raised, so no
error count moved, and no agent reported it, because an empty answer is a
plausible answer.

Two things close that gap, and they are this file's two halves. The answer has to
make a misunderstanding impossible to mistake for an empty result: refuse what was
not understood, echo what was, say when the answer was cut short. And every call
has to leave one record whose outcome separates "found nothing", "cut short" and
"did not understand" from "ok", with the tool's version and a way back to the
session that made it. With both, a later report can count silent failures per tool
and say whether a change made them rarer. Without the echo, an empty record is
still ambiguous; without the record, a good answer shape is still unmeasured.

## What it covers

Any CLI or MCP tool whose caller is an agent. They come in two families, and the
family decides where the record is written, not what it says:

- **A CLI the agent runs directly**, through its shell. It writes its own call log.
- **An MCP tool behind cerberus-gw.** The gateway already writes one audit row per
  call; that row is the record, and the backend's part is to answer in a shape the
  gateway can classify.

A third-party server behind the gateway is covered by the gateway's record alone
and is not modified; what it answers is classified as far as its shape allows.

## The answer rules

Five rules, each about one way an answer can let a misunderstanding pass.

1. **What was not understood answers `invalid`.** An unknown flag, a misplaced
   flag, a malformed value, an unknown argument, a missing required one: the call
   fails with a non-zero exit on a CLI or `IsError` on an MCP tool, and the answer
   is machine-readable JSON, `{error, code}`, where `code` is a stable word an
   agent can branch on and `error` says what to do instead. Parse errors are
   included. Usage text on stderr with no JSON is the case this rule removes: an
   agent that parses stdout reads it as nothing at all.
2. **A flag after a positional argument is rejected, not taken as text.** It
   answers `invalid` under rule 1 and names the flag it found. Text that really
   does begin with a dash goes after `--`, which ends the flags. This is the rule
   the motivating failure breaks, and it is stated apart from rule 1 because a
   parser that stops at the first positional, as Go's `flag` package does, breaks it
   without ever raising an error.
3. **Every answer that filters echoes what it interpreted, in an `interpreted`
   object.** The search text, every filter, the limit and the scope, as the tool
   parsed them and including the defaults it applied — a default is often exactly
   what the agent did not know it was getting. The echo is what lets the agent, and
   later the report, see that the question the tool answered is not the one that
   was asked.
4. **An empty result is `count: 0` beside that echo, and is a success.** Exit 0, no
   `IsError`. Once the echo shows what was asked, nothing found is a real answer,
   and it must never be the shape a failure takes.
5. **A truncated result says how many were withheld and how to get the rest.** It
   carries `count`, the total the tool found, and `returned`, how many it gave back,
   so the withheld number is their difference; and either a `hint` naming the flag
   or argument that gets the rest, or a continuation token. When the total cannot be
   known cheaply, as with a paginated upstream, `count` is null and the answer says
   that it does not know rather than guessing.

A change to an answer's shape can break the agents and skills that parse it today.
Adding members does not; renaming, moving or removing one does, and a tool
objective that does it names the break and who consumes the shape.

## The six outcomes

Every call ends in exactly one of these. A call passes through being understood,
being allowed, running, and returning; the first stage that stops it decides the
outcome, and only a call that got through all four is `ok`, `empty` or `truncated`.

| Outcome | Meaning |
|---|---|
| `invalid` | Not understood: rule 1's cases, rule 2's flag after a positional, a subcommand or tool that does not exist. |
| `refused` | Understood but not allowed: a missing permission, a ticket required and absent, a statement a policy rejects. |
| `error` | Understood and allowed, and it failed: a backend down, a timeout, a crash, the tool's own bug. |
| `truncated` | It ran and found more than it returned: `returned` below `count`, including the case that withholds every result until a limit is given. |
| `empty` | It ran and found nothing: `count` is 0. |
| `ok` | It ran and returned everything it found, at least one result; or the operation returns no collection, such as a write, and it succeeded. |

`invalid` and `refused` are kept apart on purpose. One says the caller has to ask
differently, the other that the caller has to obtain something; counted together,
a tool whose parser misreads half its calls looks like a tool with strict access
rules.

## The call record

One record per call, every call, including the ones that ended `invalid` — those
are the reason it exists.

| Field | What it holds |
|---|---|
| `at` | When the call arrived, RFC 3339 in UTC. |
| `tool` | The tool's name as `agent-tool.json` declares it. |
| `version` | The version the tool reports about itself: the revision its binary was built from, which a Go build embeds as `vcs.revision`. The CLI prints the same string on `--version`, so a record can be matched to code. |
| `session` | The session that made the call, or empty when it is not known. Never invented. |
| `operation` | The subcommand or MCP tool called; as typed when it was not recognised. |
| `interpreted` | The same object the answer echoed under rule 3, or null when the call did not get far enough to have one or the operation does not filter. |
| `outcome` | One of the six. |
| `outcome_detail` | The finer word under it: the `code` of an `invalid`, `refused` or `error` answer, or the gateway's own outcome. Empty when there is nothing finer to say. |
| `count` | The total found, before any truncation; null when the operation returns no collection or the total is unknown. |
| `returned` | How many results the answer carried; null where `count` is. |
| `latency_ms` | Wall time from arrival to answer, an integer. |

The record carries counts, never result content. `interpreted` holds what the
caller sent, as parsed; it keeps no more personal data than the tool's existing
audit already keeps, and behind the gateway it follows the rules the gateway's
`arguments` column follows. The standard adds to what a tool already logs and never
requires removing an existing log: a slog line or an audit stream stays where it
is. Rotating and retaining a CLI's call log are outside it.

## Where each family writes it

### A CLI

Appends one JSON line per call to `~/.ai-kit/calls/<tool>.jsonl`, one file per
tool, creating the directory when it is missing. It takes `session` from
`CLAUDE_CODE_SESSION_ID`. On this host on 2026-11-07, the variable was set in the
environment of a Bash call from a main thread and of one from a subagent, and its
value equalled the transcript's filename and its `sessionId`. That was observed,
not documented: Claude Code does not document the variable, so a tool objective
confirms it before relying on it. When the variable is absent — Codex, a person at
a shell — `session` stays empty.

Writing the record is best-effort. A failure to write never fails the call, never
changes what it printed and never changes its exit code: a log that can break the
tool it observes would be removed the first time it did.

### An MCP tool behind cerberus-gw

The gateway's audit row is the record, and no backend builds a call log of its own.
The gateway keeps doing what it does when its audit write fails, which is to
withhold the result: there the audit is a control, not only an observation.

What the gateway cannot do today is classify a result it cannot read, so the
backend answers in an **envelope**: a JSON result whose top-level members include
`count`, `returned`, `truncated` and `interpreted`, the same members the answer
rules already require. The gateway reads them into the row; a result without them
stays `ok`, with `count` null and `no_envelope` as its `outcome_detail`, so the
share of calls nobody can classify is itself countable.

The gateway has no `invalid` today, and only the backend knows that it did not
understand a call. A backend signals it with an `IsError` result whose JSON body is
`{error, code, outcome}`, with `outcome` set to `invalid`; a refusal of the
backend's own, such as a statement its policy rejects, sets it to `refused`. With
no `outcome` member, an `IsError` result is an `error`.

The session is joined, not recorded. The gateway already returns its row id to the
caller as a `trace:` line and under `_meta`, and both land in the transcript; the
row is tied to its session by finding that id in a transcript, so the row needs no
session column.

The gateway's current outcomes map to the six like this, keeping the original as
`outcome_detail` rather than renaming what is stored:

| Gateway outcome | Outcome | `outcome_detail` |
|---|---|---|
| `ok` | `truncated` when the envelope says `truncated` or `returned` is below `count`; else `empty` when `count` is 0; else `ok` | empty, or `no_envelope` |
| `tool_error` | the body's `outcome` when it is `invalid` or `refused`; else `error` | `tool_error` |
| `backend_error` | `error` | `backend_error` |
| `unavailable` | `error` | `unavailable` |
| `refused` | `refused` | `refused` |

The row's other fields map the same way: `at` is its `at`, `tool` its backend and
tool, `operation` its tool, `latency_ms` its duration, and `version` the gateway's
and the backend's versions, which the row will have to start carrying.

## The declaration — `agent-tool.json`

Every tool repository commits an `agent-tool.json` at its root, listing the tools
the repository provides. It is where a report finds, for each tool, where its
records are, what routing around it looks like, and what number says it works. One
file per repository, because a repository is what gets changed and deployed
together, and two tools in one repository share a release.

Each tool in the list carries:

- **`name`**, the same string its records carry as `tool`.
- **`records`**, where its call records live: `{"kind": "jsonl", "path": ...}` for a
  CLI, or `{"kind": "cerberus-gw", "backend": ...}` for a tool behind the gateway.
- **`workarounds`**, what routing around it looks like in a transcript: `commands`,
  regular expressions matched against the command of a Bash tool call, and `paths`,
  globs matched against the paths a file-reading tool call was given. A direct read
  of the tool's store is the usual case — the shell reading its log file, a SQL
  client opened on its database. A signature is a claim to be tested on transcripts
  before it is declared, because one that also matches the tool's legitimate use
  counts every call as a workaround.
- **`metric`**, the number that says it works: a `name`; a `computation` giving its
  `source`, `records` or `transcripts`, and the `numerator` and `denominator` of a
  ratio over that source; and `better`, `lower` or `higher`. A ratio and not a
  count, because a count moves with how much the tool is used.
- **`baseline`**, the metric measured before the change: its `value` and the
  `numerator` and `denominator` counts it came from, the `date` it was taken, the
  `window` of calls or transcripts it covers, the transcript `roots` it read, and
  `dedup`, the field sessions were deduplicated by.

The roots are always both `~/.claude/projects` and `~/.claude-p/projects`. Earlier
measurement on this host read only the first while current sessions live in the
second, and a baseline over one root says nothing about the other. Sessions are
deduplicated by `sessionId`, so one that appears under both roots counts once.

A worked example, with placeholders where a real declaration carries measured
values — `value` and the counts are numbers there, and the dates are real dates:

```json
{
  "tools": [
    {
      "name": "notes",
      "records": { "kind": "jsonl", "path": "~/.ai-kit/calls/notes.jsonl" },
      "workarounds": {
        "commands": ["\\bnotes\\.jsonl\\b", "\\bsqlite3\\b.*\\bnotes\\.db\\b"],
        "paths": ["**/.notes/notes.jsonl", "**/.notes/notes.db"]
      },
      "metric": {
        "name": "silent_failure_rate",
        "computation": {
          "source": "transcripts",
          "numerator": "notes search calls with a flag after the search term",
          "denominator": "notes search calls"
        },
        "better": "lower"
      },
      "baseline": {
        "value": "<ratio>",
        "numerator": "<calls counted>",
        "denominator": "<calls read>",
        "date": "<YYYY-MM-DD>",
        "window": { "from": "<YYYY-MM-DD>", "to": "<YYYY-MM-DD>" },
        "roots": ["~/.claude/projects", "~/.claude-p/projects"],
        "dedup": "sessionId"
      }
    },
    {
      "name": "notes-mcp",
      "records": { "kind": "cerberus-gw", "backend": "notes" },
      "workarounds": {
        "commands": ["\\bpsql\\b.*\\bnotes\\b"],
        "paths": []
      },
      "metric": {
        "name": "invalid_share",
        "computation": {
          "source": "records",
          "numerator": "calls with outcome invalid",
          "denominator": "all calls"
        },
        "better": "lower"
      },
      "baseline": {
        "value": "<ratio>",
        "numerator": "<calls counted>",
        "denominator": "<calls read>",
        "date": "<YYYY-MM-DD>",
        "window": { "from": "<YYYY-MM-DD>", "to": "<YYYY-MM-DD>" },
        "roots": ["~/.claude/projects", "~/.claude-p/projects"],
        "dedup": "sessionId"
      }
    }
  ]
}
```

## Before and after

A change to a tool is only known to have helped if the metric was taken before it,
by the same computation that will be used after. That decides what the metric can
be: a CLI that keeps no records before its change cannot have a records-sourced
baseline, so its metric has to be computable from transcripts, or the after will be
measured on a different instrument from the before and the comparison says
nothing.

The baseline is measured while the tool objective is planned, before any of its
code changes, and goes into that objective's `## Premise`, on the falsifier line,
as what the observation returned; the declaration carries the same numbers. The
after is not a criterion. It needs a window of real use that no verification pass
has, so the objective leaves it as an open question naming the metric and the
baseline, for the friction report to close.

## Checklist

The items become the tool objective's acceptance criteria. Each is written so a
verifier can check it against the tool or against the objective's draft, and so it
can be lifted as it stands, with the tool's own names put in. The section it comes
from is in brackets.

- [ ] A call the tool did not understand — an unknown or misplaced flag, a malformed
  value, an unknown argument, a parse error — exits non-zero or answers `IsError`,
  with `{error, code}` as JSON. [The answer rules, 1]
- [ ] A flag placed after a positional argument answers `invalid`, naming the flag,
  and is never taken into the text; text that begins with a dash is accepted after
  `--`. [The answer rules, 2]
- [ ] Every answer from an operation that filters carries an `interpreted` object
  with every filter as parsed, defaults included. [The answer rules, 3]
- [ ] An empty result answers `count: 0` beside the echo, as a success. [The answer
  rules, 4]
- [ ] A truncated result carries `count`, `returned`, and a `hint` or continuation
  that gets the rest; `count` is null only when the total cannot be known cheaply,
  and the answer then says it does not know. [The answer rules, 5]
- [ ] Every call, including an `invalid` one, leaves exactly one record with `at`,
  `tool`, `version`, `session`, `operation`, `interpreted`, `outcome`,
  `outcome_detail`, `count`, `returned` and `latency_ms`. [The call record]
- [ ] Each record's `outcome` is one of `ok | empty | truncated | invalid | refused |
  error`, decided by the first stage that stopped the call, with the answer's
  `code` as `outcome_detail`. [The six outcomes]
- [ ] The tool reports its version, and the record's `version` is the same string.
  [The call record]
- [ ] A CLI appends its record to `~/.ai-kit/calls/<tool>.jsonl`, takes `session`
  from `CLAUDE_CODE_SESSION_ID`, once confirmed to carry the session, and leaves it
  empty when absent, and a failed write changes neither its output nor its exit
  code. [Where each family writes it, A CLI]
- [ ] A tool behind cerberus-gw answers in the envelope — `count`, `returned`,
  `truncated`, `interpreted` — and signals `invalid` or `refused` through the
  `outcome` member of an `IsError` body; the gateway classifies its row by the
  mapping table and keeps the original outcome as `outcome_detail`. [Where each
  family writes it, behind cerberus-gw]
- [ ] No record keeps result content or more personal data than the tool's existing
  audit, and no existing log is removed. [The call record]
- [ ] The repository's root carries `agent-tool.json` listing the tool with
  `records`, `workarounds`, `metric` and `baseline`, each complete. [The declaration]
- [ ] The baseline is measured before the change, by the computation the after
  will use, over both transcript roots deduplicated by `sessionId`, and its value,
  date and window are in the objective's `## Premise`. [Before and after]
- [ ] The objective's draft carries an open question for the after-measurement,
  naming the metric and the baseline. [Before and after]
- [ ] Any change to an existing answer shape that renames, moves or removes a member
  is named in the objective's draft with the consumers it breaks. [The answer rules]
