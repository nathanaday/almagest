---
name: thread-receipt
description: "Close a thread with its receipt: completed, with what was delivered and how it was verified; or killed, with the reason. Then offer the wiki what the work taught. Use for finish, done, close this thread, ship it, wrap up, kill, cancel, abandon, drop this thread."
---

# thread-receipt

A receipt closes a thread. A completed one follows a review of the work against the
spec; a killed one records why. Either way the spec and the receipt are pending for the
wiki, which absorbs what the work taught.

Tools: `thread` (show, file, open). Agents: [thread-review](../../agents/thread-review.md).
References: [threads.md](../atlas/references/threads.md).

## Procedure

**Completed**

1. `thread` show. Every task must be done or dropped; `thread` refuses the receipt
   otherwise. List any open task and ask: finish it, or drop it.
2. Send [thread-review](../../agents/thread-review.md) with the thread's id. Fix what
   breaks the spec's done-when list (back to [thread-run](../thread-run/SKILL.md)).
   Record the rest as follow-ups, with the user's yes.
3. Run each task's `## Verify` once more.
4. `thread` file with `part: receipt`, `outcome: completed`, and the text:
   `## Delivered`, `## Verified`, `## Follow-ups` (a `thread` open for each, linked),
   `## Learned` (what the wiki should absorb, in a few lines).

**Killed**

4. `thread` file with `part: receipt`, `outcome: killed`, and `## Why killed`. Open
   tasks become dropped.

**Both**

5. Write `## Summary` in this session's document with Edit.
6. Offer [wiki-sync](../wiki-sync/SKILL.md) for the thread's pending documents (its
   spec and its receipt), as one change.

## Gate

The review's findings, before the receipt. The wiki change has its own gate.

## Hand off

[wiki-sync](../wiki-sync/SKILL.md).
