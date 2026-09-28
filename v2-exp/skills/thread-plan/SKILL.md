---
name: thread-plan
description: "Split a thread's spec into tasks: each one piece of work in one repository that can be verified alone, with the policies it must follow. Use for break it down, plan the tasks, how do we build this, what are the steps."
---

# thread-plan

One spec spawns many tasks. A task has one deliverable in one repository, a check that
proves it, and the tasks it waits for. Two tasks with no dependency between them can
run in two sessions at once.

Tools: `thread` (show, tasks), `context`. References:
[threads.md](../atlas/references/threads.md),
[conventions.md](../atlas/references/conventions.md).

## Procedure

1. `thread` show: read the spec.
2. Explore the code of each repository in scope, as in plan mode: where the work lands,
   what it touches, what tests exist.
3. Split the work. A task:
   - has one deliverable, in one repository;
   - can be verified alone, by a command or a check;
   - names the tasks it depends on.
   Split where the work splits. Do not aim for a number of tasks.
4. **The conventions step**, per task: `context` for the task's repository, then keep
   the policies that apply to this task, each with one line on why.
5. For each task, write `## What`, `## Where`, `## Conventions`, and `## Verify`.
6. `thread` with `action: tasks` and the list: each with `title`, `text`,
   `repository`, and `depends` (T1, or the title of an earlier task in the list).

## Gate

Show the tasks as a list: the order, the repository, the dependencies, and the verify
line of each. Wait for the user to agree or to edit.

## Hand off

[thread-run](../thread-run/SKILL.md), through [thread-work](../thread-work/SKILL.md).
