---
name: thread-run
description: "Do one task of a thread in its repository, keep the task document current, and close it with its result. Use for run T2, do the next task, continue the task, implement this task."
---

# thread-run

A task is the unit a session works on. Starting it binds this session to it, so the
board shows it active and no other session takes it by accident. The task document is
the record of the work: progress lines as it goes, the result at the end.

Tools: `thread` (show, task), `context`, and the repository's own tools. References:
[threads.md](../atlas/references/threads.md).

## Procedure

1. Pick the task: the one named, or the Thread View's `next`.
2. `thread` with `action: task`, `do: start`. The hook binds this session to the task.
   When another live session holds it, ask the user before you start with `take: true`.
3. `context` for the task's repository: follow its instruction files and the task's
   linked policies.
4. Work in the repository. Change files with Edit and Write, not with the shell: the
   guard and the session's record see the edit tools. Use the shell to run commands.
   Commit in the repository's own git as you go, in small commits whose messages name
   the thread's id. Test what can break.
5. When the work shows a better way to reach the task's deliverable, take it, and record
   the change and its reason under `## Progress`. A change to the deliverable itself, or
   to another task, goes back to the user first.
6. At each stopping point, add a dated line to the task's `## Progress` with Edit
   (`- 2026-09-27: …`). The session document quotes it, so write it once.
7. Run the task's `## Verify`.
8. `thread` with `action: task`, `do: done`, and the `result`: what changed, the
   commits, how it was verified.
9. When the task is blocked: `thread` with `action: task`, `do: set`, and `blocked` (one
   line); add a progress line; stop.

## Gate

None inside a task. The gates were the spec and the tasks.

## Hand off

The next ready task, or [thread-receipt](../thread-receipt/SKILL.md) when no task is
open.
