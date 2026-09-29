# spec-split

> Split a plan into parts: child plans, each a piece of work in one repository that can be verified alone.

**Use for**: break it down, plan the parts, how do we build this, what are the steps.

**Tools**: `work` (show, spec), `context`. **References**: `references/work.md`, `references/conventions.md`.

## Procedure

1. `work` show: read the plan.
2. Explore the code of each repository it names, as in plan mode: where the work lands, what it touches, what tests exist.
3. Split the work. A part:
   - has one deliverable, in one repository;
   - can be verified alone, by a command or a check;
   - names the sibling parts it depends on, and its order.
   Split where the work splits. Do not aim for a number of parts. A part that is still large may be split again later, into parts of its own.
4. **The conventions step**, per part: `context` for the part's repository, then keep the policies that bind this part, each with one line on why.
5. For each part, write `## Goal`, `## Done when`, `## Where`, `## Conventions`, and `## Verify`.
6. `work spec` with the list: each part with `parent` set to the plan, its `repositories`, `depends` by title, and `order`. One commit writes them all.

## Gate

Show the parts as a list: order, repository, dependencies, and the verify line of each. Wait for the user to agree or edit.

## Hand off

[[spec-run]], through [[spec-work]].
