---
name: thread-work
description: "Take a thread from where it stands to its next gate and on: load it by id or find it by the request, then run the thread skill its next step names, in a loop. This is what a hand-off line asks for. Use for Resume Atlas thread, resume work on, continue, pick up where we left off, work on X, do this, fix this, implement, build. Planting an idea for later with no work now is thread-stub; a goal that needs several threads is chord-create."
---

# thread-work

Every edit in a linked repository belongs to a started thread with an open task for
that repository; the guard refuses the edit otherwise. This skill finds the thread, or
plants it, and then follows what `thread` load says comes next. Code derives the next
step from the documents, so every session takes the same path: spec, tasks, run,
verify, close.

Tools: `search`, `context`, `thread` (load, list, stub). References:
[threads.md](../atlas/references/threads.md).

## Procedure

1. **Find the thread.**
   - The request gives an id (`Resume Atlas thread doc-…`) or a title → `thread` load.
   - The request names work → `search` with `types: [stub]` and the request's words, and
     `repository` when it names one. One match that is not ended → say it in one line,
     and load it. Several → ask which, or whether this is new work. None →
     [thread-stub](../thread-stub/SKILL.md), with the request in the user's words, then
     load the new thread.
2. Write `## Description` in this session's document with Edit: the thread, and what
   this session will do.
3. Say where the thread stands in two lines: its status, what `missing` lists, and
   `next`.
4. **Loop** on `next.step` (from `thread` load, or from `state` after any write):

   | Step | Do |
   |---|---|
   | `spec` | [thread-spec](../thread-spec/SKILL.md). Gate: the user agrees with the spec. |
   | `tasks` | [thread-tasks](../thread-tasks/SKILL.md). |
   | `start`, `run` | [thread-run](../thread-run/SKILL.md). |
   | `verify`, `findings` | [thread-verify](../thread-verify/SKILL.md). Gate: a finding that changes the spec, or that the user accepts. |
   | `close` | [thread-close](../thread-close/SKILL.md). Gate: the wiki change. |
   | `wait` | Say what it waits on (a block, or a thread before it), and stop. Offer to work on the thread it waits for. |
   | `none` | The thread is closed, dropped, or resolved. Say so, and stop. |

5. Stop at a gate, or when the user's turn is needed. Before you stop, add a dated line
   to `## Progress` in your session document: where the work stands.

Small work takes the same path, with short documents: a spec of one requirement, one
task, one verification. Write the spec and the task list in one pass, show both, and
take one yes for both. Do not skip a document, and do not put one document's content
into another.

Work that turns out to need several threads, in several repositories or over many
sessions, is a chord: [chord-create](../chord-create/SKILL.md).

## Gates

Three: the spec; a finding that changes the spec or that the user accepts; the wiki
change that closes the thread. A run with no user present (a test, or a request that
says so) takes the recommended default at the first two and says so. It stops at the
third: only the user closes a thread.

## Hand off

The thread skills, in the order `next` names.
