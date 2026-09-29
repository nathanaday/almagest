
# Hooks

A hook is code the harness runs on an event. Atlas uses hooks for two jobs that a skill cannot do reliably:

1. **Rules that must hold.** The guard refuses a call that breaks a rule, whether or not the model read the skill.
2. **Facts about sessions.** Hooks create and keep every session document and every link between a session and another document. The model does not need to remember.

Every hook is one command, `atlas-obsidian hook <event>`, which reads the event's JSON on stdin. A hook that finds no vault for the session exits at once and does nothing, so Atlas stays out of sessions that are not its own. Every hook that writes a document takes `.git/atlas.lock` for that write.

## The events

| Event | Command | Does |
|---|---|---|
| `SessionStart` | `session-start` | creates or resumes the session document; runs `vault sync`; prints the opening context |
| `UserPromptSubmit` | `prompt` | `status: running`; `last_prompt`; prints the note open in Obsidian |
| `PreToolUse` on write tools, Bash, and the atlas write tools | `guard` | refuses a call that breaks a rule |
| `PostToolUse` | `touched` | links the session to what the call touched |
| `Notification` | `notify` | `waiting` or `idle`, by the notification's type ([[Sessions#Status]]) |
| `Stop` | `stop` | `status: idle`; one reminder when something is left undone |
| `SubagentStart` | `subagent-start` | a session document for a subagent that may write; a line in the parent's document for a read-only worker |
| `SubagentStop` | `subagent-stop` | `status: ended`, or the end time on the worker's line |
| `SessionEnd` | `session-end` | `status: ended`, `ended` |

Each hook finds its session document by `agent_id` when the event has one, and by `session_id` otherwise ([[Sessions#Names and keys]]). The first six characters of the key are in the file name, so the lookup is one glob.

## session-start

1. Find the vault: the nearest `Atlas.md` at or above the working directory; else the vault, from `~/.atlas/config.json`, whose repository page holds the working directory.
2. Create the session document (`source: startup` or `clear`), or set the existing one to `running` (`resume`, `compact`).
3. Run `vault sync`: recovery of a change left `applying`, thread stages and callouts, `active` flags, lost sessions, and the harness settings ([[Vault Layout#Settings for the harness]]).
4. Print the context, from [[Vault Status]]:

```text
atlas: vault Work at ~/notes/work · this session: [[2026-09-27 1432 a1b2c3]]
Started in repository p3-edge: vault → work → p3 → p3-edge (12 commits past its page)
Running now: 2 sessions · 1 waits for you: "Tune the OTA retry policy" (p3-cloud)
Open threads: 6 (tasks 2, spec 1, stub 3)
- [tasks 1/2] Filter vehicle false alarms · high · p3-edge · active
- …
Inbox: 2 files · Pending for the wiki: 3 documents · Proposed changes: 1 · Mentions: 2
Recent changes: Ingest the DINOv2 paper (2026-09-27) · …
Rules: a change to a repository needs an open thread (thread-work). The wiki changes only through a change.
Write one line under ## Description in this session's document once you know the work.
The atlas skill routes any request. Vault context follows.
<vault-context> the body of Atlas.md, at most 60 lines </vault-context>
```

The vault context is the user's text, and the hook marks it as such.

## prompt

Sets `status: running` and `last_prompt`. Then it prints one line when Obsidian has a note open in this vault: `Open in Obsidian: [[Filter vehicle false alarms — Spec]]`. It reads the active file from `.obsidian/workspace.json`, which Obsidian keeps up to date. So "this", "the note I have open", and "the document I am looking at" mean something to the agent, and no plugin is needed.

## guard

`PreToolUse` on `Write`, `Edit`, `MultiEdit`, `NotebookEdit`, `Bash`, the atlas write tools (`change`, `thread`, `source`, `vault`), and Codex's `apply_patch` (every path of the patch, both sides of a move). The first rule that matches decides. A refusal prints `permissionDecision: deny` and a reason that names the call to make.

| # | When | Refuses | Reason names |
|---|---|---|---|
| 1 | the caller is a read-only agent (`agent_type` is `wiki-extract`, `wiki-draft`, `wiki-audit`, or `thread-review`) | every write tool; every atlas write action (`change`, `thread` writes, `source` capture, `vault` init, sync, mention); every Bash command, except for `thread-review` a command that is exactly `git log`, `git diff`, or `git show`, with arguments that hold no shell operator (`;`, `&`, `\|`, `<`, `>`, `$`, a backquote) and none of `--output`, `-c`, `--ext-diff`, `--textconv` | "this agent is read-only" |
| 2 | a Bash command runs the binary (`atlas-obsidian`, or `atlas` of 6.0 to 6.2) with `change … apply` or `hook` | the command | the `change` tool, Apply in Obsidian, or `!` for the user |
| 3 | the path is under `wiki/` | Write, Edit | `change` propose |
| 4 | the path is under `changes/`, or is `Atlas.md`, a `.base` file, or `.claude/settings.local.json` | Write, Edit | the tool that owns it |
| 5 | the path is under `sessions/` | Write; an Edit of another session's document; an Edit of the session's own document that touches the frontmatter, the lead callout, or `## Subagents` | "edit only Description, Progress, and Summary of your own document" |
| 6 | the path is under `threads/` | Write; an Edit that touches the frontmatter or the lead callout | `thread` open, file, tasks, task, or set |
| 7 | the path is inside a linked repository R, and no **open** thread in the session's `threads` holds R: in its `scope` (R, or an area above R), or as the `repository` of one of its tasks | Write, Edit | `thread` open or attach, and the thread-work skill |

- Rule 7 is the thread rule: work on a repository needs an open thread that covers it. A closed thread, or a thread about another repository, does not count. A subagent that may write inherits its parent's `threads` at `SubagentStart`.
- Only the `thread` tool binds a session to a thread. An Edit of a thread document binds nothing, so editing some stub does not unlock a repository.
- Rules 5 and 6 find the frontmatter and the lead callout by reading the file: an Edit whose `old_string` falls inside them is refused.
- Rules 3 to 7 take the vault above the file, not the vault of the session's folder, so a session outside the vault gets the same refusals.
- The gate is not a guard rule. The `change` tool keeps it, on the document it applies ([[Changes#The gate]]). Rule 2 only keeps the shell from skipping it through the CLI or from forging a user's turn through `atlas-obsidian hook prompt`.
- Bash is guarded only by rules 1 and 2. A shell command can write anywhere, and no parse of shell is reliable. So the thread rule covers the edit tools, not the shell. The snapshot commit keeps a shell write in the vault recoverable, and the repository's own git keeps the rest.

## touched

`PostToolUse` sees the tool, its input, and its result. It writes only session documents and the few fields named here.

| Call | Writes |
|---|---|
| Write or Edit inside a linked repository | adds the repository to the session's `repositories` |
| Edit of the session's own document | copies the first line of `## Description` into `description` |
| Edit of a thread document | the document's `updated` |
| `thread` open, attach, file, tasks, task | adds the thread to the session's `threads`; `task` start adds the task to `tasks`; then runs the sync of `active` |
| `change` propose | adds the change to the session's `changes`; writes the session into the change's `session` |
| any call | `updated`, at most once a minute |

## stop

Sets `status: idle`. Then it prints one `systemMessage`, once per session for each case:

- the session edited a repository and its `## Description` is empty;
- the session proposed a change that is still `proposed`;
- the session lists an open task and wrote no progress line in that task this session.

## What each host must give

| Need | Claude Code (verified 2026-09-27) | When a host lacks it |
|---|---|---|
| the session id in every event | `session_id` | Atlas cannot keep sessions; it keeps only the guard rules 3, 4, and 6 |
| a subagent's id and type in its tool events | `agent_id`, `agent_type` | a subagent's calls count as its parent's, and rule 1 cannot hold |
| a subagent's start, with the parent's id | `SubagentStart`: `agent_id`, `agent_type`, `parentSessionId` | no session document per subagent |
| the tool's result in `PostToolUse` | `tool_response`, for MCP tools too | the thread and change links are made from the input alone |
| the end of a session | `SessionEnd`, with `reason` | sessions go `idle`, then `lost` after `stale_hours` |
| a wait for the user | `Notification`, with its type | no `waiting` status |
| hook output as context | `SessionStart`; `UserPromptSubmit` to be confirmed | no opening context; no open-note line |

Codex supplies `CLAUDE_PLUGIN_ROOT` for compatibility, and its `apply_patch` input holds the patch text in `tool_input.command`. The rest of this table must be checked against the Codex hooks when that host is built.

## Speed

Hooks run on every tool call, so each one reads only what it needs: the session document by glob, and for the guard, the `path` field of the repository pages and the `scope` of the session's threads. A hook's target is under 50 ms.
