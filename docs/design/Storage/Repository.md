# Repository

A git repository on this machine, linked into the vault. The repository document is the link: agents may work in a repository only when a document names its path. It is also the card for the repository, with its git facts, and the description of its code.

## Fields

```yaml
---
id: doc-h6t2vc
type: repository
description: "The p3 edge service: runs detection on the vehicle's camera feed."
tags: [work/p3, go]
aliases: [p3 edge]
created: 2026-09-27T10:12:44
updated: 2026-09-29T09:15:40
refreshed: 2026-09-29T09:15:40
defines: work/p3/p3-edge            # the tag of this repository's own knowledge
path: "~/code/p3-edge"              # the model gives it; code checks it
unlinked: false                     # true: kept for its links; agents no longer work in it
# owned by code:
remote: "git@github.com:acme/p3-edge.git"
branch: main
head: 4ac19e2
head_time: 2026-09-28T17:02:11      # the commit time of head
described: 9e41c07                  # the commit the body describes
behind: 12                          # commits from described to head
---
```

## Its tag

A repository defines one tag, the tag of its own knowledge ([[Documents#Tag pages]]). A topic about a component of `p3-edge` holds `work/p3/p3-edge`, and a lookup for that tag finds it.

- [[repo-link]] proposes the tag: the repository's name under its main category tag (`work/p3` → `work/p3/p3-edge`), or the name alone.
- The repository holds the parent of its tag in `tags`, like every tag page.
- A policy for this one repository holds this tag ([[Topic#Policy]]).

## Git facts

Code reads git and writes `remote`, `branch`, `head`, `head_time`, and `behind`. It writes them when the document is created, and at each `vault sync` where one of them changed. Then it sets `refreshed`. A sync that finds the same facts writes nothing, so the vault gets no commit while the repository is quiet.

The state of the working tree changes with every save, so it is not in the file. The Obsidian plugin renders it live in the document ([[#Body]]).

`described` is the commit that the body describes. Code sets it when a change that absorbs a snapshot of this repository applies. `behind` counts the commits since, so an agent knows when the description is behind.

## Lead callout

```markdown
> [!repository] `~/code/p3-edge`
> `main` at `4ac19e2`, 2026-09-28 17:02 · `github.com/acme/p3-edge`
> Described at `9e41c07`, 12 commits behind · tag #work/p3/p3-edge
```

A path that is gone or is no git work tree turns the callout into a warning: `> [!repository-missing] ~/code/p3-edge is gone`.

## Body

1. The lead callout.
2. **Live status**, code's: an `atlas-repo` code block. The Obsidian plugin renders it as a panel: the files changed and not committed, the commits ahead of and behind the remote, the current branch when it is not `branch`, and the last three commits. The plugin reads the facts from the binary (`atlas-obsidian context <id> --json`) when the document opens, and on a click. Without the plugin, the block shows as a line of code that names the path.
3. `## What it is`: what the repository does and for whom. The model's.
4. `## How it is built`: the languages, the frameworks, the build and test commands.
5. `## Layout`: the main folders and what each holds.
6. `## Components`: links to entity topics, one line each.
7. `## Instructions`: the paths of `AGENTS.md` and `CLAUDE.md`, and what they require, in short.
8. `## Work`, code's: an inline Base of the specs that name this repository, open first. Obsidian renders it live, so the file does not change when a spec does.
9. `## Knowledge`, code's: an inline Base of the documents that hold this repository's tag, by type.
10. `## Notes`: yours.

[[repo-link]] writes sections 3 and 7 in short. [[repo-ingest]] writes 3 to 7 in full.

## Rules

- `path` must be the root of a git work tree, outside the vault, and no other repository document may hold it. `change` refuses a path that breaks a rule, and names it.
- The model never writes `remote`, `branch`, `head`, `head_time`, `described`, or `behind`.
- `defines` is unique, like every tag page's.
- `vault sync` lists every repository's path in `.claude/settings.local.json` ([[Vault Layout#Settings for the harness]]).
- Unlinking ([[repo-unlink]]) keeps the document, so the plans that name it keep a live link: a change sets `unlinked: true` and empties `path`. Code then drops the git facts, sync takes the path out of the harness settings, the callout reads `> [!repository-missing] Unlinked`, and the guard no longer treats the old folder as a linked repository. `change` refuses to remove a repository document while any spec names it.
- Unlinking never touches the repository on disk.
