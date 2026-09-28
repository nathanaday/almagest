
# vault

> The state of the vault in one read, and the two writes that make a vault and heal it.

| Action | Takes | Returns | Writes |
|---|---|---|---|
| `status` (default) | nothing | [[Vault Status]] | nothing |
| `init` | `name`, `path` | [[Vault Status]] | the layout, `Atlas.md`, the four Bases, `.obsidian/` with the plugin, `git init`, one `setup` commit; adds the path to `~/.atlas/config.json` |
| `sync` | nothing | what it changed | derived fields only: thread stages and callouts, `active` flags, lost sessions, `.claude/settings.json` |

`status` is cheap: it reads frontmatter only, and runs the quick checks of [[lint]] for the `problems` count. The session-start hook calls it and prints the result.

`sync` is safe at any time. It writes a file only when its derived content differs, never changes `updated`, and makes no commit. The session-start hook and the Obsidian plugin's refresh command call it.

Refusals:

- `init` refuses a folder inside a git work tree, a folder that is not empty (unless it holds only `.obsidian/`), and a path already listed in the config.
