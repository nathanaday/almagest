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
| 2026-10-01 | macOS (Darwin 25.6.0)  | 1.13.7   | 2.1.287     | 0.155.1 | 8.1.0 |

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

## How to test a setup

1. Build the plugin: `cd obsidian && npm install`.
2. Run the probe: `node scripts/probe-launch.mjs wezterm claude-work`. The first
   argument is the terminal (`terminal`, `iterm`, `wezterm`, `ghostty`, or `custom`). The
   second is the agent command. For `custom`, the third is the template.
3. The script prints `PASS` or `FAIL`. On `PASS`, change the cell to **Probe** and update
   the machine of record.
4. For **Live**, start a real session from Obsidian with Start agent, and resume it from
   the sessions pane.
