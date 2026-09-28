
# thread-review

> Check the work of a thread against its spec, its tasks, and its conventions, before the receipt. Sent by [[thread-receipt]].

**Takes**: a thread id.

**Returns**: [[Findings]], with checks of its own:

| Check | Finds |
|---|---|
| `done-when` | an item of the spec's `## Done when` that the work does not meet |
| `task` | a task whose `## Result` the commits do not show |
| `convention` | a change that breaks a policy linked from a task |
| `break` | a change that breaks behavior outside the thread, with the evidence |
| `test` | behavior that can break and has no test |

The agent reads the thread with `thread` show, the policies and instruction files with `context`, and the commits named in each task's result with `git log`, `git diff`, and `git show`. The guard hook refuses any other command from this agent ([[Hooks#guard]]).
