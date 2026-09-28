
# thread-receipt

> Close a thread with its receipt: completed, with what was delivered and how it was verified; or killed, with the reason. Then offer the wiki what the work taught.

**Use for**: finish, done, close this thread, ship it, wrap up, kill, cancel, abandon, drop this thread.

**Tools**: `thread` (show, file receipt, open), `session`. **Agents**: [[thread-review]]. **References**: `references/threads.md`.

## Procedure

**Completed**

1. `thread` show. Every task must be done or dropped; `thread` refuses the receipt otherwise. List any open task and ask: finish it, or drop it.
2. Send [[thread-review]]. Fix what breaks the spec's done-when list (back to [[thread-task]]). Record the rest as follow-ups, with the user's yes.
3. Run each task's `## Verify` once more.
4. `thread` file receipt, `outcome: completed`: `## Delivered`, `## Verified`, `## Follow-ups` (a `thread` open for each, linked), `## Learned` (what the wiki should absorb, in a few lines).

**Killed**

1. `thread` file receipt, `outcome: killed`, with `## Why killed`. Open tasks become dropped.

**Both**

5. `session` summary.
6. Offer [[wiki-sync]] for the thread's pending documents (its spec and its receipt), as one change.

## Gate

The review's findings, before the receipt. The wiki change has its own gate.

## Hand off

[[wiki-sync]].
