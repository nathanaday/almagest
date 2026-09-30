# vault

> The state of the vault in one read, and the writes that make a vault and keep it healthy.

| Action | Takes | Returns | Writes |
|---|---|---|---|
| `status` (default) | nothing | [[Vault Status]] | nothing |
| `init` | `name`, `path`, `tagging` (`open` or `known`), `description` | [[Vault Status]] | the layout, `Atlas.md`, the two Bases, the views, `.obsidian/` with the plugin and its settings, `git init` when the folder is no repository, one `setup` commit; adds the path to `~/.atlas/config.json` |
| `sync` | optional `views: true` (the views alone) | what it changed | derived fields and the views, with no commit ([[#sync]]) |
| `mention` | a mention (`path` and `line`) and a `link` | the closed mention | checks the mention's box and appends ` → [[link]]` ([[Obsidian Plugin#Mentions]]) |

`vault migrate` is a CLI command and no tool action, because only you may run it ([[Migration]]).

## status

`status` reads every document's frontmatter, the `## Absorbed` table of each applied change (for `pending`), and the bodies of notes outside `wiki/documents/`, `changes/`, `sessions/`, `views/`, and `scratchpad/` (for mentions). It runs the quick checks of [[lint]] for the `problems` count. The server may cache what it parsed by modification time. The session-start hook calls it and prints the result.

## sync

`sync` makes every derived part agree with the documents. It is safe at any time, writes a file only when its derived content differs, never changes `updated` for a derived field, and makes no commit.

1. Recover a change left `applying`.
2. Move a typed document found elsewhere under `wiki/` into `wiki/documents/` ([[Vault Layout#Hand moves]]).
3. The status of each stub and spec, from its events; `blocked`, `parts`, `root`, and `active`.
4. The git facts of each repository ([[Repository#Git facts]]); and `refreshed` where they changed.
5. The status of each source, from the applied changes.
6. Lost sessions ([[Sessions#Status]]).
7. The lead callout of each document whose facts changed.
8. `.claude/settings.local.json` ([[Vault Layout#Settings for the harness]]).
9. The views ([[Views]]).

With `views: true`, sync runs steps 3, 7, and 9 only. That is what the Obsidian plugin runs on each change, since it reads no git.

The session-start hook runs the whole sync. Every write tool runs the steps its write touched.

Refusals:

- `init` adopts a folder that already holds notes, and a folder that is already the root of a git repository: it adds the layout and never moves or edits a file that is there. Your notes stay untyped until a skill or you give them a type.
- `init` refuses a folder inside another repository's work tree (the vault must be a repository root), a folder that already holds an `Atlas.md`, and a path already listed in the config.
- Every write refuses on a vault whose `layout` is below 3, and names `atlas-obsidian vault migrate`.
