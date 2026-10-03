---
name: chord-work
description: "Take a chord forward: load it by id, take its ready threads in order, and run each through the thread skills. This is what a chord's hand-off line asks for. Use for Resume Atlas chord, continue the chord, what is next in this chord, work on the chord, how far is the chord. Making a chord is chord-create; one thread is thread-work."
---

# chord-work

A chord's threads are done in order, over many sessions. `chord` load says where it
stands: each thread's status, the threads that are ready, and the next step. A thread is
ready when every thread it comes after is verified.

Tools: `chord` (load, list), `thread` (load). References:
[threads.md](../atlas/references/threads.md).

## Procedure

1. `chord` load with `chord` (the id of `Resume Atlas chord doc-…`, or the title). With
   no chord named, `chord` list, and ask which.
2. Write `## Description` in this session's document with Edit: the chord, and what
   this session will do.
3. Say where the chord stands: its threads in order with each status, the ones that are
   ready, and the ones that are blocked or wait.
4. When `canvas.differs` is true, the user redrew the order and has not saved it. Say
   what the canvas would change, and ask the user to save or revert it in Obsidian
   before you go on.
5. Follow `next.step`:
   - `work` → pick the thread: one that is verified and waits for its wiki change comes
     first ([thread-close](../thread-close/SKILL.md)); then one that is started; then
     the first ready one. Run [thread-work](../thread-work/SKILL.md) on it.
   - `create` → the chord has no thread: [chord-create](../chord-create/SKILL.md).
   - `close` → every thread is closed or dropped: [chord-close](../chord-close/SKILL.md).
   - `wait` → every open thread is blocked or waits. Say on what, and stop.
6. Ready threads with no path between them may go to subagents, one thread each, each
   running thread-work. Each subagent gets its own session document and starts its own
   thread. Do this only when the user asks for it, or agreed to it.
7. After a thread reaches a gate or closes, `chord` load again, and go on with the next
   ready thread, until a gate needs the user or nothing is ready.

## Gate

The gates of the thread skills. None of its own.

## Hand off

[thread-work](../thread-work/SKILL.md) on each thread;
[chord-close](../chord-close/SKILL.md) when every thread is closed or dropped.
