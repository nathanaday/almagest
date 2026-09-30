# wiki-draft

> Decide what the wiki does with each subject of a [[Match Map]] slice, and draft the writes.

**Takes**: a slice of the Match Map (at most eight subjects), the ids of the documents being absorbed, their tags, and the tag vocabulary with the `tags` mode of the vault.

**Returns**: the `writes` of a [[Wiki Change Plan]], `skipped` (each subject left out, with one line on why), and `new_tags` (each tag a write uses that no document holds).

## The decision, for each subject

| Match | Question | Yes | No |
|---|---|---|---|
| `new` | Should the wiki hold this subject? (the gates below) | **create** a topic | **skip** |
| `near` | Read each neighbor. Is one of them the same subject? | treat it as a **hit** on that topic, and add the subject's name as an alias | treat it as **new**, and link the closest neighbor under `## Related` |
| `hit` | Read the topic. Do the items add information? | **modify**: merge the claims, cite them, keep the clearer description | next question |
| `hit` | Do the items confirm a claim the topic makes? | **modify**: add the citation | **skip** |
| any | Do the items contradict the topic? | **modify**: keep both claims with their citations, and set `status: contested` | — |

A `near` subject whose neighbor is a different subject goes the `new` way, so no subject is lost without a reason.

## The gates for a new topic

- **entity**: it has a name, and the document says something about it that someone would look up.
- **concept**: the document explains it, or relies on it in a way a reader must understand.
- **policy**: the document states it as a rule, and it binds work under its tags.
- A subject that fails every gate is skipped with its reason. The skill lists the skipped subjects in the change's notes, so you can ask for one.

## Writing a topic

- Follow the kind's sections ([[Topic]]). Give `description` one good sentence: it is what [[search]] and [[match]] find the topic by.
- Cite every claim with its document and locator ([[Topic#Claims and citations]]).
- Pick the tags from the vocabulary: the absorbed document's tags that fit the subject, and the tags the extractor suggested. Use a tag no document holds only when none fits, and list it in `new_tags`; in `tagging: known` mode the skill asks the user first.
- A policy's tags set its reach ([[Topic#Policy]]): give it the tags of the repositories it binds, no more.
- A topic that rests only on a plan that is not done is `draft`: an intent, not yet a fact.
- A modify gives the whole new body, and `base`, the hash of the topic as read.
- Call `source` read when an item's claim needs the text around it.
