
# Hooks

A hook is code the harness runs on an event. Atlas uses hooks for two jobs that a skill cannot do reliably:

1. **Rules that must hold.** The guard refuses a write that breaks a rule, whether or not the model read the skill.
2. **Facts about sessions.** Hooks create and keep every session document and every link between a session and another document. The model does not need to remember.

Every hook is one command, `atlas hook <event>`, which reads the event's JSON on stdin. A hook that finds no vault for the session exits at once and does nothing, so Atlas stays out of sessions that are not its own.

## The events

| Event | Command | Does |
|---|---|---|
| `SessionStart` | `session-start` | creates or resumes the session document; runs `vault sync`; prints the opening context |
| `UserPromptSubmit` | `prompt` | `status: running`; `updated` |
| `PreToolUse` on write tools and Bash | `guard` | refuses a write that breaks a rule |
| `PostToolUse` | `touched` | links the session to what the call touched; writes Session Notes |
| `Notification` (`permission_prompt`, `idle_prompt`, `agent_needs_input`) | `waiting` | `status: waiting` |
| `Stop` | `stop` | `status: idle`; one reminder when something is left undone |
| `SubagentStart` | `subagent-start` | creates the subagent's session document, linked to its parent |
| `SubagentStop` | `subagent-stop` | the subagent's document: `status: ended` |
| `SessionEnd` | `session-end` | `status: ended`, `ended` |

Each hook finds its session document by the harness's id: `session_id`, or `agent_id` for a call made inside a subagent. The first six characters of the id are in the file name, so the lookup is one glob.

## session-start

1. Find the vault: the nearest `Atlas.md` at or above the working directory; else the vault, from `~/.atlas/config.json`, whose repository page holds the working directory.
2. Create the session document (`source: startup` or `clear`), or set the existing one to `running` (`resume`, `compact`).
3. Run `vault sync`: thread stages and callouts, `active` flags, lost sessions, `.claude/settings.json`.
4. Print the context, from [[Vault Status]]:

```text
atlas: vault Work at ~/notes/work · session [[2026-09-27 1432 a1b2c3]]
Started in repository p3-edge: vault → work → p3 → p3-edge (12 commits past its page)
Running now: 2 sessions · 1 waits for you: "Tune the OTA retry policy" (p3-cloud)
Open threads: 6 (tasks 2, spec 1, stub 3)
- [tasks 1/2] Filter vehicle false alarms · high · p3-edge · active
- …
Inbox: 2 files · Pending for the wiki: 3 documents · Proposed changes: 1
Recent changes: Ingest the DINOv2 paper (2026-09-27) · …
Rules: a change to a repository needs a thread (thread-work). The wiki changes only through a change.
The atlas skill routes any request. Vault context follows.
<vault-context> the body of Atlas.md, at most 60 lines </vault-context>
```

The context is data about the vault. The vault context is the user's text, and the hook marks it as such.

## guard

`PreToolUse` on `Write`, `Edit`, `MultiEdit`, `NotebookEdit`, and `Bash`, and on `apply_patch` in Codex (every path of the patch, both sides of a move). The first rule that matches decides. A refusal prints `permissionDecision: deny` and a reason that names the call to make.

| # | When | Refuses | Reason names |
|---|---|---|---|
| 1 | the caller is a read-only agent (`agent_type` is `wiki-extract`, `wiki-draft`, `wiki-reviewer`, or `thread-review`) | every write tool; every Bash command but `git log`, `git diff`, and `git show` | "this agent is read-only" |
| 2 | the path is under `wiki/` | Write, Edit | `change` propose |
| 3 | the path is under `changes/` or `sessions/`, or is `Atlas.md`, a `.base` file, or `.claude/settings.json` | Write, Edit | the tool that owns it |
| 4 | the path is a new file under `threads/` | Write | `thread` open, file, or tasks |
| 5 | the path is inside a linked repository, and the session's document has no `thread` | Write, Edit | `thread` open or attach, and the thread-work skill |

- Rule 5 is the thread rule: work on a repository needs a thread. A subagent inherits its parent's thread at `SubagentStart`, so a subagent sent by a session that has a thread may edit.
- Edit of an existing thread document is allowed. That is how the model writes a spec's prose or a task's progress.
- Bash is guarded only by rule 1. A shell command can write anywhere, and parsing shell is not reliable. The snapshot commit keeps any such write in the vault recoverable, and the repository's own git keeps the rest.

## touched

`PostToolUse` sees the tool, its input, and its result. It writes only session documents and the few fields named here.

| Call | Writes |
|---|---|
| Write or Edit inside a linked repository | adds the repository to the session's `repositories` |
| Write or Edit of a thread document | the document's `updated`; binds the session to the thread |
| `thread` open, attach, file, tasks, task | binds the session to the thread; `task` start sets the session's `task`; done or drop clears it; then runs the sync of `active` |
| `change` propose, apply | adds the change to the session's `changes`; writes the session into the change's `session` |
| `session` | writes the [[Session Note]] into the session document |
| any call | `updated`, at most once a minute |

## stop

Sets `status: idle`. Then it prints one `systemMessage`, once per session for each case:

- the session edited a repository and has no `description`;
- the session proposed a change that is still `proposed`;
- the session holds an active task and wrote no progress line in this session.

## What each host must give

| Need | Claude Code (verified 2026-09-27) | When a host lacks it |
|---|---|---|
| the session id in every event | `session_id` | Atlas cannot keep sessions; it keeps only the guard rules 2 to 4 |
| a subagent's id and type in its tool events | `agent_id`, `agent_type` | a subagent's calls count as its parent's |
| a subagent's start, with the parent's id | `SubagentStart`: `agent_id`, `agent_type`, `parentSessionId` | no session document per subagent |
| the tool's result in `PostToolUse` | `tool_response`, for MCP tools too | the thread and change links are made from the input alone |
| the end of a session | `SessionEnd`, with `reason` | sessions go `idle`, then `lost` after `stale_hours` |
| a wait for the user | `Notification`, with its type | no `waiting` status |

Codex supplies `CLAUDE_PLUGIN_ROOT` for compatibility, and its `apply_patch` input holds the patch text in `tool_input.command`. The rest of this table must be checked against the Codex hooks when that host is built.

## Speed

Hooks run on every tool call, so each one reads only what it needs: the session document by glob, and for the guard, the `path` field of the repository pages. A hook's target is under 50 ms.
