---
name: atlas
description: "Orient in the Atlas vault, say what waits for the user, do the work a request asks in a linked repository, and route any other request to the skill that owns it. Use for /atlas, what is going on, status, what waits for me, where do I work on X, work on X, fix this, implement, build, note this, remember to, and any request when the right skill is not clear. Keeping part of this conversation is wiki-save; files to learn from are wiki-ingest."
---

# atlas

Every request passes this skill first. It reads where the session stands, names the kind
of request, finds the repository or the tags that the request names, and hands off. A
request to change code in a linked repository goes to no other skill: this skill finds
the repository and does the work.

Threads and chords left Atlas in 9.0. They live in the standalone project
obsidian-threads. The 9.0 migration moved the old thread documents to `threads/`, which
Atlas does not read.

Tools: `vault`, `search`, `context`. References:
[conventions.md](references/conventions.md).

## Procedure

1. Read the opening context: the vault, its repositories, the live sessions, the tags
   with their counts, and this session's document. Call `vault` when the context is
   missing or the user asks for the state.
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
| to ingest files, or process `ingest/` | [wiki-ingest](../wiki-ingest/SKILL.md) |
| to keep something from this conversation | [wiki-save](../wiki-save/SKILL.md) |
| to bring the wiki up to date with new sources | [wiki-sync](../wiki-sync/SKILL.md) |
| to link, unlink, or describe a repository | [repo-link](../repo-link/SKILL.md), [repo-unlink](../repo-unlink/SKILL.md), [repo-ingest](../repo-ingest/SKILL.md) |
| to check the wiki | [wiki-review](../wiki-review/SKILL.md) |
| to fix, rewrite, merge, or retag knowledge, or rename a tag | [wiki-edit](../wiki-edit/SKILL.md) |
| to organize the knowledge under a tag | [wiki-map](../wiki-map/SKILL.md) |
| a new vault | [atlas-onboard](../atlas-onboard/SKILL.md) |

A question can turn into work. When the user then asks for a change to a repository, do
the work.

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

## Status

`vault` status gives the state of the vault. Say what waits for the user first: the
proposed changes and the sessions that wait. Then the files in `ingest/`, the sources
pending for the wiki, and the lint problems, each with the skill that handles it. The
opening context names each repository that is behind its description
([repo-ingest](../repo-ingest/SKILL.md)).

## Vault rules

- Never report the vault's git state to the user. The Obsidian plugin commits the
  user's hand edits as quiet snapshots, and every write tool commits a snapshot first.
- `journals/` holds the user's own writing. Read it when the request needs it; never
  edit a file there. The guard refuses every agent edit under `journals/`.

## Gate

None. The skill that the request routes to keeps its own gate.

## Hand off

The routed skill.
