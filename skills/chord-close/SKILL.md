---
name: chord-close
description: "Close a chord whose threads are all closed: write its overview into the wiki, what was built and the threads that built it; or drop a chord, with the reason. Use for close the chord, wrap up the project, the chord is done, summarize what we built, drop this chord, cancel this project. Closing one thread is thread-close."
---

# chord-close

A chord is `done` when every thread is closed or dropped. It is `closed` when an applied
change absorbed it: the wiki then holds one page that says what the chord delivered,
with links to the threads that built each part. Each thread put its own knowledge into
the wiki when it closed; this page ties them together.

Tools: `chord` (load, drop), `search`, `change`. References:
[threads.md](../atlas/references/threads.md),
[changes.md](../atlas/references/changes.md),
[pages.md](../atlas/references/pages.md).

## Procedure

**Close**

1. `chord` load with `chord`. Its status must be `done`. Else say which threads have
   not ended, and hand to [chord-work](../chord-work/SKILL.md).
2. Read each thread's spec and last verification, and the wiki pages their changes
   wrote.
3. Build one Wiki Change Plan:
   - `absorbs`: the chord's id; `work`: the chord;
   - one topic that sums up the chord: what is true now, each part with the thread that
     delivered it, what was dropped and why, and what the work showed. Create it, or
     modify the page that already covers the subject (`search` first). Cite the chord
     and the specs in `sources`;
   - a modify for each page the whole chord changed that no single thread updated.
4. `change` propose. Show the preview.
5. After the user's yes, `change` apply. `chord` load again: the status is `closed`.
6. Write `## Summary` in this session's document with Edit.

**Drop**

1. `chord` drop with `chord` and `reason`. Each thread that is not ended is dropped with
   it; closed threads stay closed. `chord` reopen brings the chord and those threads
   back.

## Gate

The wiki change: the user's yes before apply. A drop: ask first, and name the threads
it drops.

## Hand off

None.
