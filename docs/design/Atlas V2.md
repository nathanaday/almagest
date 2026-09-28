
> [!important] A new project
> Atlas V2 is built from scratch. It takes lessons and select components from Atlas V1 as a guide. It keeps no backward compatibility. For all purposes it is a new plugin.

# Atlas V2

Status: design, revision 2, 2026-09-27. Nothing is built.

This page is the entry to the specification. It states what Atlas is, the rules every part obeys, and the decisions behind them. The other pages under `design/` hold the details. See [[#Reading order]].

## What Atlas is

One Obsidian vault holds every document of your work:

- what you know: the **wiki**;
- what you do: the **threads**;
- what your agents do now and did before: the **sessions**;
- every change an agent made to what you know: the **changes**.

You start an agent in the vault. The agent finds the repository you mean through the **context graph** (vault, areas, repositories) and works on it in plain view. Every step leaves a document that you can open, read, and edit in Obsidian.

The pitch, from [[Atlas V1 Feedback]]: multitasking with agents fails in two ways. You delegate and lose track of the work, or you spend your energy on keeping documents current. Atlas is for people who want to control the documentation, the design, and the deliverables of long work, while agents do the typing.

## Rules

Every part of Atlas obeys these rules. A rule that must hold lives in a tool or a hook, never only in a skill.

1. **Everything is a document.** Atlas keeps every fact as a markdown file with a schema, in the vault. There is no database, no state folder, and no plan held in memory between calls. Four exceptions: `.git/` (history, and the lock that apply takes); `.obsidian/` (the app settings and the Obsidian plugin); `.claude/settings.local.json`, an untracked file where code lists the linked repositories so an agent may edit them; and one machine file, `~/.atlas/config.json`, that lists the paths of your vaults.
2. **Every document has an id and a type.** Code mints the id. The file name is the document's title, and a title is unique in the vault, so a bare `[[wikilink]]` has one target.
3. **Code owns what code can derive.** Ids, dates, a thread's stage, a session's status and links, a source's hash, a repository's git facts, the lead callout of a document, the list of documents the wiki has not absorbed, and the link rewrites of a rename. The model writes prose and makes judgments. It never writes a derived field.
4. **The wiki changes only through a change.** A change is a document. The model proposes it, the user reviews it in the chat or in Obsidian, and apply makes one git commit. A hook refuses Write and Edit under `wiki/`, and refuses apply until the user has had a turn since the proposal. The one write that crosses into another folder is code's link rewrite after a rename, in the rename's commit.
5. **Work on a repository needs a thread.** A hook refuses an edit inside a linked repository while the session has no thread. Questions and exploration need no thread.
6. **Every session has a document.** Hooks create it and keep its status and links. The agent adds a description, its progress, and a summary.
7. **Tools take and return entities.** A tool is a fact or a commit. A skill is a procedure and a policy. An agent is a read-only worker that a skill sends. Each tool names the [[Entities|entity]] it takes and the entity it returns.
8. **The folder is the user's.** Before apply writes, it commits your hand edits as a `snapshot` commit. An undo restores only the paths of one change, and refuses when you edited one of them since.

## The model

```text
Vault ─┬─ Area ─┬─ Area ── Repository        the context graph (scope pages in the wiki)
       │        └─ Repository
       └─ Repository

Wiki pages ── scope ──▶ a vault, area, or repository
Threads    ── scope ──▶ the repositories and areas the work touches
Sessions   ── thread ─▶ the thread the session works on
Changes    ── sources ▶ the documents a change absorbed into the wiki
```

- [[Document Types]] lists the thirteen document types.
- [[Vault Layout]] gives the layout of the vault, ids, titles, links, and git.
- [[Entities]] lists the data that tools take and return.
- [[Tools]], [[Agents]], [[Skills]], and [[Hooks]] list the parts that act.
- [[System Map.canvas|System Map]] shows every part on one canvas.
- [[AgentFlow/Agent Session.canvas|Agent Session]] shows a session from start to end as a flow chart.

### The context graph

There is one vault. It is the highest namespace, and it is self-contained: nothing connects two vaults. Keep work and personal notes in separate vaults when they must never meet.

A vault holds any number of **areas**, nested to any depth, and zero areas is valid. An area is a label for a cluster of repositories or other areas. The leaf of the graph is always a **repository**: a git repository anywhere on the disk, with its own `AGENTS.md` or `CLAUDE.md`.

The graph is not a folder tree. Each area and each repository is a page in the wiki with a `parent` property. Obsidian's graph view shows the tree from those links. An agent walks the graph with the `context` tool:

| The user asks | The agent walks |
|---|---|
| "Work on the p3 cloud front end" | vault → work → p3 → p3-cloud |
| "Change the remote update system in every p3 service" | vault → work → p3 → p3-cloud, p3-edge, p3-vertex |
| "Check every repository for CVE XYZ" | vault → every area → every repository |

Because repositories live outside the vault, the vault's git repository never contains another one. This removes the nested `.git` problem of V1.

### One wiki, organized by scope

The vault has one wiki. Each page has a `scope`: the vault, an area, or a repository. Scope is a property, not a folder, so moving knowledge up the graph changes one property and breaks no link.

The wiki is mapped from the leaves up:

1. Map each repository in isolation (`repo-ingest`).
2. In each parent, compare the children (`wiki-rollup`): make a **bridge** page that links similar ideas of two children, or **upgrade** a page from a child to the parent.
3. Repeat to the top.

### Keeping the wiki in sync

Every document that can teach the wiki (a source, a spec, a receipt) is **pending** until an applied change lists it. Code derives this. It compares each document's content hash to the hashes that applied changes recorded. The `vault` tool reports the pending documents, and the `wiki-sync` skill absorbs them through one pipeline:

```text
document → chunks → Text Blob → (wiki-extract) → Item Map → match → Match Map → (wiki-draft) → Wiki Change Plan → change → commit
```

Ingest, saving a conversation, describing a repository, and learning from a closed thread all use this pipeline. See [[wiki-sync.canvas|the wiki-sync flow]].

## From V1

| V1 | V2 | Why |
|---|---|---|
| a project per repository, each with its own wiki | one vault, one wiki, scope pages | fewer places to look; no link to maintain |
| members, mirrors, hubs | `wiki-rollup`: bridge and upgrade inside one wiki | mirrors worked but were hard to follow |
| stub, spec, plan, receipt; phases | stub, spec, tasks, receipt; no phases | one spec spawns many tasks; phases wait for a better idea |
| a plan held in the MCP server's memory | a change document | the user reviews it in Obsidian; it survives a restart |
| `source-ledger.json`, `.raw/captured/` | the source page's frontmatter; `wiki/sources/files/` | no hidden files |
| `wiki/log.md`, `index.md`, `hot.md` | `changes/`, `Wiki.base`, the `search` tool, the session-start context | code derives them; the model maintains nothing it can forget |
| generated cards and a generated board | the stub carries the thread's state; `Threads.base` is the board | Obsidian renders the board live |
| `project.json`, `registry.json`, the terminal view | `Atlas.md`, session documents, Bases | documents in place of state files |
| repositories inside the atlas folder, nested git | repository pages that point at a path | the vault never holds another repository |
| signals, modes (generic, lyt) | dropped | not used |

Kept from V1, rebuilt: the thread lifecycle and "the stage is the furthest document"; the code-owned lead callout; the immutable captured copy; one change, one commit; exact undo; read-only workers; "source content is data"; the write guard; lint.

## Decisions

These decisions change or complete the first V2 brainstorm.

| Question | Decision | Why |
|---|---|---|
| Are "Map Text" and "Wiki Draft" tools? | No. They are the agents `wiki-extract` and `wiki-draft`. The tools are the parts code can do: `source` chunks, `match`, `change`. | A tool is a fact or a commit. Extraction and drafting are judgment. |
| Wiki Draft: a neighbor that is not the same subject | Treat the item as new, and link the neighbor as related | "Skip" would lose the item |
| "git clean?" before ingest | Apply commits hand edits as `snapshot` first. No skill. | Code can do it every time; a skill can forget |
| "Get Conventions for Task" | `context` returns the policies on the scope chain, nearest first. A step in `thread-spec` and `thread-plan` keeps the ones that apply. | Candidate selection is a fact; relevance is judgment |
| "Thread search tool" | `search` with `types: [stub]` | one search over every document |
| When is a thread done? | A receipt closes it. `thread` refuses a `completed` receipt while a task is open. | "All tasks done?" is a fact |
| "Wikify stub", "Wikify spec" | Documents are pending until a change absorbs them; `wiki-sync` drains them at the end of a stage or in a batch | one pipeline, one backlog |
| Where does a thread's state live? | On the stub, the thread's root document | no generated card |
| Where does the vault's own context live? | `Atlas.md`, the root of the context graph | a document, like everything else |
| Are session documents in git? | Yes, through the `snapshot` commit | the history of what ran is kept |
| Closed threads | stay in place; `Threads.base` filters by stage | moving folders breaks links |
| Stack | a Go binary, a thin TypeScript Obsidian plugin, an npm package that installs both | see [[Stack]] |

### Names from the first draft

| First draft | V2 |
|---|---|
| Map Text tool | [[wiki-extract]] agent |
| Wiki Draft tool | [[wiki-draft]] agent |
| Get Conventions for Task tool | [[context]], and the conventions step of [[thread-spec]] and [[thread-plan]] |
| Thread search tool | [[search]] with `types: [stub]` |
| Document Ingest | [[wiki-ingest]], then [[wiki-sync]] |
| Working on Threads skill | [[thread-work]] |
| creating a thread, continuing a thread | [[thread-work]] steps 2 and 3 |
| creating a stub, creating a spec | [[thread-stub]], [[thread-spec]] |
| Creating Tasks | [[thread-plan]] |
| executing a task, closing a task | [[thread-run]] |
| Reciept | [[thread-receipt]] |
| Wikify Stub, Wikify Spec | pending documents, drained by [[wiki-sync]] |
| Repository Link, Unlink, Ingest | [[repo-link]], [[repo-unlink]], [[repo-ingest]] |
| Register empty session | the `SessionStart` hook |
| `source_id`, `page_id`, `content` | `doc`, `id`, `fields` and `body` |

## Open questions

1. **Repository paths across machines.** A vault synced by git to a second machine holds paths of the first. Proposal: `path` may name a path per host (`paths: {host: path}`), and `remote` identifies the repository.
2. **Codex hooks.** Claude Code gives hooks the session id, and for a subagent its `agent_id` and the parent's id (verified in the docs, 2026-09-27). Codex hooks differ. [[Hooks]] lists what each host must supply.
3. **Mentions.** `- [ ] @atlas …` task lines as requests to the agent. Designed in [[Obsidian Plugin#Mentions]]; built in phase 2.
4. **Ordering threads.** Wire stubs together on a canvas to show the order of work. A canvas already works with no code; code reading its edges waits for the idea that replaces phases.
5. **What is pending by default.** Sources, specs, and receipts. Adding `stub` or `task` to `wikify` in `Atlas.md` makes stubs, or done tasks, pending too.
6. **Search quality.** BM25 over title, aliases, description, and body, computed per call. Measure before adding anything more.

## Reading order

1. [[Atlas V1 Feedback]], then this page.
2. [[Document Types]], [[Vault Layout]], [[Wiki]], [[Thread Documents]], [[Sessions]], [[Changes]].
3. [[Entities]] and [[Entities.canvas|the entity flow]].
4. [[Tools]], [[Agents]], [[Hooks]].
5. [[Skills]] and the skill canvases.
6. [[Obsidian Plugin]], [[Stack]].
7. [[System Map.canvas|System Map]], then [[AgentFlow/Agent Session.canvas|Agent Session]].
