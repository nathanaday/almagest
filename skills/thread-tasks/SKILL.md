---
name: thread-tasks
description: "Write the task lists of a thread: the steps that take each repository from where it is to the spec, as check boxes, each with the requirements it serves and its details. Use for break it down, plan the steps, write the tasks, how do we build this, add a task, what is left to do. Saying what done means is thread-spec; doing the tasks is thread-run."
---

# thread-tasks

A task list holds the how. Each task is one step a session can finish and check: it
serves one or more requirements of the spec, and its details say where and how. One
list per repository; a thread that changes no repository has one list with none.

Tools: `thread` (load, tasks), `context`. References:
[threads.md](../atlas/references/threads.md).

## Procedure

1. `thread` load with `thread`: read the spec, its requirements, and the tasks that
   exist. A thread with no spec goes back to [thread-spec](../thread-spec/SKILL.md).
2. For each repository the work touches, `context` with `repository`, then read the code
   as in plan mode: where each requirement lands, what it touches, what tests exist.
3. Write the tasks, per repository, in the order they must be done:
   - `text`: one line that says the step and its result ("Add the score to each box in
     `score.go`");
   - `requirements`: the ids it serves. Every requirement needs at least one task;
   - `details`: the files and interfaces, the approach, what to watch for, and the
     check that shows the task is done.
   A task is small enough to finish and check in one sitting, and large enough to be
   worth a line. A task that verifies ("run the benchmark on the board") is a task.
4. `thread` tasks with `thread`, `repository`, and `tasks`, once per repository. Code
   numbers the tasks. For work in no repository, leave `repository` out.
5. To add tasks later (a new requirement, a step you missed), call `thread` tasks again:
   it appends to the list of that repository. To drop one, `thread` check with
   `state: dropped` and `reason`.

Write tasks only. What you learned while planning goes to the spec's `## Decisions`
(with Edit), or to the wiki.

## Gate

None. Show the lists: each task with its requirements, per repository. The user approves
what the thread delivers (the spec), not how; go on unless the user objects.

## Hand off

[thread-run](../thread-run/SKILL.md).
