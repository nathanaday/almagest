
# thread-run

> Do one task in its repository, keep its document current, and close it with its result. The first draft's "executing a task" and "closing a task".

**Use for**: run T2, do the next task, continue the task, implement this task.

**Tools**: `thread` (show, task), `context`, and the repository's own tools. **References**: `references/threads.md`.

## Procedure

1. Pick the task: the one named, or the Thread View's `next`.
2. `thread` task start. The hook binds this session to the task, and the board shows it active.
3. `context` for the task's repository: follow its instruction files and the task's linked policies.
4. Work in the repository. Commit in the repository's own git as you go, in small commits whose messages name the thread id. Test what can break.
5. Own the outcome. When the work shows a better way than the task says, take it, and record the change and its reason under `## Progress`.
6. At each stopping point, add a dated line to the task's `## Progress` with Edit. The session document quotes it, so write it once.
7. Run the task's `## Verify`.
8. `thread` task done, with the result: what changed, the commits, how it was verified.
9. When the task is blocked: `thread` set blocked with one line, a progress line, and stop.

## Gate

None inside a task. The gates were the spec and the tasks.

## Hand off

The next ready task, or [[thread-receipt]] when no task is open.
