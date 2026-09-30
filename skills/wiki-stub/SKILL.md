---
name: wiki-stub
description: "Plant a stub from a sentence, in the user's words, with no questions. A stub may become anything later: a plan, a topic, a source. Use for note this, stub this, remember to, idea for later, read this someday, report a bug, add a todo. Keeping an answer from this conversation is wiki-save; starting the work now is spec-work."
---

# wiki-stub

A stub is an idea planted in a hurry, loose by design. It keeps the user's words as
they gave them. The questions come later, when the stub becomes something: a spec
through spec-write, a topic through wiki-edit, a source through wiki-ingest.

Tools: `search`, `work` stub. References: [work.md](../atlas/references/work.md).

## Procedure

1. `search` the open stubs and plans for the same idea: `types: [stub, spec]`,
   `status: [open, started]`, and the request's words as `text`. When one matches,
   offer to add the new words to its `## Notes` (with Edit), and stop if the user
   agrees.
2. Call `work` with `action: stub`:
   - `text`: the user's words, as given; they become the stub's `## Idea`;
   - `title`: a short name for the idea;
   - `tags`: the tags that exist and fit what the request names or the conversation is
     about, from the tag list in the session-start context; or none. Use a tag that
     exists before you make a new one. In `tagging: known` mode, a new tag needs
     `new_tags: true`, which needs the user's yes, so leave the new tag out instead;
   - `priority`: only when the user says one (high, normal, low, or someday).
3. Say the stub's link in one line.

Ask nothing else. A stub is loose by design.

## Gate

None.

## Hand off

[spec-work](../spec-work/SKILL.md), when the user wants to start work now.
