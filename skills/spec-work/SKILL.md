---
name: spec-work
description: "Take a request or a plan to its next step and on, until it is done or waits for the user: find the repository, find or plant the stub or plan, then run the spec skills in order. Use for work on X, do this, fix this, implement, build, continue, resume, pick up where we left off. Planting an idea for later with no work now is wiki-stub."
---

# spec-work

Every edit in a linked repository belongs to a started plan: the guard refuses the edit
while this session has started no plan that names the repository. This skill finds the
repositories the work touches and the plan or stub it belongs to, then runs the spec
skills on the Work View's `next` until the work is done or a gate needs the user.

Tools: `search`, `context`, `work`. References:
[work.md](../atlas/references/work.md).

## Procedure

1. **Repository.** Find the repositories the work touches: `search` with
   `types: [repository]` and the request's words or tags, then `context` with
   `repository`. Ask when two match, and name both.
2. **Plan or stub.** `search` with `types: [spec, stub]`, `status: [open, started]`,
   the request's words, and `repository`.
   - one match → say it in one line, and go on with it (`work` show);
   - several → ask which, or whether this is new work;
   - none → [wiki-stub](../wiki-stub/SKILL.md), with the request in the user's words.
3. Write `## Description` in this session's document with Edit: the plan or stub, and
   what this session will do.
4. **Loop** on the Work View's `next` (`work` show):
   - `write` → [spec-write](../spec-write/SKILL.md). Gate 1: the user agrees with the
     spec. Small work with an obvious approach may skip to a short leaf plan (`## Goal`,
     `## Done when`, `## Where`, `## Verify`); say so.
   - `start <plan>` on a plan with no parts: when the work spans several repositories,
     or pieces that a reviewer would check apart,
     [spec-split](../spec-split/SKILL.md) first. Gate 2: the user agrees with the parts.
     Otherwise [spec-run](../spec-run/SKILL.md) on the plan.
   - `start <part>` → [spec-run](../spec-run/SKILL.md) on the part. Parts that are
     ready and independent may go to subagents, one part each, each running spec-run;
     each subagent gets its own session document and starts its own part.
   - `done` → [spec-close](../spec-close/SKILL.md).
   - `none` → the stub or spec is closed, or it is a design. Say so, and stop.
5. Stop at a gate, at a blocked plan, or when the user's turn is needed. Before you
   stop, make sure the started plan's `## Progress` says where the work stands.

## Gates

Two: the spec and the split. A run with no user present (a test, or a request that says
so) takes the recommended default at each gate and says so.

## Hand off

The spec skills, in order. [wiki-sync](../wiki-sync/SKILL.md) comes through
spec-close.
