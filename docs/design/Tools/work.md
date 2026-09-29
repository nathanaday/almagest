# work

> Every write to a stub, a spec, or an event, and the two reads of work. See [[Stub]], [[Spec]], and [[Event]] for the documents and their rules.

| Action | Takes | Returns | Writes |
|---|---|---|---|
| `list` (default) | optional filters (`tags`, `repository`) | the Board ([[Work View]]) | nothing |
| `show` | a document | [[Work View]] | nothing |
| `stub` | `text`, optional `title`, `tags`, `priority`, `inbox` | [[Work View]] | the stub; removes the `inbox` note |
| `spec` | a list of specs: `title`, `kind`, `text`, and the spec fields; optional `parent`, `from`, `resolve` | [[Work View]] of the first spec's root | one document per spec |
| `promote` | a stub, `kind`, `text`, the spec fields, optional `title` | [[Work View]] | the stub becomes a spec in place; a `promoted` event |
| `start` | a plan with no parts, optional `take` | [[Work View]] | a `started` or `continued` event; `started` on each open ancestor |
| `done` | a plan, `result` (the four sections) | [[Work View]] | a `completed` event |
| `drop` | a stub or a plan, `reason` | [[Work View]] | a `dropped` event, and one for each open plan below |
| `reopen` | a stub or a plan, optional `reason` | [[Work View]] | a `reopened` event |
| `block` | a plan, `reason` (one line) | [[Work View]] | a `blocked` event |
| `unblock` | a plan | [[Work View]] | an `unblocked` event |
| `resolve` | a stub, `became` (ids), optional `text` | [[Work View]] | `became` on the stub; a `resolved` event |
| `note` | any document, `text` | [[Work View]] of it | a `note` event |
| `set` | a stub or a spec, and any of `title`, `description`, `tags`, `aliases`, `priority`, `parent`, `repositories`, `depends`, `order`, `implements`, `supersedes` | [[Work View]] | the fields; for a title, the file and every link |

- Every write is one `work` commit, with the trailer `Atlas-Work: <id>`, and ends in sync and the views.
- `text`, `result`, and `reason` are the model's prose. The tool writes them into the document's sections and writes no prose of its own. `text` for a spec is the body below the lead callout; code puts back its own sections (`## Parts`, `## History`, `## Implemented by`) and `## Origin`.
- `stub` without `title` takes the first line of `text`, cut at 60 characters at a word boundary. The model should give a title.
- `spec` writes several specs in one commit: a plan and its parts, with `depends` by title among them. Each part takes `parent`; the list may name a new parent by its title in the same call.
- `start` binds the session to the plan: the hook adds it to the session's `specs` ([[Sessions#Links]]). A second `start` in the same session writes no second `continued`.
- `done` takes `result` as four fields, `delivered`, `verified`, `follow_ups`, and `learned`, and writes them as the sections of the `completed` event ([[Event#Body]]).
- `set` of `parent`, `depends`, or `repositories` is refused on a done or dropped plan. `set` of `blocked` or `status` does not exist: those come from events.
- `list` and `show` bind nothing and write nothing, so a question can look at work freely.

Refusals: every rule in [[Spec#Rules the work tool enforces]] and [[Stub#Rules]]; a title that collides with another document or begins with `Tag · ` or `View · `; a tag that breaks [[Documents#Form]], or a new tag in `tags: known` mode without `new_tags: true`; a promote of a document that is not an open stub; `note` on a document that is not typed.
