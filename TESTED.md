# Tested setups

This file records which setups we tested, how, and when. It is not code coverage. Add a
row or change a level each time you test a setup.

## Levels

- **Live**: we ran the full action on a real machine, from Obsidian or the binary, with a
  real agent.
- **Probe**: we opened the real terminal with the real agent command through
  `scripts/probe-launch.mjs` of `obsidian-almagest`. The probe runs `<agent> --version` in place of a
  session. It proves that the terminal opens, that the login shell finds the command
  (shell functions too), and that the command runs in the vault folder.
- **Unit**: only unit tests check the command that Almagest builds. Nobody opened the
  terminal.
- **None**: not tested.

## Machine of record

| Date       | OS                     | Obsidian | Claude Code | Codex   | Almagest |
| ---------- | ---------------------- | -------- | ----------- | ------- | ----- |
| 2026-10-01 | macOS (Darwin 25.6.0)  | 1.13.7   | 2.1.287     | 0.155.1 | 8.1.1 |
| 2026-10-04 | macOS (Darwin 25.6.0)  | 1.13.7   | 2.1.287     | 0.155.1, 0.160.0 | 8.1.1 at `a623068` |

## Agent preferences

| Case                                                         | Level       |
| ------------------------------------------------------------ | ----------- |
| `almagest config`, `config set`, `config unset`, global and vault | Live (CLI)  |
| The vault file wins over the global file, key by key         | Unit, Live  |
| A missing key falls back to the global file, then the default | Unit, Live  |
| An unknown key or a bad value is an error with the allowed values | Unit, Live (CLI) |
| The settings tab shows both files and writes either one      | Live        |
| The 8.0.2 plugin settings move into the vault file once      | Unit, Live  |
| `agent: codex` from the config starts Codex from Start agent | Unit        |

## Codex plugin (2026-10-04)

Codex 0.155.1 and 0.160.0, with the plugin installed from a `git archive` of `a623068`
into a scratch `CODEX_HOME` per version, and a scratch `HOME` whose `~/.almagest/bin` held the built binary.
No Codex session ran: the checks used `codex mcp list --json`, `codex app-server`, and
`almagest doctor`.

| Case                                                                   | Level            | Result |
| ---------------------------------------------------------------------- | ---------------- | ------ |
| `codex mcp list --json` lists the almagest entry with no `${` and no `cwd` | Live (Codex CLI) | `/bin/sh -c …` with 0 `${` on both versions, `env_vars` `ALMAGEST_BIN`, `ALMAGEST_HOME`, `ALMAGEST_VAULT`. At `84f33f3` the script still held `${ALMAGEST_BIN:-}` and `${ALMAGEST_HOME:-…}` (round 1, fixed in `63f61e6`) |
| That command, in a vault with only `HOME` and `PATH`, serves the tools | Live (Codex CLI) | `initialize` 8.1.1, 9 tools, `vault status` answered for that vault |
| `doctor` on the 8.1.1 plugin (`3639ae5`)                                | Live (Codex CLI) | `FAIL codex server`: the `${CLAUDE_PLUGIN_ROOT}` placeholder |
| `doctor` on this plugin, fresh install                                 | Live (Codex CLI) | `ok codex server` (9 tools), `FAIL codex hooks` (8 untrusted), on both versions |
| The Codex update hint for a Git and for a local marketplace           | Live (Codex CLI) | Git: upgrade, remove, add; local: remove and add ran (upgrade fails there) |
| `doctor` after the 8 hooks are trusted                                 | Live (Codex CLI) | `ok codex hooks`, exit 0 |
| `doctor` after one cached hook changes                                 | Live (Codex CLI) | `FAIL codex hooks` (1 modified) |
| `setup --agent codex` on an installed plugin                           | Live (Codex CLI) | prints the `hooks` line |
| Trust through `/hooks` in the Codex TUI                                | None             | the scratch home's `config.toml` got `[hooks.state."<key>"] trusted_hash` entries from `hooks/list` instead |
| A Codex session calls an almagest tool                                    | None             | |

To repeat it: `make build`; extract `git archive HEAD` into a folder and commit it there;
then, with `CODEX_HOME` set to an empty folder, run `codex plugin marketplace add <folder>`
and `codex plugin add almagest@nathanaday-almagest`. Copy the binary to
`<scratch home>/.almagest/bin/`, and run `doctor` with `env -i HOME=<scratch home>
PATH=/usr/local/bin:/usr/bin:/bin CODEX_HOME=… CLAUDE_CONFIG_DIR=<empty folder>`.

## The Obsidian plugin

The terminal launches, Resume, and the end-to-end tests in a separate Obsidian are in
the TESTED.md of `obsidian-almagest`.
