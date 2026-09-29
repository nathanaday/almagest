# spec-close

> Close a plan: `done`, with what was delivered and how it was verified; or `dropped`, with the reason. Then offer the wiki what the work taught.

**Use for**: finish, done, close this, ship it, wrap up, kill, cancel, abandon, drop this plan.

**Tools**: `work` (show, done, drop, stub). **Agents**: [[spec-review]]. **References**: `references/work.md`.

## Procedure

**Done**

1. `work` show. Every part must be done or dropped, and `## Done when` must hold a list; `work` refuses `done` otherwise. List any open part and ask: finish it, or drop it.
2. Send [[spec-review]] for a plan with parts, or with more than a few commits. Fix what breaks the done-when list (back to [[spec-run]]). Record the rest as follow-ups, with the user's yes.
3. Run the `## Verify` of the plan, or of each part, once more.
4. `work done` with the result: `delivered` (what changed, with the commits), `verified` (how, with the commands and their results), `follow_ups` (a `work stub` for each, linked), `learned` (what the wiki should absorb, in a few lines). Code writes the `completed` event that holds it.

**Dropped**

4. `work drop` with the reason. Open parts below it are dropped too, each with its own event.

**Both**

5. Write `## Summary` in this session's document.
6. Offer [[wiki-sync]] for the pending documents the work left (the spec, and the `completed` or `dropped` event), as one change.

## Gate

The review's findings, before `work done`. The wiki change has its own gate.

## Hand off

[[wiki-sync]]; for a part, [[spec-work]] on the parent.
