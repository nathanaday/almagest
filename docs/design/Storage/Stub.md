# Stub

An idea planted in a hurry, in your words: "try DINOv2 for the false alarms", "read the OTA paper", "the login timeout is too short". A stub asks no questions and holds no plan. It may become anything later: a spec, a topic, a source, or several of them. Or it may be dropped.

The `work` tool writes a stub (`work stub`), and the [[wiki-stub]] skill calls it with no questions.

## Fields

```yaml
---
id: doc-c7v2kq
type: stub
description: "Try DINOv2 features to filter vehicle false alarms."
tags: [work/p3/p3-edge, ml]
aliases: []
created: 2026-09-29T14:32:05
updated: 2026-09-29T14:32:05
refreshed: 2026-09-29T14:32:05
priority: normal                    # high | normal | low | someday
# owned by code:
status: open                        # open | resolved | dropped
became: []                          # the documents it spawned, when resolved
---
```

## Lead callout

```markdown
> [!stub] Open · normal · planted 2026-09-29 in [[2026-09-29 1432 a1b2c3]]
> #work/p3/p3-edge · #ml
```

A resolved stub lists what it became: `> [!stub-resolved] Resolved 2026-10-02 → [[Filter vehicle false alarms]], [[DINOv2]]`. A dropped stub gives the reason's first line and links the event.

## Body

1. The lead callout.
2. `## Idea`: your words, as you gave them. Required. The model does not rewrite it.
3. `## Notes`: yours: references, concerns, links. The model adds to it only when you ask.

## Becoming something

A stub becomes something else in one of two ways. Each way writes an event, so the history keeps the move.

### In place

When the stub becomes one document, it changes type and keeps its id, its file, and every link to it. A link to the idea now leads to the result.

| Becomes | Through | Writes |
|---|---|---|
| a spec | `work promote` | the spec's fields and sections; a `promoted` event; one `work` commit |
| a topic | `change`, `op: promote` | the topic's fields and sections; a `promoted` event; in the change's commit, after your yes |

In both:

- `## Idea` becomes `## Origin`, kept word for word, after the new type's sections and before `## Notes`.
- `status`, `became`, and `priority` go, except that a spec keeps `priority`. The new type's fields come in.
- The call may give a new title. Code renames the file and rewrites every link in the same commit.
- `tags`, `aliases`, `description`, and `created` stay, unless the call gives new ones.

A source cannot come in place: it is a capture of a file. A stub that asks for a source is resolved by the capture ([[Source Document#Rules]]).

### By spawning

When the stub becomes several documents, or a source, each new document names it in `from`, and the stub closes:

1. Create each document: `work spec` with `from`, a `change` create with `from`, or `source` capture with `resolves`.
2. `work resolve` with `became`, or `resolve: true` on the call that creates the last one.

The stub's `status` becomes `resolved`, `became` lists the documents, and a `resolved` event records it. The stub stays in the vault, so links to the idea still resolve and show where it went.

### Dropped

`work drop` with a reason writes a `dropped` event that holds the reason. The stub stays, with `status: dropped`. `work reopen` brings it back to `open`.

## Rules

- A stub names no repository and never unlocks an edit in one. Work needs a started spec ([[Spec#The edit rule]]).
- `work stub` with `inbox` replaces a note in `inbox/`: the stub keeps its text, and the note goes in the same commit.
- A stub's `status` is derived from its events: `resolved` or `dropped` after the event of that kind, `open` otherwise and after `reopened` ([[Event#Status from events]]).
