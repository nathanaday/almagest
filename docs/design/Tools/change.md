# change

> The only way knowledge changes: a source, a repository, or a topic. See [[Changes]] for the document, the operations, the validation rules, apply, and undo; and [[change.canvas|the change flow]].

| Action | Takes | Returns | Writes |
|---|---|---|---|
| `show` (default) | a change id | [[Change Preview]] | nothing |
| `propose` | [[Wiki Change Plan]] | [[Change Preview]] | the change document, uncommitted |
| `apply` | a change id | [[Change Preview]] with `commit` | the documents, their events, and the change document: one `change` commit |
| `reject` | a change id, a reason | [[Change Preview]] | the change document's status and callout, uncommitted |
| `undo` | a change id | [[Change Preview]] | the restored documents and the change document: one `undo` commit |

- `propose` never commits. The change document waits under `changes/` until you approve it, reject it, or a later proposal supersedes it.
- `apply` reads the change document from disk, so an edit you made to a proposed document goes in. It validates again before it writes.
- The skill that proposes shows the preview, links the change document, and asks for a yes. The tool refuses the model's `apply` until the user of the proposing session has had a turn since the proposal ([[Changes#The gate]]). The Obsidian plugin's Apply button is the other path.
- The PostToolUse hook of `propose` writes the calling session into the change document's `session` field, and the change into the session's `changes`.
- Every action first recovers a change left `applying` by a crash ([[Changes#Apply]]).

Refusals: every rule in [[Changes#Validation]]; the gate; `apply` on a change that is not `proposed`; `undo` on a change that is not `applied`, or whose paths changed since.
