# search

> Ranked search over the typed documents in `wiki/documents/`. Takes a [[Query]], returns [[Hits]].

One search serves every need: finding a topic, finding the repository a request means, finding the plan for a piece of work, and finding the documents at the meeting of several tags.

- The candidates are the typed documents in `wiki/documents/`. Files with no type, `inbox/`, `scratchpad/`, `views/`, and your own notes are not searched. Sessions and changes are searched only when `types` names `session` or `change`, and `## Writes` of a change is never ranked, because it holds copies of documents.
- `types`, `kinds`, and `status` are lists: a document matches when its value is in the list.
- `tags` is a list, and a document matches when it holds every tag in it ([[Documents#What a tag reaches]]). So `tags: [school/cs513, self-driving]` finds the documents at the meeting of the two, and `tags: [school]` finds everything under `school`.
- `repository` limits the hits to the documents that name the repository (in `repositories`, or as `subject` of an event of such a spec), or that hold its tag.
- Ranking: BM25 over title (weight 3), aliases (3), tags (2), description (2), and body (1). A tag's words count as text, so "cs513 self-driving project" ranks the documents that hold those tags even when the query sets no filter. An empty `text` with filters lists the matches, newest `updated` first.
- `facets` in the Hits count the tags, types, and statuses among all the matches before the limit. An agent narrows a broad query with them: it adds the tag that splits the hits, as you would in the [[Obsidian Plugin#Tag navigator|tag navigator]].

The server keeps no index file. It reads the documents per call and may cache parsed files in memory by modification time.

Refusals: a `types`, `kinds`, or `status` value that no type has; a tag that breaks [[Documents#Form]] (an unknown but valid tag returns no hits, and says so).
