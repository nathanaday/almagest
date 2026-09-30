---
name: spec-split
description: "Split a plan into parts: child plans, each one piece of work in one repository that can be verified alone, with the policies it must follow. Use for break it down, plan the parts, how do we build this, what are the steps. Writing the plan itself is spec-write; doing a part is spec-run."
---

# spec-split

One plan splits into parts. A part is a child plan: one deliverable in one repository,
a check that proves it, and the sibling parts it waits for. Two parts with no
dependency between them can run in two sessions at once.

Tools: `work` (show, spec), `context`. References:
[work.md](../atlas/references/work.md),
[conventions.md](../atlas/references/conventions.md).

## Procedure

1. `work` show with `doc`: read the plan.
2. Explore the code of each repository the plan names, as in plan mode: where the work
   lands, what it touches, what tests exist.
3. Split the work. A part:
   - has one deliverable, in one repository;
   - can be verified alone, by a command or a check;
   - names the sibling parts it depends on, and its order.
   Split where the work splits. Do not aim for a number of parts. A part that is still
   large may be split again later, into parts of its own.
4. **The conventions step** ([conventions.md](../atlas/references/conventions.md)), per
   part: `context` with the part's `repository`, then keep the policies that bind this
   part, each with one line on why.
5. For each part, write `## Goal`, `## Done when`, `## Where` (files, modules,
   interfaces), `## Conventions` (policy links), and `## Verify` (the commands or checks
   that prove it works).
6. `work spec` with `specs`, one per part: `title`, `kind: plan`, `text`, `parent` (the
   plan), `repositories` (one each), `depends` (sibling parts, by title; a part of the
   same call counts), and `order`. One commit writes them all.

## Gate

Show the parts as a list: the order, the repository, the dependencies, and the verify
line of each. Wait for the user to agree or to edit.

## Hand off

[spec-run](../spec-run/SKILL.md), through [spec-work](../spec-work/SKILL.md).
