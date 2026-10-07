---
name: almagest
description: "Orient in the Almagest vault, say what waits for the user, do the work a request asks in a linked repository, and route any other request to the skill that owns it. Use for /almagest, what is going on, status, what waits for me, where do I work on X, work on X, fix this, implement, build, note this, remember to, publish my journal, what is in my journals, and any request when the right skill is not clear. Keeping part of this conversation is wiki-save; files to learn from are wiki-ingest."
---

# almagest

Every request passes this skill first. It reads where the session stands, names the kind
of request, finds the repository or the tags that the request names, and hands off. A
request to change code in a linked repository goes to no other skill: this skill finds
the repository and does the work.

Tools: `vault`, `search`, `context`. References:
[conventions.md](references/conventions.md).

## Procedure

1. Read the opening context: the vault, its repositories, the live sessions, the tags
   with their counts, and this session's document. Call `vault` when the context is
   missing or the user asks for the state. When the context says that the vault waits
   for `almagest vault migrate`, stop: tell the user to type `! almagest vault migrate`
   in this session (you cannot run it), then to start a new session. Until then, every
   tool and every edit in the vault refuses.
2. Name the kind of request with the table below.
3. When the request names work in a repository, find the repository: `search` with
   `types: [repository]` and the request's words, then `context` with `repository` set
   to the best hit. When two repositories match, ask which, and name both. Never guess
   between two.
4. When the request names categories ("my cs513 self-driving project"), turn the words
   that name tags of the vocabulary into `tags`, and pass them to the next call. The
   opening context and `vault` list the tags that exist.
5. Once the work is known, write one line under `## Description` in this session's
   document (the opening context links it). Use Edit.
6. Do the work in the repository ([Work in a repository](#work-in-a-repository)), or
   hand off.

| The user wants | Skill |
|---|---|
| an answer, an explanation, to explore; what the vault holds under a tag | [wiki-query](../wiki-query/SKILL.md) |
| a change to code in a linked repository: fix, build, implement, continue | this skill: [Work in a repository](#work-in-a-repository) |
| to note an idea, a bug, or a paper for later ("note this", "remember to") | this skill: [Notes for later](#notes-for-later) |
| to publish a journal, or to know what the journals hold | this skill: [Journals](#journals) |
| to ingest files, or process `ingest/` | [wiki-ingest](../wiki-ingest/SKILL.md) |
| to keep something from this conversation | [wiki-save](../wiki-save/SKILL.md) |
| to bring the wiki up to date with new sources | [wiki-sync](../wiki-sync/SKILL.md) |
| to link, unlink, or describe a repository | [repo-link](../repo-link/SKILL.md), [repo-unlink](../repo-unlink/SKILL.md), [repo-ingest](../repo-ingest/SKILL.md) |
| to check the wiki | [wiki-review](../wiki-review/SKILL.md) |
| to fix, rewrite, merge, or retag knowledge, or rename a tag | [wiki-edit](../wiki-edit/SKILL.md) |
| to organize the knowledge under a tag | [wiki-map](../wiki-map/SKILL.md) |
| a reading stack: check out the material on a subject, or return a checkout | [wiki-checkout](../wiki-checkout/SKILL.md) |
| to wikify a note: mark what the wiki knows in it, and the subjects worth a topic | [wiki-wikify](../wiki-wikify/SKILL.md) |
| a new vault | [almagest-onboard](../almagest-onboard/SKILL.md) |

A question can turn into work. When the user then asks for a change to a repository, do
the work.

## Messages from the palette

The Almagest palette in Obsidian starts an agent with one of seven messages. Each one names
its skill, and four name a work document. A work document's kind is `ingest`, `repair`,
or `draft`:

| The message | Skill |
|---|---|
| `/almagest:wiki-ingest Ingest the files of ingest/ … Your work document is [[…]] (<id>) …` | [wiki-ingest](../wiki-ingest/SKILL.md) |
| `/almagest:wiki-review …` with a repair work document | [wiki-review](../wiki-review/SKILL.md), its Repair path |
| `/almagest:wiki-sync Absorb the source [[<edition>]] (<id>), the user's journal edition. … Your work document is [[…]] (<id>) …` | [wiki-sync](../wiki-sync/SKILL.md), its journal edition rules |
| `/almagest:wiki-edit Remove [[<title>]] …: point each backlink elsewhere …` | [wiki-edit](../wiki-edit/SKILL.md), its Safe delete path |
| `/almagest:wiki-checkout Check out the material on: <request>` | [wiki-checkout](../wiki-checkout/SKILL.md) |
| `/almagest:wiki-wikify Wikify [[<copy title>]]: mark what the wiki knows …` | [wiki-wikify](../wiki-wikify/SKILL.md) |
| `/almagest:wiki-edit Draft a topic titled <Title> from [[<note>]] … Your work document is [[…]] (<id>) …` with a draft work document | [wiki-edit](../wiki-edit/SKILL.md), its Draft path |

The user decides in the document (Approve or Cancel), so these tasks do not stop for a
yes before they propose. Ask the user only for a real edge case that the skill names.

## The end of a task

End each task with one or two lines: what was done, and the change or work document as
a link. The documents hold the details; do not repeat them in the chat.

## Work in a repository

1. Read what `context` returned for the repository in step 3: follow its `instructions`
   (the paths of AGENTS.md and CLAUDE.md, and what they require) and the policies that
   bind the work ([conventions.md](references/conventions.md)).
2. Do the work. Change files with Edit and Write, not with the shell; use the shell to
   run commands. Test what can break.
3. When the work is long, or stops before it is done, add a dated line to `## Progress`
   in this session's document: where the work stands.
4. What you learn on the way has a home:
   - a fact the wiki should hold (how a tool behaves, a result, a pitfall) →
     [wiki-save](../wiki-save/SKILL.md);
   - work for later → a note in `scratchpad/`.

## Notes for later

An idea, a bug, or a paper the user wants to remember goes in a note in `scratchpad/`:
the user's words, with a short title as the file name. A file the wiki should learn from
goes in `ingest/`, for [wiki-ingest](../wiki-ingest/SKILL.md).

## Journals

`journals/` holds the user's own writing. A volume is a folder directly under
`journals/`. Its notes are every `.md` file under it, except its publication
history, `Journal · <folder>.md`, which code writes.

- **"Publish my journal."** Publish is the user's act. Tell the user to press Publish
  next to the volume on the Journals page of the Almagest palette, or to type
  `! almagest journal publish <volume>` in the session. Never run that command
  yourself: the guard refuses it from your shell. Publish captures the volume as one
  source, an edition. The palette then starts wiki-sync on the edition, with a work
  document. After a publish with `!`, the edition waits as a pending source; absorb it
  with [wiki-sync](../wiki-sync/SKILL.md) when the user asks.
- **"What is in my journals?"** `vault` status (`journals`) names each volume with its
  note count, its latest edition, and whether it changed since that edition.
  `almagest journal list` prints the same. Read the notes with Read, Grep, and
  Glob; search does not index `journals/`. Never edit a file there.

## Status

`vault` status gives the state of the vault. Say what waits for the user first: the
proposed changes and the sessions that wait. Then the running work documents
(`changes.running`), the files in `ingest/`, the sources
pending for the wiki, the journal volumes with changes to publish (`journals` entries
with `changed: true`; the user publishes them), the checkouts with edited copies that
are not returned (`checkouts` entries with `edited` above 0 and `returned` empty; Return
proposes their edits), and the lint problems, each with the
skill that handles it. The opening context names each repository that is behind its
description ([repo-ingest](../repo-ingest/SKILL.md)), and each journal volume with
changes to publish.

## Vault rules

- Never report the vault's git state to the user. The Obsidian plugin commits the
  user's hand edits as quiet snapshots, and every write tool commits a snapshot first.
- `journals/` holds the user's own writing. Read it when the request needs it; never
  edit a file there, and never run `almagest journal publish`. The guard refuses
  every agent edit under `journals/`, and the publish command from your shell.
- `checkout/` is the user's: the copies the librarian checked out, their reading lists,
  and the ledger. Code writes it; the user reads and edits the copies. Never edit a file
  there; the guard refuses every agent edit under `checkout/`.
- A wikified copy (`scratchpad/<name> · wikified.md`) is the user's scratch. Only
  `wikify mark` writes into it; never edit it with Edit or Write.
- `tool/trash/` holds what the user deleted. Never read or edit it, and never run
  `almagest vault trash`: safe delete is the user's action. The guard refuses
  every agent edit under `tool/trash/`.

## Gate

None. The skill that the request routes to keeps its own gate.

## Hand off

The routed skill.
