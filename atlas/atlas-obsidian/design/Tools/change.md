
# change

> The only way the wiki changes. See [[Changes]] for the document, the validation rules, apply, and undo; and [[change.canvas|the change flow]].

| Action | Takes | Returns | Writes |
|---|---|---|---|
| `show` (default) | a change id | [[Change Preview]] | nothing |
| `propose` | [[Wiki Change Plan]] | [[Change Preview]] | the change document, uncommitted |
| `apply` | a change id | [[Change Preview]] with `commit` | the pages and the change document: one `change` commit |
| `reject` | a change id, a reason | [[Change Preview]] | the change document's status |
| `undo` | a change id | [[Change Preview]] | the restored pages and the change document: one `undo` commit |

- `propose` never commits. The change document waits under `changes/` until you approve it, reject it, or a later proposal supersedes it.
- `apply` reads the change document from disk, so an edit you made to a proposed page goes in. It validates again before it writes.
- The skill that proposes shows the preview, links the change document, and asks for a yes. The tool cannot know whether you said yes; the skill's gate is the rule, and the preview in chat and in Obsidian makes it visible.
- The PostToolUse hook writes the calling session into the change document's `session` field, and the change into the session's `changes`.

Refusals: every rule in [[Changes#Validation]]; `apply` on a change that is not `proposed`; `undo` on a change that is not `applied`, or whose paths changed since.
