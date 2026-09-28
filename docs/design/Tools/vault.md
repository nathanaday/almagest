
# vault

> The state of the vault in one read, and the two writes that make a vault and heal it.

| Action | Takes | Returns | Writes |
|---|---|---|---|
| `status` (default) | nothing | [[Vault Status]] | nothing |
| `init` | `name`, `path`, `areas`, `description` | [[Vault Status]] | the layout, `Atlas.md`, the four Bases, `.obsidian/` with the plugin, `git init` when the folder is no repository, one `setup` commit; adds the path to `~/.atlas/config.json` |
| `sync` | nothing | what it changed | derived fields only: recovery of a change left `applying`, thread stages and callouts, `active` flags, lost sessions, `.claude/settings.local.json` |
| `mention` | a mention (`path` and `line`) and a `link` | the closed mention | checks the mention's box and appends ` → [[link]]` ([[Obsidian Plugin#Mentions]]) |

`status` reads every document's frontmatter, the `## Absorbed` table of each applied change (for `pending`), and the bodies of notes outside `wiki/`, `changes/`, `sessions/`, and `scratchpad/` (for mentions). It runs the quick checks of [[lint]] for the `problems` count. The server may cache what it parsed by modification time. The session-start hook calls it and prints the result.

`sync` is safe at any time. It writes a file only when its derived content differs, never changes `updated`, and makes no commit. The session-start hook and the Obsidian plugin's refresh command call it.

Refusals:

- `init` adopts a folder that already holds notes, and a folder that is already the root of a git repository: it adds the layout and never moves or edits a file that is there. Your notes stay untyped until a skill or you give them a type.
- `init` refuses a folder inside another repository's work tree (the vault must be a repository root), a folder that already holds an `Atlas.md`, and a path already listed in the config.
