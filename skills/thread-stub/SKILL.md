---
name: thread-stub
description: "Plant a stub from a sentence, in the user's words, with no questions. A stub is the front page of a thread; it may also become a topic or a source later. Use for note this, stub this, remember to, idea for later, read this someday, report a bug, add a todo, new thread. Keeping an answer from this conversation is wiki-save; starting the work now is thread-work; several threads toward one goal is chord-create."
---

# thread-stub

A stub is an idea planted in a hurry, loose by design. It keeps the user's words as
they gave them. The questions come later, when the stub gets its spec (thread-spec), or
becomes a topic (wiki-edit) or a source (wiki-ingest).

Tools: `search`, `thread` (stub). References:
[threads.md](../atlas/references/threads.md).

## Procedure

1. `search` the open threads for the same idea: `types: [stub]`, and the request's words
   as `text`. When one matches and is not ended, offer to add the new words to its
   `## Notes` (with Edit), and stop if the user agrees.
2. Call `thread` with `action: stub`:
   - `text`: the user's words, as given; they become the stub's `## Idea`;
   - `title`: a short name for the idea;
   - `tags`: the tags that exist and fit what the request names or the conversation is
     about, from the tag list in the session-start context; or none. Use a tag that
     exists before you make a new one. In `tagging: known` mode, a new tag needs
     `new_tags: true`, which needs the user's yes, so leave the new tag out instead;
   - `priority`: only when the user says one (high, normal, low, or someday);
   - `chord` and `after`: only when the user names the chord it belongs to, or the
     thread it must wait for.
3. Say the stub's link and its hand-off line (`Resume Atlas thread <id>`) in one line.

Ask nothing else. Write nothing else into the stub: its spec holds the detail.

## Gate

None.

## Hand off

[thread-work](../thread-work/SKILL.md), when the user wants to start the work now.
