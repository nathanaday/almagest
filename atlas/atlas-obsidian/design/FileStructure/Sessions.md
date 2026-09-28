
# Sessions

Every agent session has a document in `sessions/`. It is the one page that says what the session works on, its status now, and what it touched. It is not a transcript.

Hooks create the document and keep every field of it, so the record does not depend on the model. The agent writes three short sections of its own document with Edit: a description, its progress, and a summary.

`Sessions.base` answers "what is running now" and "what have we run". The Obsidian plugin's sidebar reads the same documents ([[Obsidian Plugin]]).

## Names and keys

| Session | File | Key the hooks look it up by |
|---|---|---|
| a main session | `sessions/2026-09/2026-09-27 1432 a1b2c3.md`: the start date and time, and the first six hex characters of `session_id` | `session_id` |
| a subagent that may write (any agent type but the four read-only [[Agents]]) | `sessions/2026-09/2026-09-27 1432 a1b2c3 · 9f07d1.md`: the parent's name, and the first six hex characters of `agent_id` | `agent_id` |
| a read-only worker | no document; one line under `## Subagents` in its parent's document | — |

A hook event from a subagent carries both `session_id` (the parent's) and `agent_id`. The hooks use `agent_id` when it is present.

The id of a session document is `ses-` and the same six hex characters. It is the one id that code does not mint in base32 ([[Document Types#Fields every document has]]).

## The session document

```yaml
---
id: ses-a1b2c3
type: session
created: 2026-09-27
updated: 2026-09-27T15:02
# every field is the hooks':
harness: claude                     # claude | codex
harness_id: 7f3e…                   # the session_id, or the agent_id of a subagent
status: running                     # running | waiting | idle | ended | lost
started: 2026-09-27T14:32
ended: ""
last_prompt: 2026-09-27T14:58       # the last UserPromptSubmit; the gate on change apply reads it
cwd: "~/notes/work"
parent: ""                          # the parent session, for a subagent
agent: ""                           # the subagent's type, for a subagent
threads: ["[[Filter vehicle false alarms]]"]      # every thread the session worked on
tasks: ["[[Filter vehicle false alarms — T1 Score boxes by motion]]"]   # every task it started
repositories: ["[[p3-edge]]"]       # every repository the session edited
changes: ["[[2026-09-27 Ingest the DINOv2 paper]]"]
description: "Score detection boxes by motion to cut vehicle false alarms"   # copied from ## Description
---
```

Body:

1. The lead callout, the hooks': `> [!session] running · T1 of [[Filter vehicle false alarms]] · [[p3-edge]]`. It names the task started last that is still open, and quotes that task's last progress line.
2. `## Description`: one line on what this session works on. The agent's.
3. `## Progress`: dated lines, for work that is not a task. Work on a task writes its progress in the task document only. The agent's.
4. `## Summary`: what the session did, at its end. The agent's.
5. `## Subagents`: one line per read-only worker: its type, what it was sent to do, and when it ended. The hooks'.

The agent edits only its own document, and only sections 2 to 4. The guard refuses an Edit to another session's document, and an Edit that touches the frontmatter, the lead callout, or `## Subagents` ([[Hooks#guard]]). After such an Edit, the `touched` hook copies the first line of `## Description` into `description`, so Bases can show it.

## Status

| Status | Set when |
|---|---|
| `running` | `SessionStart`, `SubagentStart`, `UserPromptSubmit`, and any tool call |
| `waiting` | `Notification` of type `permission_prompt`, `elicitation_dialog`, or `agent_needs_input`: the session waits for your answer |
| `idle` | `Stop`, or `Notification` of type `idle_prompt`: the turn ended and the session is open |
| `ended` | `SessionEnd`, `SubagentStop` |
| `lost` | the session-start hook of any session finds a `running`, `waiting`, or `idle` session with no hook event for `stale_hours` |

`updated` is the time of the last hook event, written at most once a minute. An `idle` session still holds its threads and tasks. Only `ended` and `lost` let them go.

## Links

Hooks own every link between a session and another document. The MCP server never learns a session id, and it does not need to, because the `PostToolUse` hook sees each call with its input and its result.

| Field | Set by the hook when |
|---|---|
| `threads` | a `thread` call that acts on one thread: `open`, `attach`, `file`, `tasks`, `task`. A read (`list`, `show`) binds nothing |
| `tasks` | `thread` task `start` |
| `repositories` | an Edit or Write lands inside a linked repository |
| `changes` | a `change` call proposes a change. The hook also writes `session` into the change document, so the field is in place before apply commits it ([[Changes#Apply]]) |
| `parent`, `agent` | `SubagentStart`, from `parentSessionId` and `agent_type` |
| `last_prompt` | `UserPromptSubmit` |

A subagent that may write starts with its parent's `threads` and `tasks`, so the thread rule holds inside it.

## The Stop reminder

The `Stop` hook reminds the agent once, when the session edited a repository or proposed a change and `## Description` is empty.

## Sessions.base

Views: running, waiting, and idle now; today; by thread; by repository; lost. Each row shows the description, the status, the threads, and the tasks.
