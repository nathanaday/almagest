
# Sessions

Every agent session has a document in `sessions/`. It is the front page of the session: what it works on, its status now, and what it touched. It is not a transcript.

Hooks create and keep the document, so the record does not depend on the model remembering to write it. The agent adds three pieces of prose through the `session` tool: a description, its progress, and a summary.

`Sessions.base` answers "what is running now" and "what have we run". The Obsidian plugin's sidebar reads the same documents ([[Obsidian Plugin]]).

## The session document

`sessions/2026-09/2026-09-27 1432 a1b2c3.md`: the start date and time, and the first six characters of the harness's session id.

```yaml
---
id: ses-a1b2c3
type: session
created: 2026-09-27
updated: 2026-09-27T15:02
# owned by the hooks:
harness: claude                     # claude | codex
harness_id: 7f3e…                   # the session_id, or the agent_id of a subagent
status: running                     # running | waiting | idle | ended | lost
started: 2026-09-27T14:32
ended: ""
cwd: "~/notes/work"
parent: ""                          # the parent session, for a subagent
agent: ""                           # the subagent's type, for a subagent
thread: "[[Filter vehicle false alarms]]"
task: "[[Filter vehicle false alarms — T1 Score boxes by motion]]"
repositories: ["[[p3-edge]]"]       # every repository the session edited
changes: ["[[2026-09-27 Ingest the DINOv2 paper]]"]
# written through the session tool:
description: "Score detection boxes by motion to cut vehicle false alarms"
---
```

Body:

1. The lead callout, owned by the hooks: `> [!session] running · T1 of [[Filter vehicle false alarms]] · [[p3-edge]]`.
2. `## Progress`: dated lines. `session` progress appends one.
3. `## Summary`: what the session did, written at its end.

## Status

| Status | Set when |
|---|---|
| `running` | `SessionStart`, `SubagentStart`, `UserPromptSubmit`, and any tool call |
| `waiting` | `Notification` of type `permission_prompt`, `idle_prompt`, or `agent_needs_input`: the session waits for you |
| `idle` | `Stop`: the turn ended; the session is open |
| `ended` | `SessionEnd`, `SubagentStop` |
| `lost` | the session-start hook of any session finds a `running`, `waiting`, or `idle` session with no hook event for `stale_hours` |

`updated` is the time of the last hook event.

## Links

Hooks own every link between a session and another document, keyed by the harness's session id, and for a subagent by its `agent_id`. The MCP server never learns a session id. It does not need to, because the `PostToolUse` hook sees each call with its input and its result.

| Field | Set by the hook when |
|---|---|
| `thread` | a `thread` call that acts on one thread: `open`, `attach`, `file`, `tasks`, `task`. A read (`list`, `show`) binds nothing |
| `task` | `thread` task `start`; cleared by `done` or `drop` on that task |
| `repositories` | an Edit or Write lands inside a linked repository |
| `changes` | a `change` call proposes or applies a change; the hook also writes `session` into the change document |
| `parent`, `agent` | `SubagentStart`, from `parentSessionId` and `agent_type` |

A subagent starts with its parent's `thread` and `task`, so the thread rule holds inside it.

## Subagents

Each subagent gets its own session document, linked to its parent. The parent document lists its children through backlinks. A read-only worker ([[Agents]]) gets one too: it costs one small file, and it shows you what the orchestrator sent out.

## What the agent writes

| Call | Writes | When the skill makes it |
|---|---|---|
| `session` describe | `description` | once the agent knows the work, in the first turn |
| `session` progress | a line under `## Progress` | at each stopping point |
| `session` summary | `## Summary` | at the end of the work |

The `Stop` hook reminds the agent once when the session edited a repository or proposed a change and has no description.

## Sessions.base

Views: running and waiting now; today; by thread; by repository; lost. Each row shows the description, the status, the thread, and the task.
