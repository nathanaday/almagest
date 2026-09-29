# repo-link

> Link a git repository on this machine to the vault: a repository document that points at it, with its tags and its own tag, written as one change.

**Use for**: link this repository, add a repo, work with ~/code/x, connect a repository. Describing its code is [[repo-ingest]].

**Tools**: `context`, `search`, `change`. **References**: `references/changes.md`, `references/pages.md`.

## Procedure

1. For each path: check that it exists and is the root of a git work tree, and that `context` with `path` does not already resolve it (then it is linked). `change` refuses a path inside the vault.
2. Choose each repository's `tags`: the category tags that exist and fit (from the session-start tag list, or `vault`), by its name, its remote, and the repositories linked already. In `tags: known` mode, a tag that no document holds needs the user's yes.
3. Choose its own tag, `defines`: its name under its main category tag (`work/p3` → `work/p3/p3-edge`), or the name alone. `tags` then holds the parent of that tag.
4. Read the repository's README and instruction files, and write its document: a `description` of one sentence, `aliases`, and a short `## What it is` and `## Instructions`. [[repo-ingest]] writes the rest. Code fills `remote`, `branch`, `head`, and the live status block.
5. Build one [[Wiki Change Plan]] with a create per repository. Set `new_tags` when the user agreed to a new tag.

## Gate

Propose the change. Show the preview: each repository with its tags and its own tag, and each new tag. Wait for the yes, then apply. The next `vault sync` lists the repositories in `.claude/settings.local.json`, so later sessions in the vault can edit them. For this session, tell the user to run `/add-dir <path>`, or start a new session.

## Hand off

[[repo-ingest]], one repository at a time.
