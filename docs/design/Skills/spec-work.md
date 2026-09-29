# spec-work

> Take a request or a plan to its next step and on, until it is done or waits for the user. It runs the spec skills in order, for new work and for work that exists.

**Use for**: work on X, do this, fix this, implement, build, continue, resume, pick up where we left off.

**Tools**: `search`, `context`, `work`. **References**: `references/work.md`.

## Procedure

1. **Repository.** Find the repositories the work touches (`search` with `types: [repository]` and the request's words or tags, then `context`). Ask when two match.
2. **Plan or stub.** `search` with `types: [spec, stub]`, `status: [open, started]`, the request's words, and the repository.
   - one match → say it in one line, and go on with it;
   - several → ask which, or whether this is new work;
   - none → [[wiki-stub]], with the request in the user's words.
3. Write `## Description` in this session's document: the plan, and what this session will do.
4. **Loop** on the [[Work View]]'s `next`:
   - `write` → [[spec-write]]. Gate 1: the user agrees with the spec. Small work with an obvious approach may skip to a short leaf plan (Goal, Done when, Where, Verify); say so.
   - `start <plan>` on a plan with no parts: when the work spans several repositories, or pieces that a reviewer would check apart, [[spec-split]] first. Gate 2: the user agrees with the parts. Otherwise [[spec-run]]. Parts that are ready and independent may go to subagents, one part each, each running [[spec-run]]; each subagent gets its own session document and inherits the started plans.
   - `done` → [[spec-close]].
5. Stop at a gate, at a blocked plan, or when the user's turn is needed. Before stopping, make sure the plan's `## Progress` says where the work stands.

## Gates

Two: the spec and the split. A run with no user present (a test, or a request that says so) takes the recommended default at each gate and says so.

## Hand off

The spec skills, in order. [[wiki-sync]] comes through [[spec-close]].
