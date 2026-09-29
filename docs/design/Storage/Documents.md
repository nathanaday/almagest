# Documents

Every typed document shares one schema: a set of fields that every document has, and a set that its type adds. This page gives the common schema, the tags, and the rules every type obeys. Each type's page gives its own fields and the layout of its body.

## The types

| Type | Page | Kinds | Written by | Pending by default |
|---|---|---|---|---|
| `source` | [[Source Document]] | — | `source` capture; then `change` | yes, until an ingest absorbs it |
| `repository` | [[Repository]] | — | `change`; code keeps the git facts | no |
| `topic` | [[Topic]] | `concept`, `entity`, `policy`, `overview` | `change` | no |
| `stub` | [[Stub]] | — | `work` | no |
| `spec` | [[Spec]] | `plan`, `design` | `work` | yes, when its prose changed since the wiki last absorbed it |
| `event` | [[Event]] | `started`, `continued`, `completed`, `dropped`, `reopened`, `blocked`, `unblocked`, `promoted`, `resolved`, `note` | code | `completed`, `dropped`, and `note` |

Families:

- **Knowledge**: source, repository, topic. What the vault knows. Only an applied change writes one ([[Changes]]).
- **Work**: stub, spec. What you mean to do. The `work` tool writes them, one commit per call, and the model and you edit their prose.
- **Record**: event. What happened. Code writes it when a document changes state.

Two record types live outside `wiki/documents/` with schemas of their own: the [[Sessions|session]] and the [[Changes|change]].

## The common fields

```yaml
---
id: doc-k3m9qa                      # code; never changes, even when the type does
type: topic                         # code; changes only when a stub becomes something in place
description: "Training a model on data with no labels, from a signal in the data itself."
tags: [ml/self-supervised, vision]  # the categories; any number
aliases: [SSL]                      # other names; unique in the vault like titles
created: 2026-09-27T14:51:03        # code
updated: 2026-09-29T09:15:40        # code
refreshed: 2026-09-29T09:15:40      # code; see below
---
```

| Field | Owner | Rule |
|---|---|---|
| `id` | code | `doc-` and six characters of lowercase Crockford base32 (digits and letters, without `i`, `l`, `o`, `u`). Code mints it and checks that no document holds it. A document from a 6.x vault keeps its old prefix (`con-`, `tsk-`, …), because an id never changes ([[Migration#Ids]]). |
| `type` | code | one of the six. The model names the type when it creates a document; after that only a promotion changes it ([[Stub#Becoming something]]). |
| `description` | the model; you | one sentence, required, at most 200 characters. Search and match rank on it, and every view shows it. A document without a good description cannot be found. |
| `tags` | the model; you | the categories, as below. May be empty. |
| `aliases` | the model; you | other names. Each is unique in the vault, like a title. |
| `created` | code | the time the document was written first. |
| `updated` | code | the time a tool, a hook, or a hand edit last changed the file. The touched hook and sync set it after an edit. |
| `refreshed` | code | the time the document was last checked against what it describes. |

- The file name is the title. There is no `title` or `name` field.
- Times are local, to the second, in the form Obsidian reads as a date and time: `2026-09-29T14:32:05`.
- Other fields are the type's. A field that no schema names is yours. Code keeps it and never reads it.

### refreshed

`refreshed` answers "how current is this?" The meaning follows the type:

| Type | Set to the time when |
|---|---|
| source | capture; the original never changes |
| repository | sync last found new git facts: a new head or branch ([[Repository#Git facts]]) |
| topic | a change last modified it, or confirmed it with no edit (`op: confirm`, [[Changes#Operations]]) |
| stub, spec | its last event, or its creation |
| event | its time |

Views sort by it, and lint reports a topic whose cited sources, specs, or repository changed after it was refreshed ([[Findings]]).

## Tags

The `tags` property holds the document's categories. It is Obsidian's own property, so the tag pane, the search operator `tag:`, the graph's tag filter, and `file.hasTag()` in Bases work on it with no code.

### Form

- A tag is lower case: letters, digits, `-`, and `/`. It holds at least one letter. No `#` in the property.
- `/` nests a tag: `school/cs513` is a child of `school`. A document that holds `school/cs513` belongs to `school` too. A lookup for `school` finds it.
- Code normalizes a tag the model gives: lower case, spaces and `_` made `-`, a leading `#` removed. It refuses a tag that is still not valid, and names the rule.
- Tags in the body (`#todo` in a task line) are markers, not categories. Obsidian shows both kinds in its tag pane, but code reads only the `tags` property as categories. The work view lists the `#todo` lines ([[Views#Work]]).

### The vocabulary

The tags that exist are the union of every document's `tags`. There is no list to keep. `vault` status returns each tag with its count, and the session-start context prints the most used.

- In `tags: open` mode, the model may add a tag that no document holds. The preview of the write names each new tag.
- In `tags: known` mode, `change` and `work` refuse a tag that no document holds, unless the call sets `new_tags: true`. The skills set it only after the user agrees in the chat.

### Tag pages

A tag may have a page: a topic of kind `overview`, or a repository, that declares the tag in `defines`:

```yaml
defines: school/cs513
```

- At most one document defines a tag. `change` refuses a second, and names the first.
- A document that defines a tag holds that tag's parent in `tags`, when it has one: the page of `school/cs513` holds `school`.
- The page holds what the tag means, and the context an agent needs to work under it. The views and the [[context]] tool show the page with the tag.
- A repository defines the tag of its own knowledge: `p3-edge` defines `work/p3/p3-edge`, and the pages about its code hold that tag ([[Repository#Its tag]]).
- A tag without a page is valid. [[wiki-map]] writes the pages that are worth writing.

### What a tag reaches

A document **holds** a tag `t` when its `tags` or its `defines` list `t` or a tag below `t`.

- **Search** with `tags: [a, b]` returns the documents that hold `a` and hold `b`.
- **Policy**: a policy topic applies to a repository when the repository holds every tag in the policy's `tags`. A policy with no tags holds everywhere. A policy for one repository holds that repository's own tag ([[Topic#Policy]]).
- **Context**: the [[context]] of a repository gives the pages of every tag it holds, and every tag above them.

### Renaming a tag

A tag is renamed through a change (`op: retag`), because it rewrites many documents. Code rewrites the tag in `tags` and `defines` of every typed document, and every inline `#tag` in every markdown file, your notes too. The children move with it: `p3` → `work/p3` also makes `p3/edge` into `work/p3/edge`. A retag to a tag that exists merges the two. See [[Changes#Tag rewrites]].

## Lead callouts

The first callout of a document is code's when its type is one of the lead types: `source`, `repository`, `topic`, `stub`, `spec`, `event`. It is the document's card: what the document is, its state, and its links to the documents around it. Each type's page shows its callout. Sync writes the callout again when a fact in it changes, and writes the file only when the content differs.

- The callout type names the document type or kind (`> [!spec]`, `> [!policy]`), so the Obsidian plugin can give each its color and icon. Without the plugin, Obsidian draws a default callout, and the text reads the same.
- Every other callout is yours.
- Code finds the lead callout as the first block of the body, when it is a callout of a lead type.

## The layout of a body

Each type's page lists the sections of its body, in order. The rules for every type:

- The lead callout comes first.
- Sections are `##` headings with fixed names. A section with nothing to say may be left out, unless the type's page marks it required.
- `## Notes` is the last section of every type. It is yours: the model adds to it only when you ask.
- A section that code writes says so on the type's page. The model does not edit it; the guard refuses an Edit inside it.

## What a schema says

Each type's page gives:

1. its fields, with the owner of each;
2. its lead callout;
3. the sections of its body, in order;
4. the rules that `change`, `work`, or `lint` enforce.

A document that breaks its schema is a finding of [[lint]], never an error that stops a tool. A tool refuses only to write a document that breaks its schema.

## Status

Every type but `repository` and `event` has a `status`, and each type's page defines its values. The work types derive theirs:

| Type | `status` | Set by |
|---|---|---|
| source | `pending`, `absorbed` | code, from the applied changes ([[Changes#Pending documents]]) |
| topic | `draft`, `stable`, `contested`, `deprecated` | the model, in a change |
| stub | `open`, `resolved`, `dropped` | code, from its last lifecycle event |
| spec (plan) | `open`, `started`, `done`, `dropped` | code, from its last lifecycle event |
| spec (design) | `current`, `superseded` | code: `superseded` when a later design spec names it in `supersedes` |

`search` and the views filter on it.
