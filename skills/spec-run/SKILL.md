---
name: spec-run
description: "Do one plan with no parts in its repository, keep the plan's progress current, and hand it to spec-close with its result. Use for run this part, do the next part, continue the plan, implement this plan. Closing a plan is spec-close; splitting one is spec-split."
---

# spec-run

A plan with no parts is the unit a session works on. Starting it binds this session to
it, so the board shows it active, no other session takes it by accident, and the guard
lets this session edit the plan's repository. The plan is the record of the work:
progress lines as it goes, the result when it closes.

Tools: `work` (show, start, block), `context`, and the repository's own tools.
References: [work.md](../atlas/references/work.md).

## Procedure

1. Pick the plan: the one named, or the Work View's `next`.
2. `work start` with `spec`. The hook binds this session to the plan. Starting a part
   starts the plans above it. Code writes a `started` event, or `continued` when an
   earlier session started the plan. When another live session holds it, ask the user
   before you start with `take: true`. When this session started it already, go on
   with the work.
3. `context` with the plan's `repository`: follow its instruction files and the plan's
   linked policies.
4. Work in the repository. Change files with Edit and Write, not with the shell: the
   guard and the session's record see the edit tools. Use the shell to run commands.
   Commit in the repository's own git as you go, in small commits whose messages name
   the plan's id. Test what can break.
5. When the work shows a better way to reach the plan's goal, take it, and record the
   change and its reason under `## Progress`. A change to the goal or the done-when
   list, or to another plan, goes back to the user first.
6. At each stopping point, add a dated line to the plan's `## Progress` with Edit
   (`- 2026-09-27: …`). The session document quotes it, so write it once.
7. Run the plan's `## Verify`.
8. When the work is done, hand to [spec-close](../spec-close/SKILL.md) for the result.
9. When the plan is blocked: `work block` with `spec` and `reason` (one line); add a
   progress line; stop.

## Gate

None inside a plan. The gates were the spec and the split.

## Hand off

[spec-close](../spec-close/SKILL.md) for this plan; then the next ready part, or
spec-close on the parent when no part is open.
