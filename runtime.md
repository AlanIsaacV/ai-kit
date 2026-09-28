# Host runtime

Read this once when entering an ai-kit skill or agent. It adapts the shared
workflow to the current host; the host's system and developer instructions,
tool availability, permission rules and the user's existing authorization
remain authoritative.

## Paths and memory

Keep the **ai-kit clone** separate from the **target repository**. Resolve the
installed `SKILL.md` to its physical file, following directory symlinks. Its
location is `<ai-kit clone>/skills/<skill>/SKILL.md`; the clone root is two
directories above that skill directory. The entrypoint's
`references/runtime.md` links to this file. Agent wrappers also provide the
absolute path to this file and their contract.

Resolve references to `agents/<role>.md`, sibling `skills/` and ai-kit's own
`tools/memory/` documentation against that clone root, never against the target
repository. Each skill's `agents/openai.yaml` is display metadata, not a role
contract. Resolve project source paths, `.ai-kit/` artifacts and `repo_root`
against the target repository identified by the user or current workspace.

Before the first memory operation, check `command -v memory` and
`memory --help`. Run memory commands with the target repository as the working
directory, including when the task concerns another repository. Never switch
to the ai-kit clone to query or write that other repository's log. Pass the
target repository's absolute `repo_root` to every delegated role.

If memory is missing or broken, the installation repair is `make install` from
the ai-kit clone. The default command link is `$HOME/.local/bin/memory`; ensure
`$HOME/.local/bin` is on the shell's `PATH`, then repeat the preflight from the
target repository. Perform installation only within the task's authorization
and the host's filesystem permissions. When setup cannot run, report the
missing prerequisite instead of inventing log output. Keep subsequent workflow
commands spelled `memory`; resolve installation and PATH here once.

## Skill transitions and questions

| Shared instruction | Claude Code | Codex |
| --- | --- | --- |
| Invoke the `Skill` tool for a skill | Use the installed skill name, normally `ai-kit:<skill>`. | Read the installed target skill's `SKILL.md` and follow it in this conversation; use its advertised name when a native invocation tool exists. |
| Ask through `AskUserQuestion` | Use that tool when available and permitted. | Use an available native question tool only when its mode and question rules allow it; otherwise ask the user in plain text. |
| Launch a named subagent | Use the installed plugin role, normally `ai-kit:<role>`. | Use the installed native agent `ai-kit-<role>` through the host's delegation tool. |

A skill transition stays in the orchestrator's conversation; reading the next
skill does not mean delegating its planning or user discussion. For missing
skill invocation tools, resolve `<ai-kit clone>/skills/<skill>/SKILL.md` and
read it directly. Keep advertised skill names and role identifiers distinct.

Batch questions within the native tool's actual capacity. A question tool that
is unavailable in the current mode is not a reason to change modes. Approval
and escalation use the host's required mechanism; a general question tool does
not grant execution permission. Preserve decisions and authorization already
given in this conversation, subject to the host's rules.

## Delegated roles

The shared roles are `explore`, `implement`, `verify`, `review`, `evaluate` and
`record`. Their authoritative contracts are `<ai-kit clone>/agents/<role>.md`.
The installer supplies Codex with native TOML definitions that load this
runtime and those contracts; it does not translate Claude YAML into Codex
configuration.

When the available delegation tool cannot select a named role, read the
authoritative contract and pass its Markdown body, this runtime's host mapping
and the required task inputs to a fresh subagent through that tool. Retain the
contract's scope, restrictions and return format. This fallback adapts role
selection, not workflow responsibilities: verification still needs a separate
agent from implementation. If delegation itself is unavailable, report that
limitation instead of claiming to have launched a role.

Claude agent frontmatter such as `tools: Read, Edit, Write` belongs to Claude
Code; the agents declare no model, so Claude Code runs them on the session's
model. On Codex use the tools actually exposed by the host and inherit the
current Codex model defaults; Claude tool names are not Codex identifiers. The
contract body supplies task instructions, not system authority or extra tool
permissions.

Native Codex execution uses its native `implement` role. The optional Codex
companion subsection in `work` applies only to a Claude Code orchestrator
intentionally launching that external executor; it is not a dependency of
running ai-kit inside Codex.
