
# thread-spec

> Turn a stub into a spec: what will be true when the work is done, and why. Research first; ask only what research cannot answer.

**Use for**: spec this, define this thread, what should this be, design this, flesh out the stub, requirements.

**Tools**: `thread` (show, file), `context`, `search`. **References**: `references/threads.md`, `references/conventions.md`.

## Procedure

1. `thread` show: read the stub and its notes.
2. `context` for each scope: the instruction files, the repository facts, and the candidate policies.
3. Research: read the code the work touches, and the wiki (`search`, then Read) for what the vault already knows.
4. Decide what the research decides. Ask only what it cannot answer and a wrong guess would change, in one message, each question with a recommended answer. Ask no fixed number of questions.
5. **The conventions step** ([[Conventions Check.canvas|Conventions Check]]): for each candidate policy, decide whether it applies to this thread. Keep the ones that do, each with one line on why.
6. `thread` file spec: `## Goal`, `## Done when` (a list a reviewer can check), `## Decisions` (each with its reason), `## Out of scope`, `## Conventions`, `## Open questions`.

## Gate

Link the spec and say its goal and its done-when list in a few lines. Wait for the user to agree or to edit it. An edit in Obsidian counts; read the spec again before you go on.

## Hand off

[[thread-tasks]].
