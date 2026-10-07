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
| 2026-10-07 | macOS (Darwin 25.6.0)  | 1.14.4   | —           | —       | 11.0.0 (Go suite, launcher, release build) |

## Agent preferences

| Case                                                         | Level       |
| ------------------------------------------------------------ | ----------- |
| `almagest config`, `config set`, `config unset`, global and vault | Live (CLI)  |
| The vault file wins over the global file, key by key         | Unit, Live  |
| A missing key falls back to the global file, then the default | Unit, Live  |
| An unknown key or a bad value is an error with the allowed values | Unit, Live (CLI) |
| The settings tab shows both files and writes either one      | Live        |
| `agent: codex` from the config starts Codex from Start agent | Unit        |

## Codex plugin

Not yet run with the launcher in the Codex server entry. Each case below is to repeat.

| Case | Level |
| ---- | ----- |
| `codex mcp list --json` lists the almagest entry: `/bin/sh -c <the launcher>`, no `${`, no `cwd` | None |
| That command, in a vault with only `HOME` and `PATH`, installs the binary and serves the tools | None |
| `doctor`: `ok codex server` on a fresh install, and the hooks' trust before and after `/hooks` | None |
| A Codex session calls an almagest tool | None |

To run them: with `CODEX_HOME` set to an empty folder, run `codex plugin marketplace add
<a clone of this repository>` and `codex plugin add almagest@nathanaday-almagest`, then
run `doctor` with `env -i HOME=<scratch home> PATH=/usr/local/bin:/usr/bin:/bin
CODEX_HOME=… CLAUDE_CONFIG_DIR=<empty folder>`. Set `ALMAGEST_BIN` to `build/almagest`
to test a build that has no release.

## The launcher and the release (2026-10-07)

`bin/almagest` and the Codex server entry run the same launcher (`internal/release`).

| Case | Level | Result |
| ---- | ----- | ------ |
| `ALMAGEST_BIN` first; one that is not executable fails with the reason | Unit | both forms, no PATH lookup |
| The installed binary of the plugin's version, under `ALMAGEST_HOME` or `~/.almagest` | Unit | runs it, not an older version |
| No binary and `ALMAGEST_NO_DOWNLOAD=1`; a platform the plugin does not pin | Unit | fails with the reason |
| A download whose sha256 matches | Unit | installs, links `bin/almagest`, writes `install.log` |
| A download whose sha256 differs | Unit | installs nothing, leaves no file and no link |
| A hook with no binary | Unit | `session-start` says why and passes; other hooks pass quietly |
| The pinned 11.0.0 launcher installs the real 11.0.0 binary from the release folder `make pin` built (`file://`) | Live (macOS arm64) | sha256 matches, `version --json` prints 11.0.0 and protocol 1, the second run uses the installed copy |
| `make release` twice, and from a copy in another folder with another module cache | Live (macOS arm64) | the same checksums |
| The release workflow's build on Linux matches the checksums of a macOS `make pin` | Live (GitHub Actions) | the 11.0.0 release, and every release run on `main` since |

## The Obsidian plugin

The terminal launches, Resume, and the end-to-end tests in a separate Obsidian are in
the TESTED.md of `obsidian-almagest`.
