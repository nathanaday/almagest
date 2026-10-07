---
name: wiki-wikify
description: "Experimental. Mark a copy of the user's note with what the wiki knows: a link mark where a phrase names a document of the wiki, and a new mark where it names a subject worth a topic. The user accepts or ignores each mark in Obsidian; nothing enters the wiki by itself. Use for wikify this note, review this doc, link this draft to the wiki, what in this note does the wiki know. Drafting the topic of a new subject is wiki-edit; files to learn from are wiki-ingest."
---

# wiki-wikify

Wikify works on a copy of a note in `scratchpad/`, never on the note itself. This skill
finds the subjects that the copy's prose names, matches them against the wiki, and
writes marks into the copy with one `wikify mark` call. The Obsidian plugin shows each
mark as a bubble. The user accepts a link, ignores a mark, or asks an agent to create
the topic of a new subject.

Tools: `wikify` (start, mark), `match`, and Read. References:
[pages.md](../almagest/references/pages.md).

## Procedure

1. **The copy.** The palette sends `/almagest:wiki-wikify Wikify [[<copy title>]]:
   mark what the wiki knows and the subjects worth a topic, with wikify mark.` The link
   names the copy, `scratchpad/<copy title>.md`. The palette made the copy and opened
   it. From the chat ("wikify this note"), find the note that the user names, and ask
   when two notes match. Then call `wikify` with `action: start` and `note` set to the
   note's path in the vault. Code copies the note to `scratchpad/<name> · wikified.md`
   (`<name> · wikified (2).md` when that name is taken) and returns the path in `copy`.
   Start refuses a file that is not markdown, `Almagest.md`, and a note under
   `source-core/`, `changes/`, `sessions/`, `wiki-view/`, `trash/`, or `.obsidian/`. A
   note in `journals/` may be wikified; its copy lies in `scratchpad/`.
2. **Read** the copy with Read.
3. **Subjects.** List the subjects that the prose names, with the kinds of
   [wiki-extract](../../agents/wiki-extract.md): entity, concept, or policy. For each
   subject, write the `name` the wiki would give it, the `aliases` (the note's wording
   when it differs, and abbreviations), and a one-sentence `description` from the note.
   Code never marks text in the frontmatter, a heading, a table row, code, a link, a URL, a comment,
   or a mark, so leave out a subject that the note names only there.
4. **Match.** Put the subjects in one Item Map, `{"doc": "<the copy's path>", "chunk":
   0, "items": [...]}`, and call `match` with `items` set to that one map. Each subject
   comes back as `hit` (`page`: the document that holds its name or an alias), `near`
   (`neighbors` above the threshold, with scores), or `new` (the closest neighbors).
5. **Choose the marks.** One mark per subject, at most:
   - **hit**: a mark with `link` set to the page's id.
   - **near**: read each neighbor's description, and the document with Read when the
     description does not settle it. Mark a `link` only when the neighbor surely is the
     subject. Otherwise judge the subject as new.
   - **new**: a mark with `new` set to a title, only when the subject carries weight in
     the note (the note explains it, or relies on it) and the wiki has no topic for it.
     Mark a few new subjects per note at most: the ones with the most weight. Give each
     a title in the form of the wiki's titles: look at the titles of the hits and the
     neighbors, and use the same case and the same form (a name, not a phrase from the
     sentence). Code removes `/ \ : * ? " < > | [ ] # ^ →` from a title, and a title
     holds at most 150 bytes.
   - The `phrase` is the words of the subject's first mention, as the note writes them.
     It is one line, with no `|`, `{`, or `}`. Code finds the first mention that is free
     (whole words, without regard to case), so the phrase names the first mention, not
     a later one.
   - Never mark a phrase inside a quote of another person's words. When the first
     mention of a subject lies in such a quote, mark nothing for it, unless another
     wording of the subject comes first outside the quote.
   - Put a phrase that holds another phrase first ("learning rate schedule" before
     "learning rate"). Code places the marks in the order given, and a later phrase
     cannot land inside an earlier mark.
6. **Mark.** Call `wikify` once, with `action: mark`, `note` set to the copy's path, and
   `marks`: `[{"phrase": "…", "link": "<id>"}, {"phrase": "…", "new": "<Title>"}]`,
   exactly one of `link` and `new` in each. Code resolves each `link` to a document (by
   id, title, or alias) and writes that document's title. A `new` title that names a
   document already becomes a link to it. Code replaces each phrase with
   `{{link:<Title>|<phrase>}}` or `{{new:<Title>|<phrase>}}`, keeps the note's spelling
   of the phrase, and writes the copy once. It returns `placed`, and `missing` for the
   phrases it did not find. A refused call writes nothing: fix what the refusal names
   (a mark, or the note) and call again.
7. End with one line: the copy as a link, the count of links and of new subjects you
   marked, and each phrase in `missing`.

## Gate

The copy is the user's scratch. Never edit the original note. Never edit the copy with
Edit or Write: only `wikify mark` writes into it. Code commits nothing; the next quiet
snapshot keeps the copy. Never report the vault's git state.

Nothing enters the wiki from this skill. In Obsidian, Accept turns a link mark into a
link, and Ignore turns a mark back into its phrase. Create on a new mark starts a draft
work document and an agent ([wiki-edit](../wiki-edit/SKILL.md)) that proposes the
topic. The topic enters the wiki only when the user approves that change.

## Hand off

[wiki-edit](../wiki-edit/SKILL.md) when the user asks in the chat to draft the topic of
a new subject. [wiki-ingest](../wiki-ingest/SKILL.md) when the user wants the note
itself in the wiki as a source.
