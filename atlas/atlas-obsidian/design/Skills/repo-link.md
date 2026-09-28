
# repo-link

> Creates a new repository link document: a repository page that points at a git repository on this machine, under the right area.

**Use for**: link this repository, add a repo, work with ~/code/x, connect a repository.

**Tools**: `context`, `search`, `change`. **References**: `references/changes.md`, `references/pages.md`.

## Procedure

1. For each path: check that it exists and is the root of a git work tree, and that `context` with `path` does not already resolve it (then it is linked). `change` refuses a path inside the vault.
2. Choose each repository's parent, by the vault's `areas` setting:
   - `many`: propose an area for each cluster (by name, remote, or the areas that exist), and create it when none fits;
   - `few`: propose a top-level area, or the vault;
   - `manual`: ask, offering the existing areas and "none".
3. Read the repository's README and instruction files, and write its page: a `description` of one sentence, `aliases`, and a short `## What it is`. [[repo-ingest]] writes the rest. Code fills `path`, `remote`, and `branch`.
4. Build one [[Wiki Change Plan]]: the new area pages, then the repository pages.

## Gate

Propose the change. Show the preview: each repository with its parent, and each new area. Wait for the yes, then apply. Apply also lists the repositories in `.claude/settings.json`, so a session in the vault can edit them.

## Hand off

[[repo-ingest]], one repository at a time.
