# spec-run

> Do one leaf plan in its repository, keep the spec current, and hand it to spec-close with its result.

**Use for**: run this part, do the next part, continue the plan, implement this plan.

**Tools**: `work` (show, start, block), `context`, and the repository's own tools. **References**: `references/work.md`.

## Procedure

1. Pick the plan: the one named, or the Work View's `next`.
2. `work start`. The hook binds this session to the plan, and the guard now lets it edit the plan's repository. Code writes `started`, or `continued` when the plan was started before.
3. `context` for the plan's repository: follow its instruction files and the plan's linked policies.
4. Work in the repository. Commit in the repository's own git as you go, in small commits whose messages name the plan's id. Test what can break.
5. When the work shows a better way to reach the plan's goal, take it, and record the change and its reason under `## Progress`. A change to the goal or the done-when list, or to another plan, goes back to the user first.
6. At each stopping point, add a dated line to the plan's `## Progress` with Edit. The session document quotes it, so write it once.
7. Run the plan's `## Verify`.
8. When the work is done, hand to [[spec-close]] for the result.
9. When the plan is blocked: `work block` with the reason in one line, a progress line, and stop.

## Gate

None inside a plan. The gates were the spec and the split.

## Hand off

[[spec-close]] for this plan; then the next ready part, or [[spec-close]] on the parent when no part is open.
