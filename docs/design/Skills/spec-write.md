# spec-write

> Turn a stub or a request into a spec: a plan (what will be true when the work is done, and why), or a design (how a part must work). Research first; ask only what research cannot answer.

**Use for**: spec this, define this, what should this be, design this, flesh out the stub, requirements, how should this work.

**Tools**: `work` (show, promote, spec), `context`, `search`. **References**: `references/work.md`, `references/conventions.md`.

## Procedure

1. `work` show: read the stub, its idea, and its notes. With no stub, read the request.
2. Choose the kind: `plan` for work that ends; `design` for how a part must keep working. A request for both is a design and a plan that `implements` it.
3. `context` for each repository: the instruction files, the git facts, the tag pages, and the candidate policies.
4. Research: read the code the work touches, and the wiki (`search`, then Read) for what the vault already knows, designs included.
5. Decide what the research decides. Ask only what it cannot answer and a wrong guess would change, in one message, each question with a recommended answer. Ask no fixed number of questions.
6. **The conventions step**: for each candidate policy, decide whether it binds this work. Keep the ones that do, each with one line on why.
7. Write the spec:
   - a plan: `## Goal`, `## Done when` (a list a reviewer can check), `## Decisions` (each with its reason), `## Out of scope`, `## Conventions`, `## Open questions`; `repositories`, `tags`, `priority`;
   - a design: `## Purpose`, `## Behavior`, `## Interfaces`, `## Constraints`, `## Decisions`, `## Open questions`; `repositories`, `tags`.
8. File it:
   - the stub becomes this one spec → `work promote` (in place; the stub's words stay as `## Origin`);
   - the stub becomes several documents → `work spec` with `from` and `resolve: true` on the call that creates the last;
   - no stub → `work spec`.

## Gate

Link the spec and say its goal and its done-when list (or its behavior) in a few lines. Wait for the user to agree or to edit it. An edit in Obsidian counts; read the spec again before you go on.

## Hand off

[[spec-split]] for a plan that spans more than one repository or more than one piece of work; [[spec-run]] for a leaf plan; none for a design.
