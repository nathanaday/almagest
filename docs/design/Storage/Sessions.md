# Sessions

Every agent session has a document in `sessions/`. It is the one page that says what the session works on, its status now, and what it touched. It is not a transcript.

Sessions are operational records, so they live outside `wiki/documents/` with a schema of their own. The durable history of work is the events ([[Event]]); each event links the session that caused it.

Hooks create the document and keep every field, so the record does not depend on the model. The agent writes three short sections of its own document with Edit: a description, its progress, and a summary.

`Sessions.base` answers "what is running now" and "what have we run". The Obsidian plugin's sessions pane reads the same documents ([[Obsidian Plugin#Sessions pane]]).

## Names and keys

| Session | File | Key the hooks look it up by |
|---|---|---|
| a main session | `sessions/2026-09/2026-09-29 1432 a1b2c3.md`: the start date and time, and the first six hex characters of `session_id` | `session_id` |
| a subagent that may write (any agent type but the four read-only [[Agents]]) | `sessions/2026-09/2026-09-29 1432 a1b2c3 · 9f07d1.md`: the parent's name, and the first six hex characters of `agent_id` | `agent_id` |
| a read-only worker | no document; one line under `## Subagents` in its parent's document | — |

A hook event from a subagent carries both `session_id` (the parent's) and `agent_id`. The hooks use `agent_id` when it is present.

The id of a session document is `ses-` and the same six hex characters.

## The session document

```yaml
---
id: ses-a1b2c3
type: session
created: 2026-09-29T14:32:05
updated: 2026-09-29T15:02:40
# every field is the hooks':
harness: claude                     # claude | codex
harness_id: 7f3e…                   # the session_id, or the agent_id of a subagent
status: running                     # running | waiting | idle | ended | lost
started: 2026-09-29T14:32:05
ended: ""
last_prompt: 2026-09-29T14:58:12    # the last real user prompt; the change gate reads it
cwd: "~/notes/work"
parent: ""                          # the parent session, for a subagent
agent: ""                           # the subagent's type, for a subagent
specs: ["[[Score boxes by motion]]"]    # every plan the session started; the guard reads it
work: ["[[Filter vehicle false alarms]]", "[[Try a new tracker]]"]   # every stub or spec a work call wrote
repositories: ["[[p3-edge]]"]       # every repository the session edited
changes: ["[[2026-09-29 Ingest the DINOv2 paper]]"]
events: 3                           # how many events its calls wrote
description: "Score detection boxes by motion to cut vehicle false alarms"   # copied from ## Description
---
```

Body:

1. The lead callout, the hooks': `> [!session] running · [[Score boxes by motion]] · [[p3-edge]]`. It names the plan started last that is still started, and quotes that plan's last `## Progress` line.
2. `## Description`: one line on what this session works on. The agent's.
3. `## Progress`: dated lines, for work that is not a plan. Work on a plan writes its progress in the spec only. The agent's.
4. `## Summary`: what the session did, at its end. The agent's.
5. `## Subagents`: one line per read-only worker: its type, what it was sent to do, and when it ended. The hooks'.

The agent edits only its own document, and only sections 2 to 4. The guard refuses an Edit to another session's document, and an Edit that touches the frontmatter, the lead callout, or `## Subagents` ([[Hooks#guard]]). Every hook event copies the first line of `## Description` into `description`, so Bases can show it.

## Status

| Status | Set when |
|---|---|
| `running` | `SessionStart`, `SubagentStart`, a real `UserPromptSubmit`, and any tool call |
| `waiting` | `Notification` of type `permission_prompt`, `elicitation_dialog`, or `agent_needs_input`: the session waits for your answer |
| `idle` | `Stop`, or `Notification` of type `idle_prompt`: the turn ended and the session is open |
| `ended` | `SessionEnd`, `SubagentStop` |
| `lost` | the session-start hook of any session finds a `running`, `waiting`, or `idle` session with no hook event for `stale_hours` |

`updated` is the time of the last hook event, written at most once a minute. A **live** session is `running`, `waiting`, or `idle`. A live session still holds the plans it started. Only `ended` and `lost` let them go.

## Links

Hooks own every link between a session and another document. The MCP server never learns a session id, because the `PostToolUse` hook sees each call with its input and its result.

| Field | Set by the hook when |
|---|---|
| `specs` | `work start` binds the session to the plan |
| `work` | any `work` call that writes a stub or a spec |
| `events` | a `work` call or a `change` apply writes events; the hook also writes `session` into each of them |
| `repositories` | an Edit or Write lands inside a linked repository, or a Bash command with a write mark runs in one or names one |
| `changes` | `change` propose; the hook also writes `session` into the change document, so the gate can read it ([[Changes#The gate]]) |
| `parent`, `agent` | `SubagentStart`, from the parent's id and `agent_type` |
| `last_prompt` | a `UserPromptSubmit` that is the user's turn, not a subagent's hand-back or a task notice |

A subagent that may write starts with its parent's `specs`, so the edit rule holds inside it.

An event's `session` field is written by the hook after the `work` call returns, under the lock, before the next commit. A `work` call from the CLI or the Obsidian plugin has no hook, so its events have no session and `by: user`.

## The Stop hook

The `Stop` hook sets `status: idle`. It blocks the stop once per session, with the reason, for the agent's own debts:

- the session edited a repository and its `## Description` is empty;
- the session started a plan and wrote no `## Progress` line in it this session.

A change that waits for the user is a `systemMessage`, because the user acts on it.

## Sessions.base

Views: live now; today; by spec; by repository; lost. Each row shows the description, the status, the specs, and the last event.
