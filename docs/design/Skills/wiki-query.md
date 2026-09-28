
# wiki-query

> Answer a question from the wiki, with citations, and say what the wiki does not cover. Reads only.

**Use for**: what does the wiki say, explain from my notes, find in the wiki, what do we know about X, which repositories use Y.

**Tools**: `search`, `context`, and Read. **References**: none.

## Procedure

1. Call `search` with the question's terms, and its scope when the question names one. For a question about repositories or areas ("which services use MQTT"), call `context` for the area.
2. Read the best pages. Follow a link one hop when the answer needs it.
3. Answer. Cite each claim with its page (`[[Page]]`), and through the page its source. Keep the wiki's claims apart from your own reasoning.
4. Say what the wiki lacks, when it lacks something the question needs. Do not fill the gap from general knowledge without saying so.

## Hand off

[[wiki-save]] when the answer is new synthesis the user wants kept. [[thread-work]] when the question turns into work.
