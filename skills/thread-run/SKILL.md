---
name: thread-run
description: "Do the open tasks of a thread in its repositories, and check each one when it is done, with its commits. Use for run the tasks, do the next task, implement this, continue the thread, build it. Writing the tasks is thread-tasks; checking the work against the spec is thread-verify."
---

# thread-run

Starting a thread binds this session to it: the board shows it active, no other session
takes it by accident, and the guard lets this session edit each repository the thread
has open tasks for. The task list is the record of what was done: a check mark with its
commits, task by task.

Tools: `thread` (load, start, check, tasks, block, note), `context`. References:
[threads.md](../atlas/references/threads.md).

## Procedure

1. `thread` load with `thread`. Read the spec and the open tasks with their details.
2. `thread` start with `thread`. When another live session holds it, ask the user before
   you start with `take: true`. When this session started it already, go on.
3. For each repository, `context` with `repository`: follow its instruction files and
   the spec's rules.
4. Take the open tasks in order. For each:
   - do the work. Change files with Edit and Write, not with the shell; use the shell to
     run commands. Test what can break;
   - commit in the repository's own git, with a message that names the task and the
     thread's id;
   - `thread` check with `thread`, `task`, and `commits`. Work that made no commit
     takes `note`, one line on what was done.
   Check a task when it is done, not in a batch at the end.
5. When the work shows a step the list lacks, add it (`thread` tasks) before you do it.
   When a task is not needed, drop it (`thread` check with `state: dropped`, `reason`).
6. When the work shows that a requirement is wrong, or cannot be met, stop that task.
   Say what you found. A change to the spec is the user's to approve; after the yes,
   revise it ([thread-spec](../thread-spec/SKILL.md)).
7. What you learn on the way has a home, and it is not the stub or the spec:
   - where the work stands, what you tried → a dated line under `## Progress` in your
     session document;
   - a fact the wiki should hold (how a tool behaves, a result, a pitfall) →
     [wiki-save](../wiki-save/SKILL.md), now;
   - work for another thread → `thread` stub.
8. When the thread waits on something outside: `thread` block with `reason` (one line);
   add a progress line; stop.

After the last task of a repository is checked, the guard refuses more edits there. A
fix needs a task first.

## Gate

None inside the tasks. The gate was the spec.

## Hand off

[thread-verify](../thread-verify/SKILL.md), when every task is done or dropped.
