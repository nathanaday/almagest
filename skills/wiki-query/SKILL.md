---
name: wiki-query
description: "Answer a question from the wiki, with citations, and say what the wiki does not cover. Reads only. Use for what does the wiki say, explain from my notes, find in the wiki, what do we know about X, which repositories use Y. Keeping the answer is wiki-save."
---

# wiki-query

The wiki holds what the vault knows, each claim cited. This skill turns a question into
a search with tags, reads the documents that answer it, and answers with a citation
for each claim, keeping the wiki's claims apart from your own reasoning.

Tools: `search`, `context`, and Read.

## Procedure

1. Turn the question into a query. Words that name tags in the vocabulary (the tag
   list in the session-start context, or `vault`) become `tags`: "my cs513
   self-driving project" → `tags: [school/cs513, self-driving, project]`. The rest is
   `text`. Add `types` or `kinds` when the question asks for one ("which policies…" →
   `types: [topic]`, `kinds: [policy]`).
2. Call `search`. A document must hold every tag, or a tag below it. When the hits are
   many, read `facets.tags` and add the tag that splits them; `facets.types` and
   `facets.status` narrow the same way. When there are no hits, drop the least certain
   tag and search again. Say which tags you used.
3. For a question about repositories ("which services use MQTT"), call `context` with
   the `tags`, or with `repository` for one repository. Read the tag pages' `## Context`
   in `pages`, and the `repositories` and `policies` it lists.
   When the user wants to browse a tag rather than ask, link the tag's page (its
   overview topic or repository page): it lists the documents under the tag.
4. Read the best documents with Read. Follow a link one hop when the answer needs it.
5. Answer. Cite each claim with its document (`[[Topic]]`), and through the topic its
   source. Keep the wiki's claims apart from your own reasoning, and mark the reasoning
   as yours.
6. Say what the wiki lacks, when it lacks something the question needs. Do not fill the
   gap from general knowledge without saying so.

## Gate

None. The skill writes nothing.

## Hand off

[wiki-save](../wiki-save/SKILL.md) when the answer is new synthesis the user wants kept.
[thread-work](../thread-work/SKILL.md) when the question turns into work.
