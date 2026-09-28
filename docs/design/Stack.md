
# Stack

Three parts, one version number:

| Part | Is | Holds |
|---|---|---|
| the binary `atlas` | Go, one static file | the MCP server, the CLI, the hooks: every rule and every write |
| the agent plugin | files for Claude Code and Codex | `.mcp.json`, `hooks/hooks.json`, the skills, the agents, the references |
| the Obsidian plugin | TypeScript, thin | styles, the change bar, the sessions pane, mentions ([[Obsidian Plugin]]) |

## Why Go, and not TypeScript for all three

TypeScript everywhere would put the core and the Obsidian plugin in one language. It buys little here, because the Obsidian plugin reads frontmatter through Obsidian's own metadata cache and makes no write of its own. So there is little code to share.

Go buys three things:

1. **Fast hooks.** A hook runs on every tool call. A Go binary starts in a few milliseconds; Node takes tens. The target is under 50 ms per hook.
2. **One static file**, with no runtime to install.
3. **V1 as a guide.** The parts below solved hard problems once, with tests.

## What V1 lends

Each is a guide to rebuild from, not code to copy whole:

| V1 package | Lends | V2 use |
|---|---|---|
| `gitx` | scoped git commands with stable parsing | every commit, snapshot, and undo |
| `txn` | atomic writes, recovery, undo by restoring from the parent | `change` apply and undo |
| `threads` | stage from the furthest document; `setField`, which edits one frontmatter line and keeps the rest; `replaceLead` | `thread`, and the code-owned fields of every document |
| `lint` | the link parser and resolver | `lint`, the link checks of `change`, rename rewrites |
| `capture` | content hashing, the measure of a PDF or an outline | `source` |
| `hooks` | the guard, including Codex's `apply_patch` paths | the guard |
| `place` | finding the entity from any working directory | finding the vault |

New in V2: the document model (types, schemas, ids, titles, links), `search` (BM25), `match`, the change document's format and parser, the session hooks, and `context`.

## Packages

```text
cmd/atlas/            main
internal/doc/         a document: frontmatter, body, lead callout; read and write one field; ids; titles
internal/schema/      the thirteen types: fields, owners, body sections, checks
internal/vault/       the layout, Atlas.md, init, sync, the config file, finding the vault
internal/gitx/        git commands, the lock
internal/change/      the change document: propose, validate, apply, reject, undo, pending
internal/source/      capture, repository snapshots, chunks, read
internal/search/      BM25 over the documents
internal/match/       normalize, join, match
internal/scope/       the context graph: chains, policies, repository facts
internal/threads/     the thread documents, stage, sync
internal/sessions/    the session documents, keyed by harness id
internal/lint/        the checks
internal/mcpserver/   the eight tools, thin over the packages above
internal/hooks/       the nine hook commands
internal/cli/         one method per command
```

Each tool is a thin layer over one package, and each CLI command over the same function. The Obsidian plugin calls the CLI.

## Dependencies

`gopkg.in/yaml.v3` and the official MCP Go SDK. Nothing else. `git` is a runtime requirement. macOS and Linux.

## Install

```bash
npx atlas-obsidian setup
```

- The npm package carries the binary for each platform as an optional dependency, the pattern esbuild uses. `setup` puts it at `~/.atlas/bin/atlas`.
- `setup` adds the agent plugin to Claude Code (`claude plugin marketplace add`, then `claude plugin install`) or to Codex (`--agent codex`).
- `setup` then runs [[atlas-onboard]] in a new session, or `atlas vault init` with `--yes`.
- `vault init` copies the Obsidian plugin into the vault's `.obsidian/plugins/atlas/`. You turn it on once in Obsidian.
- `atlas doctor` checks the binary, both plugins, their versions, git, the config, and every vault.

## Tests

- Every package is tested against temporary vaults with real git. Tests never touch a real `~/.atlas`, and skip when `git` is missing.
- The MCP tools are tested in-process over the SDK's in-memory transport.
- Each hook is tested with the event JSON of each host, as fixtures.
- A new vault lints clean.
- A plugin test holds the skills to their form ([[Skills#Testing the skills]]).
- End-to-end runs in a scratch vault with scratch repositories, one per skill.

## Build order

Each phase ends usable, and each is tested before the next begins.

1. **Documents.** `doc`, `schema`, `vault` init and sync, `lint` (schema checks). A vault that lints clean.
2. **The wiki's write path.** `change`, `search`, `scope` (`context`), the guard's wiki rules. Then [[repo-link]], [[wiki-edit]], and [[wiki-query]] work.
3. **Threads and sessions.** `threads`, `sessions`, every hook, the thread rule. Then [[thread-work]] runs from a sentence to a receipt, with a session document for each session and subagent.
4. **The pipeline.** `source`, `match`, the two agents. Then [[wiki-ingest]], [[wiki-sync]], and [[repo-ingest]] work.
5. **Across scopes.** [[wiki-rollup]]; the deep run of [[wiki-review]].
6. **Obsidian.** The plugin's phase 1, then phase 2.
