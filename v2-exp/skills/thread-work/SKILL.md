---
name: thread-work
description: "Take a request or a thread to its next stage and on, until it is done or waits for the user: find the scope, find or open the thread, then run the stage skills in order. Use for work on X, do this, fix this, implement, build, continue, resume, pick up where we left off."
---

# thread-work

Every change to a repository belongs to a thread. This skill finds the repositories the
work touches and the thread it belongs to, binds this session to that thread, and runs
the stage skills on the thread's `next` until the work is done or a gate needs the user.

Tools: `search`, `context`, `thread`. References:
[threads.md](../atlas/references/threads.md).

## Procedure

1. **Scope.** Find the repositories or areas the work touches: `search` with
   `types: [repository, area]` and the request's words, then `context`. Ask when two
   match, and name both.
2. **Thread.** `search` with `types: [stub]`, `state: {stage: [stub, spec, tasks]}`,
   the request's words, and the scope.
   - one match → say it in one line, and call `thread` with `action: attach`;
   - several → ask which, or whether this is new work;
   - none → [thread-stub](../thread-stub/SKILL.md), with the request in the user's
     words.
3. Write `## Description` in this session's document with Edit: the thread, and what
   this session will do.
4. **Loop** on the Thread View's `next`:
   - `spec` → [thread-spec](../thread-spec/SKILL.md). Gate 1: the user agrees with the
     spec. Small work with an obvious approach may skip the spec; say so.
   - `tasks` → [thread-plan](../thread-plan/SKILL.md). Gate 2: the user agrees with the
     tasks.
   - `task T<n>` → [thread-run](../thread-run/SKILL.md). Tasks that are ready and
     independent may go to subagents, one task each, each running thread-run; each
     subagent gets its own session document and inherits the thread.
   - `receipt` → [thread-receipt](../thread-receipt/SKILL.md).
5. Stop at a gate, at a blocked task, or when the user's turn is needed. Before you
   stop, make sure the task's `## Progress` says where the work stands.

## Gates

Two: the spec and the tasks. A run with no user present (a test, or a request that says
so) takes the recommended default at each gate and says so.

## Hand off

The stage skills, in order. [wiki-sync](../wiki-sync/SKILL.md) comes through
thread-receipt.
