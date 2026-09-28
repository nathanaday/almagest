
# wiki-draft

> Decide what the wiki does with each subject of a [[Match Map]] slice, and draft the writes. The first draft called this "Wiki Draft". See [[wiki-draft.canvas|the draft flow]].

**Takes**: a slice of the Match Map (at most eight subjects), the ids of the documents being absorbed, and the scope of each.

**Returns**: the `writes` of a [[Wiki Change Plan]], and `skipped`: each subject left out, with one line on why.

## The decision, for each subject

| Match | Question | Yes | No |
|---|---|---|---|
| `new` | Should the wiki hold this subject? (the gates below) | **create** a page | **skip** |
| `near` | Read each neighbor. Is one of them the same subject? | treat it as a **hit** on that page, and add the subject's name as an alias | treat it as **new**, and link the closest neighbor under `## Related` |
| `hit` | Read the page. Do the items add information? | **modify**: merge the claims, cite them, keep the clearer description | next question |
| `hit` | Do the items confirm a claim the page makes? | **modify**: add the citation | **skip** |
| any | Do the items contradict the page? | **modify**: keep both claims with their citations, and set `status: contested` | — |

The first draft skipped a `near` subject whose neighbor was a different subject. That loses the subject, so it goes the `new` way instead.

## The gates for a new page

- **entity**: it has a name, and the document says something about it that someone would look up.
- **concept**: the document explains it, or relies on it in a way a reader must understand.
- **policy**: the document states it as a rule, and it binds work in the scope.
- A subject that fails every gate is skipped with its reason. The skill lists the skipped subjects in the change's summary, so you can ask for one.

## Writing a page

- Follow the type's schema and body sections ([[Wiki]]). Give `description` one good sentence: it is what [[search]] and [[match]] find the page by.
- Cite every claim with its document and locator ([[Wiki#Claims and citations]]).
- A new page takes the scope of the document it came from. A page that rests only on the spec of an open thread is `draft`.
- A modify gives the whole new body, and `base`, the hash of the page as read.
- Call `source` read when an item's claim needs the text around it.
