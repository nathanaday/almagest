
# thread

> Every write to a thread, and the two reads. See [[Thread Documents]] for the documents and the rules.

| Action | Takes | Returns | Writes |
|---|---|---|---|
| `list` (default) | nothing | the Board ([[Thread View]]) | nothing |
| `show` | a thread | [[Thread View]] | nothing |
| `open` | `text`, `scope`, optional `title`, `priority`, `inbox`, `mention` | [[Thread View]] | the folder and the stub; removes the `inbox` note |
| `attach` | a thread | [[Thread View]] | nothing; the hook binds the session to the thread |
| `file` | a thread, `part: spec \| receipt`, `text`, `outcome` for a receipt | [[Thread View]] | the document |
| `tasks` | a thread, a list of tasks | [[Thread View]] | one task document per task |
| `task` | a task, `do: start \| done \| drop \| reopen`, `result` for done | [[Thread View]] | the task's `status` and `## Result`; nothing for `start` |
| `set` | a thread, `title`, `priority`, `blocked`, or `scope` | [[Thread View]] | the stub; for a title, every file and link |
| `reopen` | a thread | [[Thread View]] | removes the receipt |

- Every write is one `thread` commit and ends in sync.
- `open` without `title` takes the first line of `text`, cut at 60 characters at a word boundary. The model should give a title.
- `open` with `inbox` removes that note from `inbox/` in the same commit. The stub keeps its text, so nothing is lost.
- `open` with `mention` checks the box of that mention line and appends a link to the new stub ([[Obsidian Plugin#Mentions]]).
- `task start` writes nothing. The hook sets the session's `task`, and sync marks the task `active`. Starting a task whose dependencies are open is refused.
- The acts that bind the session to the thread are `open`, `attach`, `file`, `tasks`, and `task`. `list` and `show` bind nothing, so a question can look at threads freely.

Refusals: every rule in [[Thread Documents#Rules the thread tool enforces]]; a title that collides with another document.
