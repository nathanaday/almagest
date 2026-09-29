# spec-review

> Check the work of a plan against its spec, its parts, and its conventions, before `work done`. Sent by [[spec-close]].

**Takes**: a plan id.

**Returns**: [[Findings]], with checks of its own:

| Check | Finds |
|---|---|
| `done-when` | an item of the plan's `## Done when` that the work does not meet |
| `part` | a part whose `completed` event claims what its commits do not show |
| `convention` | a change that breaks a policy linked from the plan or a part |
| `break` | a change that breaks behavior outside the plan, with the evidence |
| `test` | behavior that can break and has no test |

The agent reads the plan, its parts, and their events with `work` show; the policies and instruction files with `context`; and the commits named in `## Progress` and in each part's `completed` event with `git log`, `git diff`, and `git show` (with `-C <path>` for the repository). The guard hook refuses any other command from this agent ([[Hooks#guard]]).
