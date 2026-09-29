# wiki-extract

> Read one chunk of one document, and return what it says as an [[Item Map]]. See [[wiki-extract.canvas|the extract flow]].

**Takes**: a document id, a chunk index, the document's tags, the tag vocabulary (every tag with its count), and the vault's `description` from `Atlas.md` (so the agent knows what matters here).

**Returns**: one [[Item Map]].

## Procedure

1. Call `source` read for the chunk. For a PDF or an image, Read the pages or the file the [[Text Blob]] names.
2. Write `summary`: what the chunk covers, in two to four sentences.
3. List the subjects the chunk says something durable about, each with its `kind`:
   - **entity**: a thing with a name: a person, an organization, a tool, a component, a service, a dataset, a document;
   - **concept**: an idea the chunk explains or relies on: a method, a theory, a pattern, a link between ideas;
   - **policy**: a rule the chunk states for how things must be done, with its reason when it gives one. In a spec, a decision is a policy only when it binds future work.
4. For each subject, write its claims: one statement each, in your words or quoted, with the locator (page, section, line).
5. Give each subject the name the document uses, and its aliases (abbreviations, other spellings).
6. Suggest tags for each subject from the vocabulary: the ones that fit. Never invent a tag.

## Rules

- Extract; do not judge the wiki. The agent does not search the wiki. [[match]] finds the topics, and [[wiki-draft]] decides.
- No claim without a locator in the chunk. No invented quotation, number, or date.
- Passing mentions are not subjects. A name that the chunk only lists, with nothing said about it, is left out.
- At most 30 subjects per chunk. Past that, keep the ones with the most claims and list the rest under `questions`.
