
# thread-tasks

> Split the spec into tasks: each one a piece of work in one repository that can be verified alone. It replaces V1's plan stage, because one spec spawns many tasks.

**Use for**: break it down, plan the tasks, how do we build this, what are the steps.

**Tools**: `thread` (show, tasks), `context`. **References**: `references/threads.md`, `references/conventions.md`.

## Procedure

1. `thread` show: read the spec.
2. Explore the code of each repository in scope, as in plan mode: where the work lands, what it touches, what tests exist.
3. Split the work. A task:
   - has one deliverable, in one repository;
   - can be verified alone, by a command or a check;
   - names the tasks it depends on.
   Split where the work splits. Do not aim for a number of tasks.
4. **The conventions step**, per task: `context` for the task's repository, then keep the policies that apply to this task, each with one line on why.
5. For each task, write `## What`, `## Where`, `## Conventions`, and `## Verify`.
6. `thread` tasks with the list.

## Gate

Show the tasks as a list: order, repository, dependencies, and the verify line of each. Wait for the user to agree or edit.

## Hand off

[[thread-task]], through [[thread-work]].
