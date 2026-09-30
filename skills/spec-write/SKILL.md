---
name: spec-write
description: "Turn a stub or a request into a spec: a plan (what will be true when the work is done, and why), or a design (how a part must work). Research first; ask only what research cannot answer. Use for spec this, define this, what should this be, design this, flesh out the stub, requirements, how should this work. Splitting a plan into parts is spec-split."
---

# spec-write

A spec is a plan or a design. A plan says what is true when the work is done, in a list
a reviewer can check, with the decisions and their reasons. A design says how a part
must keep working. Most of a spec comes from reading the code and the wiki. The user
answers only what reading cannot decide and a wrong guess would change.

Tools: `work` (show, promote, spec), `context`, `search`. References:
[work.md](../atlas/references/work.md),
[conventions.md](../atlas/references/conventions.md).

## Procedure

1. `work` show with `doc`: read the stub, its idea, and its notes. With no stub, read
   the request.
2. Choose the kind: `plan` for work that ends; `design` for how a part must keep
   working. A request for both is a design and a plan that `implements` it.
3. `context` with `repository` for each repository: the instruction files, the git
   facts, the tag pages, and the candidate policies.
4. Research: read the code the work touches, and the wiki (`search`, then Read) for
   what the vault already knows, designs included.
5. Decide what the research decides. Ask only what it cannot answer and a wrong guess
   would change, in one message, each question with a recommended answer. Ask no fixed
   number of questions.
6. **The conventions step** ([conventions.md](../atlas/references/conventions.md)): for
   each candidate policy, decide whether it binds this work. Keep the ones that do, each
   with one line on why.
7. Write the spec:
   - a plan: `## Goal`, `## Done when` (a list a reviewer can check), `## Decisions`
     (each with its reason), `## Out of scope`, `## Conventions`, `## Open questions`;
     for a plan with no parts to come, also `## Where` and `## Verify`. Set
     `repositories`, `tags`, and `priority`.
   - a design: `## Purpose`, `## Behavior`, `## Interfaces`, `## Constraints`,
     `## Decisions`, `## Open questions`. Set `repositories` and `tags`.
   A plan needs a `## Done when` list before work starts: `work start` refuses a plan
   without one.
8. File it:
   - the stub becomes this one spec → `work promote` with `stub`, `kind`, `text`, and
     `title`, `description`, `tags`, `repositories`, `parent`, `priority` as they
     apply. The spec keeps the stub's id, and the stub's words stay as `## Origin`.
   - the stub becomes several documents → `work spec` with `specs`, each with
     `from` set to the stub, and `resolve: true` on the call that writes the last. In
     one call, `parent` and `depends` may name another spec of the call by its title.
   - no stub → `work spec` with `specs`: each with `title`, `kind`, `text`, and
     `description`, `tags`, `repositories`, `priority`, `implements`, `supersedes` as
     they apply.
   In `tagging: known`, a tag no document holds needs the user's yes; then pass
   `new_tags: true`.

## Gate

Link the spec and say its goal and its done-when list (or its behavior) in a few lines.
Wait for the user to agree or to edit it. An edit in Obsidian counts: read the spec
again before you go on.

## Hand off

[spec-split](../spec-split/SKILL.md) for a plan that spans more than one repository or
more than one piece of work; [spec-run](../spec-run/SKILL.md) for a leaf plan; none for
a design.
