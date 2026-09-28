
# thread-work

> Take a request or a thread to its next stage and on, until it is done or waits for the user. The conductor of the thread skills. It covers "creating a thread" and "continuing a thread" from the first draft. See [[thread-work.canvas|the thread-work flow]].

**Use for**: work on X, do this, fix this, implement, build, continue, resume, pick up where we left off.

**Tools**: `search`, `context`, `thread`. **References**: `references/threads.md`.

## Procedure

1. **Scope.** Find the repositories or areas the work touches (`search` with `types: [repository, area]`, then `context`). Ask when two match.
2. **Thread.** `search` with `types: [stub]`, `state: {stage: [stub, spec, tasks]}`, the request's words, and the scope.
   - one match → say it in one line, and `thread` attach;
   - several → ask which, or whether this is new work;
   - none → [[thread-stub]], with the request in the user's words.
3. Write `## Description` in this session's document: the thread, and what this session will do.
4. **Loop** on the Thread View's `next`:
   - `spec` → [[thread-spec]]. Gate 1: the user agrees with the spec. Small work with an obvious approach may skip the spec; say so.
   - `tasks` → [[thread-plan]]. Gate 2: the user agrees with the tasks.
   - `task T<n>` → [[thread-run]]. Tasks that are ready and independent may go to subagents, one task each, each running [[thread-run]]; each subagent gets its own session document and inherits the thread.
   - `receipt` → [[thread-receipt]].
5. Stop at a gate, at a blocked task, or when the user's turn is needed. Before stopping, make sure the task's `## Progress` says where the work stands.

## Gates

Two: the spec and the tasks. An unattended run takes the recommended default at each and says so.

## Hand off

The stage skills, in order. [[wiki-sync]] comes through [[thread-receipt]].
