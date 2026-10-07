---
name: wiki-checkout
description: "The librarian: choose the wiki documents that serve a request, follow their links only while the documents still serve it, and check out copies with a reading list in reading order. Return proposes the user's edits of the copies as one change. Use for check out the material on X, pull together what we know about X, make me a reading stack on X, a reading list on X, return my checkout. An answer to a question is wiki-query."
---

# wiki-checkout

A checkout is a folder of `checkout/` that holds copies of wiki documents and a reading
list. The user reads and edits the copies in Obsidian. Code ranks the candidates and
writes the folder. This skill chooses the documents, puts them in reading order, and
gives each one a reason. Return turns the edited copies into one proposed change of
their originals.

Tools: `checkout` (candidates, make, list, return), and Read. References:
[changes.md](../almagest/references/changes.md).

## Procedure

1. Take the request in the user's words. The palette sends
   `/almagest:wiki-checkout Check out the material on: <request>`; the text after
   the colon is the request. When the request names a project or a category, turn the
   words that name tags of the vocabulary into `tags` (the tag list in the
   session-start context, or `vault`).
2. Call `checkout` with `action: candidates`, `text` set to the request, and the `tags`.
   Add `types: [topic, repository, source]` when the user asks for the sources
   themselves ("the papers on X"). Code ranks every document with the search, takes the
   best hits as seeds, and adds the documents linked to or from them, up to two links
   away. Each candidate has `ref` (with its description), `score`, `distance` (0 for a
   search hit), `via` (the title it was reached from), and `words` (the length of its
   body).
3. Choose. Read each candidate's description. When the description does not settle
   whether the document serves the request, read the document with Read. Keep a
   document that serves the request; drop one that does not. Judge a linked document on
   its own content, not on the hit that led to it. When you drop a document, also drop
   the candidates reached only through it, unless they serve the request on their own.
   So a branch stops where its documents stop serving.
4. Follow a branch past the list only while it serves. The candidates stop two links
   from a search hit. When a kept document links others that serve the request, read
   them and keep the ones that do.
5. Size the checkout for a sitting or a week of reading: at most about 30 documents.
   `words` tells you how long each one is. When more than about 30 documents serve, the
   request is too broad: ask the user once to narrow it, and offer two or three narrower
   requests from the tags or subjects of the candidates. After the answer, make the
   checkout.
6. Order the documents foundations first: a definition, an overview, or a concept that
   the others assume comes before the documents that build on it. Give each document a
   `why`: one line that says what it gives the reader for this request.
7. Call `checkout` with `action: make`, and:
   - `request`: the user's words;
   - `name`: a short name for the folder, such as "Reinforcement learning";
   - `documents`: `[{id, why}]` in reading order;
   - `notes`: what you left out and why: the near subjects you dropped, the branches
     you stopped, and a gap the wiki has.

   Code writes `checkout/<date> <name>/` in one commit: a copy of each document as
   `<Title> (checkout).md`, and the checkout's index `_index.md` (its name, request,
   status, and reading order). The ledger `checkout/Checkout · Ledger.md` is a Base of
   every checkout's index. A link between two documents of the checkout points at the
   copy; every other link points at the wiki.

## Return

When the user asks to return a checkout:

1. Find its folder. When the user does not name it, call `checkout` (`action: list`)
   and take the checkout with `status: out` whose request matches. When two match, ask
   which, and name both.
2. Call `checkout` with `action: return` and `folder`. Code makes one `modify` write for
   each edited copy, with the copy's text and its links pointed back at the originals,
   and proposes the change "Return <name>". Then it moves the checkout, every file of
   it, to `tool/returned/<folder>/`, marks its index returned, and keeps it there as
   the user left it. Return carries a copy's body only; an edit of a copy's frontmatter
   does not return.
3. Show the result in one or two lines: the change document as a link, when there is
   one, the checkout's place now, and each copy in `skipped` with its reason. A copy is
   skipped when its original is gone or changed since the checkout; its edits stay in
   the returned copy. With no edited copy, there is no change: the checkout just moves.
4. A checkout returns once. When the tool says it was returned already, tell the user to
   check the documents out again to work on them further.

## Gate

None for `make`: a checkout changes no knowledge, and the copies are the user's. Never
edit a file in `checkout/` or `tool/returned/`; the guard refuses it. Return proposes a change, and the user
decides in the change document (Approve or Cancel). Never apply it: the change names no
session, so the `change` tool refuses your apply.

End with one or two lines that name the checkout's index as a link
(`[[checkout/<folder>/_index|<name>]]`, as `make` returns it in `index`) and the count of
documents.

## Hand off

[wiki-query](../wiki-query/SKILL.md) for a question about the material.
[wiki-edit](../wiki-edit/SKILL.md) to carry the edits of a skipped copy into the wiki,
or for any other change to the originals.
