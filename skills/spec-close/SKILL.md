---
name: spec-close
description: "Close a plan: done, with what was delivered and how it was verified; or dropped, with the reason. Then offer the wiki what the work taught. Use for finish, done, close this, ship it, wrap up, kill, cancel, abandon, drop this plan. Closing a stub that became a spec or a source happens in spec-write or wiki-ingest."
---

# spec-close

A plan closes `done` or `dropped`. A done plan follows a review of the work against its
`## Done when` list; a dropped one records why. Either way code writes an event, and the
spec and the event are pending for the wiki, which absorbs what the work taught.

Tools: `work` (show, done, drop, stub). Agents:
[spec-review](../../agents/spec-review.md). References:
[work.md](../atlas/references/work.md).

## Procedure

**Done**

1. `work` show with `doc`. `work done` refuses a plan that is not started, whose
   `## Done when` is empty, or with a part that is not done or dropped. List any open
   part and ask: finish it, or drop it. A parent plan is done when its parts are closed.
2. Send [spec-review](../../agents/spec-review.md) with the plan's id, for a plan with
   parts or with more than a few commits. Fix what breaks the done-when list (back to
   [spec-run](../spec-run/SKILL.md)). Record the rest as follow-ups, with the user's
   yes.
3. Run the `## Verify` of the plan, or of each part, once more.
4. For each follow-up, `work stub` with the user's words or yours, in one line.
5. `work done` with `spec` and `result`:
   - `delivered` (required): what changed, with the commits;
   - `verified` (required): how, with the commands and their results;
   - `follow_ups`: links to the stubs of step 4;
   - `learned`: what the wiki should absorb, in a few lines.
   Code writes the `completed` event that holds the result.

**Dropped**

5. `work drop` with `doc` and `reason`. Open parts below it are dropped too, each with
   its own event.

**Both**

6. Write `## Summary` in this session's document with Edit.
7. Offer [wiki-sync](../wiki-sync/SKILL.md) for the pending documents the work left
   (the spec, and its `completed` or `dropped` event), as one change.

## Gate

The review's findings, before `work done`. The wiki change has its own gate.

## Hand off

[wiki-sync](../wiki-sync/SKILL.md); for a part, [spec-work](../spec-work/SKILL.md) on
the parent.
