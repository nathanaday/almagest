# wiki-query

> Answer a question from the wiki, with citations, and say what the wiki does not cover. Reads only.

**Use for**: what does the wiki say, explain from my notes, find in the wiki, what do we know about X, which repositories use Y.

**Tools**: `search`, `context`, and Read. **References**: none.

## Procedure

1. Turn the question into a query. Words that name tags in the vocabulary become `tags` ("my cs513 self-driving project" → `tags: [school/cs513, self-driving, project]`); the rest is `text`. Add `types` or `kinds` when the question asks for one ("which policies…").
2. Call `search`. When the hits are many, read `facets.tags` and add the tag that splits them; when they are none, drop the least certain tag and search again. Say which tags you used.
3. For a question about repositories ("which services use MQTT"), call `context` with the tags, and read the tag pages' `## Context`.
4. Read the best documents. Follow a link one hop when the answer needs it.
5. Answer. Cite each claim with its document (`[[Topic]]`), and through the topic its source. Keep the wiki's claims apart from your own reasoning.
6. Say what the wiki lacks, when it lacks something the question needs. Do not fill the gap from general knowledge without saying so.

## Hand off

[[wiki-save]] when the answer is new synthesis the user wants kept. [[spec-work]] when the question turns into work.
