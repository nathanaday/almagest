# Event

A record of something that happened to a document: work started on a plan, work completed, a stub became a spec. Code writes an event when a `work` call or a `change` apply moves a document from one state to another. Linked to their subjects, the events are the history of what the agents and you did, and the timeline view reads them ([[Views#Timeline]]).

The events of a document decide its status. So the history is the record, and no field can disagree with it.

## Kinds

| Kind | Subject | Written by | Holds prose | Sets status |
|---|---|---|---|---|
| `started` | plan | `work start` on an open plan; the first start of a child, on each open ancestor | no | `started` |
| `continued` | plan | `work start` on a started plan, at most once per session | no | `started` |
| `completed` | plan | `work done` | yes: the result | `done` |
| `dropped` | stub, plan | `work drop` | yes: the reason | `dropped` |
| `reopened` | stub, plan | `work reopen` | optional: why | `open` |
| `blocked` | plan | `work block` | the one line | no; sets `blocked` |
| `unblocked` | plan | `work unblock` | no | no; clears `blocked` |
| `promoted` | the document a stub became in place | `work promote`; `change` apply of `op: promote` | no | the new type's |
| `resolved` | stub | `work resolve`; a create or capture with `resolve` | no | `resolved` |
| `note` | any document | `work note` | yes | no |

There is no `opened` event. A document's `created` time records it, and the timeline reads it from there.

## Fields

```yaml
---
id: doc-7h2kpe
type: event
kind: completed
description: "Completed: Score boxes by motion"
tags: [work/p3/p3-edge, ml]        # the subject's tags at the time
aliases: []
created: 2026-09-30T11:02:45
updated: 2026-09-30T11:02:45
refreshed: 2026-09-30T11:02:45
# owned by code, every one:
at: 2026-09-30T11:02:45
subject: "[[Score boxes by motion]]"
subject_id: doc-9d4mzt
session: "[[2026-09-30 1040 f3e9a1]]"   # empty when you acted in Obsidian or a terminal
by: agent                           # agent | user
from_type: ""                       # promoted: the type before (stub)
to_type: ""                         # promoted: the type after (spec, topic)
became: []                          # resolved: the documents the stub spawned
change: ""                          # promoted through a change: the change document
---
```

- Code writes every field. The model gives only the prose of the kinds that hold it.
- `description` is `<Kind>: <subject title>`, and for a block, `Blocked: <the line>`.
- `tags` are a copy of the subject's tags at the time, so a tag's view shows its history.
- `session` comes from the hook ([[Sessions#Links]]). A call from the CLI or the Obsidian plugin has no session and `by: user`.

## Title

`<subject title> · <kind> <yyyy-mm-dd hhmm>`: `Score boxes by motion · completed 2026-09-30 1102`. In the flat `wiki/documents/`, the events of a document sort next to it.

- A second event of one kind on one subject in the same minute takes the seconds: `… 110245`.
- An event's title keeps the subject's title of its time. A later rename of the subject rewrites the link in `subject`, and leaves the event's file name alone.

## Lead callout

```markdown
> [!completed] Completed · [[Score boxes by motion]] · 2026-09-30 11:02:45
> By the agent in [[2026-09-30 1040 f3e9a1]] · part of [[Filter vehicle false alarms]]
```

The callout type is the kind. A promotion reads `Promoted from stub to spec`; a resolution lists what the stub became.

## Body

1. The lead callout.
2. The prose of the kind, from the `work` call:
   - `completed`: `## Delivered` (what changed, with the commits), `## Verified` (how, with the commands and their results), `## Follow-ups` (links to new stubs), `## Learned` (what the wiki should absorb; [[wiki-sync]] reads it first).
   - `dropped`: `## Why`, and `## Follow-ups`.
   - `reopened`: `## Why`, when given.
   - `note`: `## Note`.
3. `## Notes`: yours.

## Status from events

Code derives the status of a stub or a plan from its lifecycle events: the kinds with a status in the table above. The last one by `at` decides. With none, the status is `open`.

- Sync derives the status again after every write, at session start, and when the Obsidian plugin reports a change. A hand edit works: delete the `completed` event in Obsidian, and the plan is `started` again at the next sync.
- A `promoted` event ends the stub's lifecycle: the document is a spec or a topic from then on, with that type's status.
- The first `started` of a plan is when work began. The `completed` event holds the result.

## Rules

- An event is written by code in the commit of the call that caused it. The model never creates one, and the guard refuses a Write that would.
- The model may Edit the prose of an event, as it may a spec's, to correct a result. It may not edit the frontmatter or the lead callout.
- `subject` must name a document. Lint reports an event whose subject is gone.
- An event is never removed by a tool. `work reopen` writes a new event; it does not delete the old one.

## Pending

`completed`, `dropped`, and `note` events are pending until an applied change absorbs them, when `wikify` holds `event` ([[Vault Layout#Atlas.md]]). The other kinds hold no prose, and are never pending.
