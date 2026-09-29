# Atlas 7

Status: design, 2026-09-29. The build is 6.5.0. This revision replaces the 6.x design. Its input is [[Atlas 7 Brainstorm]].

This page is the entry to the specification. It states what Atlas is, the parts and their jobs, the rules every part obeys, and the decisions behind them. See [[#Reading order]] for the other pages.

## What Atlas is

One Obsidian vault holds every document of your work: what you know, what you plan and do, and what happened. Each document is a markdown file with a type, an id, a description, and tags. No database or index sits behind the files. Linking a repository writes a repository document. Planting an idea writes a stub. Finishing work writes an event.

You start an agent in the vault. The agent finds the documents and the repository you mean by their tags and descriptions, works in plain view, and leaves a document for each step. You open, read, and edit every one of them in Obsidian.

The pitch, from [[Atlas V1 Feedback]]: multitasking with agents fails in two ways. You delegate and lose track of the work, or you spend your energy on keeping documents current. Atlas is for people who want to control the documentation, the design, and the deliverables of long work, while agents do the typing.

## Three jobs, three owners

Atlas does three jobs. Each job has one owner, and no owner does another's job.

| Job | Holds | Owner | Does not |
|---|---|---|---|
| **Storage** | `wiki/documents/`, `wiki/assets/`, `sessions/`, `changes/`, `Atlas.md` | the binary's write tools (`change`, `work`, `source`) and the hooks | arrange files for people to browse |
| **Lookup** | nothing on disk | the binary's read tools (`search`, `context`, `match`, `lint`, `vault`), the skills, and the agents | keep an index; write a document |
| **Navigation** | `views/`, derived and untracked | the binary's view writer (`vault sync`), and the Obsidian plugin, which renders and runs it | hold a fact that is not in a document |

- Storage is flat. Every typed document is a file in `wiki/documents/`. Its folder says nothing about it. Its type, its tags, and its links say everything.
- Lookup reads the documents on each call. The filters are the fields every document has: type, tags, status, and the text of the title, the aliases, the description, and the body.
- Navigation is a set of views that code writes from the documents. A view is never a copy of a document. It is a way to find the document you want to open. Delete `views/` and the next sync writes it again, and no fact is lost.

## Rules

A rule that must hold lives in a tool or a hook, never only in a skill.

1. **Everything is a document.** Atlas keeps every fact as a markdown file with a schema, in the vault. Five exceptions: `.git/` (history, and the lock); `.obsidian/` (the app settings and the Obsidian plugin); `views/` (derived from the documents, and out of git); `.claude/settings.local.json` (untracked; code lists the linked repositories there); and one machine file, `~/.atlas/config.json`, that lists the paths of your vaults.
2. **Every document has an id, a type, a description, and tags.** Code mints the id. The file name is the title. A title is unique in the vault, so a bare `[[wikilink]]` has one target. See [[Documents]].
3. **Storage is flat, and navigation is derived.** No tool files a document by folder. Every view that groups documents is written by code from their fields ([[Views]]).
4. **Code owns what code can derive.** Ids, dates, the status of a stub or a spec (from its events), a source's hash, a repository's git facts, lead callouts, the pending list, the link and tag rewrites of a rename, and the views. The model writes prose and makes judgments. It never writes a derived field.
5. **Knowledge changes only through a change.** A topic, a source, or a repository changes only when an applied [[Changes|change]] writes it: the model proposes, the user reviews in the chat or in Obsidian, and apply makes one git commit. The work documents (stub, spec, event) are written by the `work` tool, one commit per call, with no review.
6. **Work on a repository needs a started spec.** A hook refuses an edit inside a linked repository while the session works on no started spec that covers it. Questions and exploration need no spec.
7. **Every session has a document.** Hooks create it and keep its status and links. The agent adds a description, its progress, and a summary.
8. **Tools take and return entities.** A tool is a fact or a commit. A skill is a procedure and a policy. An agent is a read-only worker that a skill sends. See [[Entities]].
9. **The folder is the user's.** Before a tool writes, it commits your hand edits as a `snapshot` commit. An undo restores only the paths of one change, and refuses when you edited one of them since.

## The model

```text
wiki/documents/
  source       ◀── sources ─── topic ─── defines ──▶ tag
  repository   ─── defines ──▶ tag                     ▲
  topic                                                 │ tags (any number, on every document)
  stub         ─── became ──▶ any document              │
  spec         ─── parent ──▶ spec
               ─── repositories ──▶ repository
  event        ─── subject ──▶ stub, spec, or any document
               ─── session ──▶ session

sessions/  session ─── specs ──▶ the specs it started
changes/   change  ─── absorbs ──▶ the documents the wiki learned from
```

Six document types live in `wiki/documents/`:

| Type | Is | Written by |
|---|---|---|
| [[Source Document]] | an outside document the vault captured, with the original in `wiki/assets/` | `source` capture, then `change` |
| [[Repository]] | a git repository on this machine that agents may work in | `change`; code keeps its git facts |
| [[Topic]] | an article: a concept, an entity, a policy, or the overview of a tag | `change` |
| [[Stub]] | an idea planted in a hurry, that may become anything | `work` |
| [[Spec]] | a plan for work in repositories, or the design of how something must work | `work` |
| [[Event]] | a record of something that happened to a document | code, through `work` and `change` |

Two record types stay outside: [[Sessions|session]] documents in `sessions/` and [[Changes|change]] documents in `changes/`.

### Tags

A tag is a category. Every document holds any number of tags in Obsidian's own `tags` property, so a document about the self-driving project of the course CS513 holds `school/cs513`, `self-driving`, and `project`, and a lookup for that project asks for the three at once.

- A nested tag (`school/cs513`) belongs to its parent too. A lookup for `school` finds it.
- A tag may have a page: the one topic or repository that declares `defines: school/cs513`. The page holds what the tag means and the context an agent needs to work under it. A tag works without a page.
- A policy applies to a repository when the repository holds every tag the policy holds.
- `Atlas.md` sets how freely the agent adds tags: `open`, or `known` (it uses the tags that exist and asks before it adds one).

See [[Documents#Tags]].

### Work

A stub is the first form of anything: an idea, a question, a paper to read, a bug. It becomes something else in one of two ways ([[Stub#Becoming something]]):

- **in place**: it becomes one spec or one topic, and keeps its id, its file, and every link to it;
- **by spawning**: it becomes several documents, links each of them, and closes as `resolved`.

A spec plans work. A spec may have child specs, to any depth. A leaf spec is one piece of work in one repository that a reviewer can check alone. Work on a spec starts, continues, and completes, and each step is an event. The status of a spec is its last lifecycle event, so no field can disagree with the history. See [[Spec]] and [[Event]].

### Keeping the wiki in sync

Every document that can teach the wiki (a source, a spec, a completion event) is **pending** until an applied change records its content hash. The `vault` tool lists the pending documents, and the [[wiki-sync]] skill absorbs them through one pipeline:

```text
document → chunks → Text Blob → (wiki-extract) → Item Map → match → Match Map → (wiki-draft) → Wiki Change Plan → change → commit
```

## From 6.5

| 6.5 | 7.0 | Why |
|---|---|---|
| a folder per scope under `wiki/`, a type folder per scope | one flat `wiki/documents/`; tags | a folder holds a document in one place; tags hold it in many |
| areas and the scope chain | nested tags and tag pages | a document belongs to several categories at once |
| `wiki/sources/files/` | `wiki/assets/` | the originals and your attachments in one place |
| concept, entity, policy | a topic with `kind` | one article type; the kind sets the sections |
| threads: stub, spec, task, receipt, in a folder per thread | stub, spec (nested), event | work is documents in the wiki like the rest; any document can start as a stub |
| stage from the furthest document | status from the last lifecycle event | the history is the record |
| receipt | the `completed` event | a record of what happened is an event |
| `thread` tool | `work` tool | the documents are no longer a thread |
| `Threads.base`, `Threads.canvas`, `Wiki.base` | `views/`: tag tree, work, timeline, documents | navigation apart from storage |
| scope folders in the file explorer | `views/tags/`: a folder per tag, a view note per folder | the tree is derived, so a document never moves |
| `areas: many \| few \| manual` | `tags: open \| known` | the setting governs what now categorizes |

Kept: the change document and its gate; the one ingest pipeline; unique titles and link rewrites; code-owned lead callouts; the immutable captured copy; session documents and the hooks; the read-only workers; the guard; lint; the graph colors.

## Decisions

| Question | Decision | Why |
|---|---|---|
| A `name` field? | No. The file name is the title. `Atlas.md` keeps `name`, since its file name is fixed. | one record of the title |
| A `categories` field? | No. Obsidian's `tags` property. | the tag pane, tag search, the graph's tag filter, and `file.hasTag()` in Bases work with no code |
| The id prefix | `doc-` for every document | a stub may become a spec or a topic in place, so a prefix by type would lie |
| Date format | `2026-09-29T14:32:05`, local time | Obsidian reads it as a date and time, so Bases sort and compare it |
| `last-refreshed` | `refreshed`: when the document was last checked against what it describes | one word; the meaning differs by type ([[Documents#refreshed]]) |
| Is `opened` an event? | No. `created` records it, and the timeline reads it. | half the event files, no lost fact |
| Where does a spec's result go? | its `completed` event | the event is the receipt |
| Where does the live state of a repository go? | commit-level facts in the frontmatter; the working tree's state rendered live by the plugin | no commit for every file you save in a repository |
| Is `views/` in git? | No. `.git/info/exclude` holds it. | derived; it would add a snapshot to most writes |
| A view note's title | `Tag · school › cs513`; no document title may begin with `Tag · ` or `View · ` | a view never takes a document's link target |
| Canvas board | dropped in 7.0 | the views and the graph do its job; see [[Obsidian Plugin#Later]] |

## Open questions

1. **Repository paths across machines.** A vault synced by git to a second machine holds paths of the first. Proposal: `path` may name a path per host (`paths: {host: path}`), and `remote` identifies the repository.
2. **Codex hooks.** [[Hooks]] lists what each host must supply. Codex is not verified.
3. **Tag sprawl.** `known` mode stops the agent from adding tags. Whether `open` mode needs a merge suggestion (two tags on the same documents) is to be measured in a real vault. [[wiki-review]] reports near-duplicate tags as a start.
4. **What is pending by default.** Sources, specs, and completion events. `wikify` in `Atlas.md` adds stubs or other events.
5. **Search quality.** BM25 over title, aliases, tags, description, and body, computed per call. Measure before adding more.

## Reading order

1. [[Atlas V1 Feedback]], then this page.
2. Storage: [[Vault Layout]], [[Documents]], then the six types ([[Source Document]], [[Repository]], [[Topic]], [[Stub]], [[Spec]], [[Event]]), [[Sessions]], [[Changes]], [[Migration]].
3. Lookup: [[Entities]], [[Tools]], [[Agents]], [[Hooks]], [[Skills]].
4. Navigation: [[Views]], [[Obsidian Plugin]].
5. [[Stack]]: the packages and the build order.
