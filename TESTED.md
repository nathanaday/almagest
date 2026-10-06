# Tested setups

This file records which setups we tested, how, and when. It is not code coverage. Add a
row or change a level each time you test a setup.

## Levels

- **Live**: we ran the full action on a real machine, from Obsidian or the binary, with a
  real agent.
- **Probe**: we opened the real terminal with the real agent command through
  `obsidian/scripts/probe-launch.mjs`. The probe runs `<agent> --version` in place of a
  session. It proves that the terminal opens, that the login shell finds the command
  (shell functions too), and that the command runs in the vault folder.
- **Unit**: only unit tests check the command that Atlas builds. Nobody opened the
  terminal.
- **None**: not tested.

## Machine of record

| Date       | OS                     | Obsidian | Claude Code | Codex   | Atlas |
| ---------- | ---------------------- | -------- | ----------- | ------- | ----- |
| 2026-10-01 | macOS (Darwin 25.6.0)  | 1.13.7   | 2.1.287     | 0.155.1 | 8.1.1 |
| 2026-10-04 | macOS (Darwin 25.6.0)  | 1.13.7   | 2.1.287     | 0.155.1, 0.160.0 | 8.1.1 at `a623068` |

## Start agent: agent × terminal (macOS)

| Agent command                   | Terminal.app | iTerm2 | WezTerm | Ghostty | Custom |
| ------------------------------- | ------------ | ------ | ------- | ------- | ------ |
| `claude`                        | Probe        | Unit   | Probe   | Unit    | Probe  |
| A shell function (`claude-work`) | Probe        | Unit   | Live¹   | Unit    | None   |
| `codex`                         | Probe        | Unit   | Probe   | Unit    | None   |

¹ Start agent from the plugin in a scratch Obsidian, with the vault's command set
through the settings tab, and WezTerm from the global file. A probe stood in for the
session.

The Custom column used WezTerm as the custom program:
`/Applications/WezTerm.app/Contents/MacOS/wezterm start -- /bin/zsh -lic {command}`.

iTerm2 and Ghostty were not installed on the machine of record. When the chosen app is
missing, the plugin shows a notice and copies the command.

Off macOS, the plugin copies the command to the clipboard and opens no terminal. Linux
and Windows: **None**.

## Resume

| Session                                     | Level | Notes                                                    |
| ------------------------------------------- | ----- | -------------------------------------------------------- |
| Claude Code, default account (`~/.claude`)  | Live  | 8.0.2. The resume command ran and opened the conversation. |
| Claude Code, second account (`CLAUDE_CONFIG_DIR`) | Unit | The command is built; nobody ran it.                     |
| Codex                                       | Unit  | `codex resume <id>` is built; nobody ran it.             |

## Agent preferences

| Case                                                         | Level       |
| ------------------------------------------------------------ | ----------- |
| `atlas-obsidian config`, `config set`, `config unset`, global and vault | Live (CLI)  |
| The vault file wins over the global file, key by key         | Unit, Live  |
| A missing key falls back to the global file, then the default | Unit, Live  |
| An unknown key or a bad value is an error with the allowed values | Unit, Live (CLI) |
| The settings tab shows both files and writes either one      | Live        |
| The 8.0.2 plugin settings move into the vault file once      | Unit, Live  |
| `agent: codex` from the config starts Codex from Start agent | Unit        |

## Codex plugin (2026-10-04)

Codex 0.155.1 and 0.160.0, with the plugin installed from a `git archive` of `a623068`
into a scratch `CODEX_HOME` per version, and a scratch `HOME` whose `~/.atlas/bin` held the built binary.
No Codex session ran: the checks used `codex mcp list --json`, `codex app-server`, and
`atlas-obsidian doctor`.

| Case                                                                   | Level            | Result |
| ---------------------------------------------------------------------- | ---------------- | ------ |
| `codex mcp list --json` lists the atlas entry with no `${` and no `cwd` | Live (Codex CLI) | `/bin/sh -c …` with 0 `${` on both versions, `env_vars` `ATLAS_BIN`, `ATLAS_HOME`, `ATLAS_VAULT`. At `84f33f3` the script still held `${ATLAS_BIN:-}` and `${ATLAS_HOME:-…}` (round 1, fixed in `63f61e6`) |
| That command, in a vault with only `HOME` and `PATH`, serves the tools | Live (Codex CLI) | `initialize` 8.1.1, 9 tools, `vault status` answered for that vault |
| `doctor` on the 8.1.1 plugin (`3639ae5`)                                | Live (Codex CLI) | `FAIL codex server`: the `${CLAUDE_PLUGIN_ROOT}` placeholder |
| `doctor` on this plugin, fresh install                                 | Live (Codex CLI) | `ok codex server` (9 tools), `FAIL codex hooks` (8 untrusted), on both versions |
| The Codex update hint for a Git and for a local marketplace           | Live (Codex CLI) | Git: upgrade, remove, add; local: remove and add ran (upgrade fails there) |
| `doctor` after the 8 hooks are trusted                                 | Live (Codex CLI) | `ok codex hooks`, exit 0 |
| `doctor` after one cached hook changes                                 | Live (Codex CLI) | `FAIL codex hooks` (1 modified) |
| `setup --agent codex` on an installed plugin                           | Live (Codex CLI) | prints the `hooks` line |
| Trust through `/hooks` in the Codex TUI                                | None             | the scratch home's `config.toml` got `[hooks.state."<key>"] trusted_hash` entries from `hooks/list` instead |
| A Codex session calls an atlas tool                                    | None             | |

To repeat it: `make build`; extract `git archive HEAD` into a folder and commit it there;
then, with `CODEX_HOME` set to an empty folder, run `codex plugin marketplace add <folder>`
and `codex plugin add atlas-obsidian@nathanaday-atlas-obsidian`. Copy the binary to
`<scratch home>/.atlas/bin/`, and run `doctor` with `env -i HOME=<scratch home>
PATH=/usr/local/bin:/usr/bin:/bin CODEX_HOME=… CLAUDE_CONFIG_DIR=<empty folder>`.

## Obsidian end-to-end tests

Date: 2026-10-06. Obsidian 1.14.4 (the app update, over the 1.8.7 installer), macOS
(Darwin 25.6.0), Go 1.24.2, Node 22.14.0. Atlas 10.0.0: the working tree over `fbf8e52`.

`cd obsidian && npm run test:obsidian` builds the plugin, builds `atlas-obsidian` from
this checkout into a temporary folder, and runs `test/obsidian/plugin.test.ts`. Each test
makes its own vault with `vault init`, with `ATLAS_HOME` in a temporary folder. The
harness copies `obsidian/dist` over the plugin that `vault init` installs, and sets
`binaryPath` to the built binary. Then it starts a separate Obsidian with a temporary
profile (`--user-data-dir`) and drives it with Playwright over the DevTools protocol. The
user's Obsidian, `~/.atlas`, and vaults stay as they are. `npm test` does not need
Obsidian. The suite takes 24 to 26 seconds. It passed three runs in a row.

| Test | What it proves |
| ---- | -------------- |
| Loads in a 10.0 vault | The plugin loads with no console error. A manual sync runs the binary and shows its notice. With every folder open, no element in the file explorer has an `atlas-` class or a `data-atlas` attribute, and no rule of the plugin's `styles.css` matches an element there. |
| Reads the layout at Obsidian's start, 10.0 | Obsidian quits and starts again with its metadata index deleted, so the plugin loads at start, as it does for a user. It shows no notice and logs no error. |
| Reads the layout at Obsidian's start, 9.0 | The same start in a vault whose `Atlas.md` says `layout: 5`. The plugin shows one notice, which names the 9.0 layout. |
| Approves a change | `change propose` from the CLI writes a change document. In live preview, the widget shows Approve and Cancel. A click on Approve writes the topic file, sets `status: applied`, and commits `change: Add Alpha`. The widget then shows "Applied <time>." with no buttons. |
| Cancels a change | In reading view, Cancel opens the modal. The reason typed there goes into `reason`, the status becomes `rejected`, and no topic file is written. The widget shows "Rejected: <reason>". |
| Quiet snapshots | With `snapshotQuietSeconds: 2`, a note is created and then changed through `app.vault`. One second after each edit, git holds no snapshot, so each edit starts the quiet period again. Then one commit "snapshot: N files edited by hand" holds the last text, and the tree is clean. In some runs the `vault sync --views` that the same edit starts holds the lock; the snapshot then runs again after the next quiet period, as designed. |
| Offers the migration | In a vault with `layout: 5`, the notice names the 9.0 layout. "Show the migration" opens the dry run in the modal. Migrate sets `layout: 6` and commits `layout: migrate to 10.0`. |

Found with these tests (harness only, not Atlas): Obsidian ignores SIGTERM for about one
second after it starts, so the harness kills Obsidian when it deletes the profile. It uses
SIGTERM only for a restart, which must keep the profile. A large vault (6,000 notes still
in the index queue at load) still gave the right layout notice.

## How to test a setup

1. Install the plugin's dependencies: `cd obsidian && npm install`. That is enough for the
   probe, which bundles `src/agents.ts` itself. To build the plugin, run `npm run build`
   there, or `make obsidian` at the root, which also copies it into the binary.
2. Run the probe: `node scripts/probe-launch.mjs wezterm claude-work`. The first
   argument is the terminal (`terminal`, `iterm`, `wezterm`, `ghostty`, or `custom`). The
   second is the agent command. For `custom`, the third is the template.
3. The script prints `PASS` or `FAIL`. On `PASS`, change the cell to **Probe** and update
   the machine of record.
4. For **Live**, start a real session from Obsidian with Start agent, and resume it from
   the sessions pane.
