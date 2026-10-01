---
name: thread-close
description: "Close a verified thread: absorb its spec and its verification into the wiki, so the knowledge base says what the product does now; or drop a thread, with the reason. Use for close this, finish, wrap up, ship it, absorb this thread, why is this still open, kill, cancel, abandon, drop this thread. Checking the work is thread-verify."
---

# thread-close

No call closes a thread. Code closes it when two things are true: the thread is
verified, and an applied change absorbed its spec and its last verification. So closing
a thread is writing down what the work changed: "p3-edge loads one TFLite format", on
the pages of the repository and the entities it touched. The user's apply is the close.

Tools: `thread` (load, drop), `vault`, `change`. References:
[threads.md](../atlas/references/threads.md),
[changes.md](../atlas/references/changes.md).

## Procedure

**Close**

1. `thread` load with `thread`. Its status must be `verified`. For any other status,
   say what `missing` lists, and go to the skill `next` names. Never tell the user to
   close it by hand: no button and no call does that.
2. Run [wiki-sync](../wiki-sync/SKILL.md) on two documents: the spec and the last
   verification (their ids are in the load). The change says what is true now:
   - each repository the thread changed: what it does now, in its repository document or
     the topics under its tag;
   - each entity, concept, or policy the spec's `## Knowledge` lists: what the work
     changed or showed about it;
   - what the verification's notes and findings taught.
   Set `work` to the thread. Cite the spec and the verification in `sources`.
3. When the work taught the wiki nothing new, propose the change with `absorbs` and no
   writes, and say so in its `notes`.
4. Show the preview. The user's yes, then `change` apply, closes the thread. The change
   tool refuses this apply until the user has answered, with or without writes.
5. `thread` load again: the status is `closed`. Write `## Summary` in this session's
   document with Edit.

**Drop**

1. `thread` drop with `thread` and `reason`. A dropped thread lets the threads after it
   go on.
2. What the thread learned before it was dropped still belongs in the wiki: offer
   [wiki-sync](../wiki-sync/SKILL.md) on the `dropped` event.

## Gate

The wiki change: the user's yes before apply. A drop of a started thread: ask first.

## Hand off

[chord-work](../chord-work/SKILL.md), when the thread belongs to a chord: its next
threads are ready. [chord-close](../chord-close/SKILL.md), when it was the chord's last.
