---
name: wiki-query
description: "Answer a question from the wiki, with citations, and say what the wiki does not cover. Reads only. Use for what does the wiki say, explain from my notes, find in the wiki, what do we know about X, which repositories use Y."
---

# wiki-query

The wiki holds what the vault knows, each claim cited. This skill finds the pages that
answer a question, reads them, and answers with a citation for each claim, keeping the
wiki's claims apart from your own reasoning.

Tools: `search`, `context`, and Read.

## Procedure

1. Call `search` with the question's terms, and with `scope` when the question names
   one. For a question about repositories or areas ("which services use MQTT"), call
   `context` for the area, then search inside it.
2. Read the best pages with Read. Follow a link one hop when the answer needs it.
3. Answer. Cite each claim with its page (`[[Page]]`), and through the page its source.
   Keep the wiki's claims apart from your own reasoning, and mark the reasoning as
   yours.
4. Say what the wiki lacks when the question needs something it does not hold. Do not
   fill the gap from general knowledge without saying so.

## Gate

None. The skill writes nothing.

## Hand off

[wiki-save](../wiki-save/SKILL.md) when the answer is new synthesis the user wants kept.
[thread-work](../thread-work/SKILL.md) when the question turns into work.
