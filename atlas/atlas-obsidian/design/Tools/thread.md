
# thread

> Every write to a thread, and the two reads. See [[Thread Documents]] for the documents and the rules.

| Action | Takes | Returns | Writes |
|---|---|---|---|
| `list` (default) | nothing | the Board ([[Thread View]]) | nothing |
| `show` | a thread | [[Thread View]] | nothing |
| `open` | `text`, optional `title`, `scope`, `priority`, `inbox` | [[Thread View]] | the folder and the stub; removes the `inbox` note |
| `attach` | a thread | [[Thread View]] | nothing; the hook binds the session to the thread |
| `file` | a thread, `part: spec \| receipt`, `text`, `outcome` for a receipt | [[Thread View]] | the document |
| `tasks` | a thread, a list of tasks | [[Thread View]] | one task document per task |
| `task` | a task, `do: start \| done \| drop \| reopen \| set`; `result` for done; `take` for start; the fields for set | [[Thread View]] | the task's `status` and `## Result`; its fields for set; nothing for `start` |
| `set` | a thread, `title`, `priority`, `blocked`, or `scope` | [[Thread View]] | the stub; for a title, every file and link |
| `reopen` | a thread | [[Thread View]] | marks the receipt superseded |

- Every write is one `thread` commit and ends in sync.
- `open` without `title` takes the first line of `text`, cut at 60 characters at a word boundary. The model should give a title.
- `open` with `inbox` removes that note from `inbox/` in the same commit. The stub keeps its text, so nothing is lost.
- `task start` writes nothing. The hook adds the task to the session's `tasks`, and sync marks the task `active`. Starting a task whose dependencies are open is refused. Starting a task that another `running`, `waiting`, or `idle` session lists is refused unless `take: true`, so two sessions never work one task by accident.
- `task set` changes a task's `title`, `repository`, `depends`, `order`, or `blocked`. A blocked task holds only itself; the thread's `blocked` is for the whole thread.
- A rename (`set` with `title`) uses the link rewrite pass of [[Changes#Link rewrites]].
- The acts that bind the session to the thread are `open`, `attach`, `file`, `tasks`, and `task`. `list` and `show` bind nothing, so a question can look at threads freely.

Refusals: every rule in [[Thread Documents#Rules the thread tool enforces]]; a title that collides with another document.
