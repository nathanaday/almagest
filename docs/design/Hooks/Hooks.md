# Hooks

A hook is code the harness runs on an event. Atlas uses hooks for two jobs that a skill cannot do reliably:

1. **Rules that must hold.** The guard refuses a call that breaks a rule, whether or not the model read the skill.
2. **Facts about sessions.** Hooks create and keep every session document and every link between a session and another document. The model does not need to remember.

Every hook is one command, `atlas-obsidian hook <event>`, which reads the event's JSON on stdin. A hook that finds no vault for the session exits at once and does nothing, so Atlas stays out of sessions that are not its own. Every hook that writes a document takes `.git/atlas.lock` for that write. A hook never writes the views.

## The events

| Event | Command | Does |
|---|---|---|
| `SessionStart` | `session-start` | creates or resumes the session document; runs `vault sync`; prints the opening context |
| `UserPromptSubmit` | `prompt` | `status: running`; `last_prompt` for a real prompt; prints the note open in Obsidian |
| `PreToolUse` on write tools, Bash, and the atlas write tools | `guard` | refuses a call that breaks a rule |
| `PostToolUse` | `touched` | links the session to what the call touched |
| `Notification` | `notify` | `waiting` or `idle`, by the notification's type ([[Sessions#Status]]) |
| `Stop` | `stop` | `status: idle`; blocks once for the agent's own debts |
| `SubagentStart` | `subagent-start` | a session document for a subagent that may write; a line in the parent's document for a read-only worker |
| `SubagentStop` | `subagent-stop` | `status: ended`, or the end time on the worker's line |
| `SessionEnd` | `session-end` | `status: ended`, `ended` |

Each hook finds its session document by `agent_id` when the event has one, and by `session_id` otherwise ([[Sessions#Names and keys]]). The first six characters of the key are in the file name, so the lookup is one glob.

## session-start

1. Find the vault: the nearest `Atlas.md` at or above the working directory; else the vault, from `~/.atlas/config.json`, whose repository document holds the working directory.
2. Create the session document (`source: startup` or `clear`), or set the existing one to `running` (`resume`, `compact`).
3. When the vault's `layout` is below 3, print one line that names `atlas-obsidian vault migrate`, and stop here.
4. Run `vault sync` ([[vault#sync]]).
5. Print the context, from [[Vault Status]]:

```text
atlas: vault Work at ~/notes/work · this session: [[2026-09-29 1432 a1b2c3]]
Started in repository p3-edge (tag work/p3/p3-edge; 12 commits past its description)
Running now: 2 sessions · 1 waits for you: "Tune the OTA retry policy" (p3-cloud)
Work: 2 started, 5 open, 1 blocked, 6 stubs
- [started] Score boxes by motion · part of Filter vehicle false alarms · p3-edge · active
- …
Tags (known mode): work/p3 64 · school/cs513 34 · self-driving 12 · ml 30 · …
Inbox: 2 files · Pending for the wiki: 3 documents · Proposed changes: 1 · Mentions: 2
Recent: completed Collect false alarm clips (2026-09-29) · applied Ingest the DINOv2 paper · …
Rules: an edit in a repository needs a started plan (spec-work). Knowledge changes only through a change.
Write one line under ## Description in this session's document once you know the work.
The atlas skill routes any request. Vault context follows.
<vault-context> the body of Atlas.md, at most 60 lines </vault-context>
```

The vault context is the user's text, and the hook marks it as such. The tag line lists at most 30 tags, the most used first; `vault` status has the rest.

## prompt

Sets `status: running`. For a prompt that is the user's turn, it sets `last_prompt`; a subagent's hand-back (`<agent-message …>`) and a background task's notice (`<task-notification …>`) arrive as prompts too, and do not count. Then it prints one line when Obsidian has a note open in this vault: `Open in Obsidian: [[Score boxes by motion]]`. It reads the active file from `.obsidian/workspace.json`, which Obsidian keeps up to date.

## guard

`PreToolUse` on `Write`, `Edit`, `MultiEdit`, `NotebookEdit`, `Bash`, the atlas write tools (`change`, `work`, `source`, `vault`), and Codex's `apply_patch` (every path of the patch, both sides of a move). The first rule that matches decides. A refusal prints `permissionDecision: deny` and a reason that names the call to make.

| # | When | Refuses | Reason names |
|---|---|---|---|
| 1 | the caller is a read-only agent (`agent_type` is `wiki-extract`, `wiki-draft`, `wiki-audit`, or `spec-review`) | every write tool; every atlas write action; every Bash command, except for `spec-review` a command that is exactly `git log`, `git diff`, or `git show` (optionally with `-C <path>`), with arguments that hold no shell operator (`;`, `&`, `\|`, `<`, `>`, `$`, a backquote) and none of `--output`, `-c`, `--ext-diff`, `--textconv` | "this agent is read-only" |
| 2 | a Bash command runs the binary with `change … apply`, `vault migrate`, or `hook` | the command | the `change` tool, Apply in Obsidian, or `!` for the user |
| 3 | the path is in `wiki/documents/` and the file does not exist | Write | `work` stub or spec, `change` create, or `source` capture |
| 4 | the path is a source, repository, or topic in `wiki/documents/`, or is under `wiki/assets/` | Write, Edit | `change` propose |
| 5 | the path is a stub, spec, or event in `wiki/documents/` | an Edit that touches the frontmatter, the lead callout, or a section that code writes (`## Parts`, `## History`, `## Implemented by`, `## Origin`) | `work` set, or the section's owner |
| 6 | the path is under `changes/` or `views/`, or is `Atlas.md`, a `.base` file, or `.claude/settings.local.json` | Write, Edit | the tool that owns it |
| 7 | the path is under `sessions/` | Write; an Edit of another session's document; an Edit of the session's own document that touches the frontmatter, the lead callout, or `## Subagents` | "edit only Description, Progress, and Summary of your own document" |
| 8 | the path is inside a linked repository R, and no plan in the session's `specs` is `started` and covers R ([[Spec#The edit rule]]) | Write, Edit | `work` start, and the spec-work skill |

- Rule 8 is the edit rule. A done, dropped, or open plan does not count, nor a plan about another repository. A subagent that may write inherits its parent's `specs` at `SubagentStart`.
- Only `work` start binds a session to a plan. An Edit of a spec binds nothing, so editing a spec does not unlock a repository.
- Rules 4 and 5 read the file's `type` from its frontmatter. Rule 5 finds the frontmatter, the lead callout, and the code's sections by reading the file: an Edit whose `old_string` falls inside them is refused.
- Rules 3 to 8 take the vault above the file, not the vault of the session's folder, so a session outside the vault gets the same refusals.
- The gate is not a guard rule. The `change` tool keeps it, on the document it applies ([[Changes#The gate]]). Rule 2 only keeps the shell from skipping it or from forging a user's turn.
- Bash is guarded only by rules 1 and 2. A shell command can write anywhere, and no parse of shell is reliable. The snapshot commit keeps a shell write in the vault recoverable, and the repository's own git keeps the rest. The skills tell the agent to change files with Edit and Write.

## touched

`PostToolUse` sees the tool, its input, and its result. It writes only session documents and the fields named here.

| Call | Writes |
|---|---|
| Write or Edit inside a linked repository | adds the repository to the session's `repositories` |
| Bash with a write mark (a redirect, `sed -i`, `git commit`, …) that runs in a linked repository or names one | adds the repository to `repositories` |
| Edit of a stub, spec, or event | the document's `updated` |
| `work` start | adds the plan to the session's `specs`; sets `active` on it |
| any `work` write | adds each stub or spec it wrote to `work`; writes the session into each event it wrote, and counts them in `events` |
| `change` propose | adds the change to the session's `changes`; writes the session into the change's `session` |
| `change` apply | writes the session into each `promoted` event it wrote |
| any call | `updated`, at most once a minute; the first line of `## Description` into `description` |

## stop

Sets `status: idle`. Then, once per session and never while `stop_hook_active`, it blocks the stop with the reason when:

- the session edited a repository and its `## Description` is empty;
- the session started a plan and wrote no `## Progress` line in it this session.

A change the session proposed that is still `proposed` is a `systemMessage`, because the user acts on it.

## What each host must give

| Need | Claude Code (verified 2026-09-28) | When a host lacks it |
|---|---|---|
| the session id in every event | `session_id` | Atlas cannot keep sessions; it keeps only the guard rules 3 to 7 |
| a subagent's id and type in its tool events | `agent_id`, `agent_type` | a subagent's calls count as its parent's, and rule 1 cannot hold |
| a subagent's start, with the parent's id | `SubagentStart`: `agent_id`, `agent_type`, the parent's `session_id` | no session document per subagent |
| the tool's result in `PostToolUse` | `tool_response`, for MCP tools too | the event and change links are made from the input alone |
| the end of a session | `SessionEnd`, with `reason` | sessions go `idle`, then `lost` after `stale_hours` |
| a wait for the user | `Notification`, with its type | no `waiting` status |
| hook output as context | `SessionStart`; `UserPromptSubmit` | no opening context; no open-note line |

Codex supplies `CLAUDE_PLUGIN_ROOT` for compatibility, and its `apply_patch` input holds the patch text in `tool_input.command`. The rest of this table must be checked against the Codex hooks when that host is built.

## Speed

Hooks run on every tool call, so each one reads only what it needs: the session document by glob, and for the guard, the `path` field of the repository documents, the `type` of the file it guards, and the `status` and `repositories` of the session's plans. A hook's target is under 50 ms.
