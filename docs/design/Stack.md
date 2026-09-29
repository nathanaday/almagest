# Stack

Three parts, one version number:

| Part | Is | Holds |
|---|---|---|
| the binary `atlas-obsidian` | Go, one static file | the MCP server, the CLI, the hooks, the view writer: every rule and every write |
| the agent plugin | files for Claude Code and Codex | `.mcp.json`, `hooks/hooks.json`, the skills, the agents, the references |
| the Obsidian plugin | TypeScript, thin | styles, the views folder, the tag navigator, the repository panel, the change bar, the work buttons, the sessions pane, mentions, graph colors ([[Obsidian Plugin]]) |

## Why Go

1. **Fast hooks.** A hook runs on every tool call. A Go binary starts in a few milliseconds; Node takes tens. The target is under 50 ms per hook.
2. **One static file**, with no runtime to install.
3. **One writer.** The binary writes every document and every view, so the Obsidian plugin shares no write code with it and needs none.

## Packages

What 7.0 keeps, changes, and adds, against the 6.5 tree:

```text
cmd/atlas-obsidian/   main
internal/doc/         kept: frontmatter, body, one-field edits, lead callouts; changed: ids (doc-, and the 6.x prefixes read), times to the second with T
internal/schema/      changed: six types, their kinds, the owners of each field, body sections, the common fields
internal/tags/        new: normalize, the tree, "holds", the vocabulary, policy reach, retag rewrites
internal/links/       kept: the link parser, resolver, and rewrite pass
internal/vault/       changed: layout 3, init, EnsureFolders, sync steps, the config file, finding the vault
internal/migrate/     new: vault migrate from 6.x ([[Migration]]); replaces vault.MigrateLayout
internal/gitx/        kept: git commands, the lock
internal/change/      changed: ops promote, confirm, retag; every path is wiki/documents/<title>.md; place and folder moves go
internal/work/        new, from threads/: stub, spec, and event writes; status from events; parts, root, active, ready, next
internal/source/      changed: wiki/assets/, tags, resolves, media
internal/search/      changed: filters (types, kinds, tags, status, repository), tags as a ranked field, facets
internal/match/       changed: kinds; across the child tags of a tag
internal/context/     new, from scope/: tag pages, policies by tags, work, instructions, git facts now
internal/views/       new: the view writer ([[Views]])
internal/sessions/    changed: specs, work, events
internal/lint/        changed: the checks in [[Findings]]
internal/mcpserver/   changed: the eight tools; `work` replaces `thread`
internal/hooks/       changed: the guard rules and the touched links of [[Hooks]]
internal/cli/         changed: one method per command, and `vault migrate`
internal/host/        kept: the host's event fields
internal/obsidian/    kept: the Obsidian plugin template that init installs
internal/plugin/      kept: the version checks across the parts
internal/testvault/   changed: builds 7.0 vaults, and 6.5 vaults for the migration tests
```

`internal/scope/` and `internal/threads/` go when their parts have moved. Each tool is a thin layer over one package, and each CLI command over the same function. The Obsidian plugin calls the CLI.

## Dependencies

`gopkg.in/yaml.v3` and the MCP Go SDK, pinned at v1.4.0. Nothing else in Go. `git` is a runtime requirement. macOS and Linux.

## Install

```bash
npx atlas-obsidian setup
```

- The npm package carries the binary for each platform as an optional dependency, the pattern esbuild uses. `setup` puts it at `~/.atlas/bin/atlas-obsidian`. Until the package exists, setup runs from the binary.
- `setup` adds the agent plugin to Claude Code (`claude plugin marketplace add`, then `claude plugin install`) or to Codex (`--agent codex`).
- `setup` then runs [[atlas-onboard]] in a new session, or `atlas-obsidian vault init` with `--yes`.
- `vault init` copies the Obsidian plugin into the vault's `.obsidian/plugins/atlas/`. You turn it on once in Obsidian.
- `atlas-obsidian doctor` checks the binary, both plugins, their versions, git, the config, and every vault, and names a vault that needs `vault migrate`.

## Tests

- Every package is tested against temporary vaults with real git. Tests never touch a real `~/.atlas`, and skip when `git` is missing.
- The MCP tools are tested in-process over the SDK's in-memory transport.
- Each hook is tested with the event JSON of each host, as fixtures.
- A new vault lints clean.
- The view writer is tested on fixed vaults against golden view files, and for idempotence: a second sync writes nothing.
- The migration is tested on 6.5 vaults that the test builds, with every kind of thread, a renamed scope, a superseded receipt, untyped notes, and images in scope folders. After the migration: lint is clean, every link resolves, every document is counted once, every status matches its 6.x stage, and nothing is pending that was absorbed before. Then it runs once on a copy of a real 6.5 vault.
- A plugin test holds the skills to their form ([[Skills#Testing the skills]]).
- End-to-end runs in a scratch vault with scratch repositories, one per skill.

## Build order

7.0 changes the layout, so it ships as one release: a vault migrates once, to the whole of it. The work happens on a branch, in phases. Each phase ends with its tests passing, and each is tested before the next begins.

1. **Documents.** `doc`, `schema`, `tags`, `vault` init and EnsureFolders for layout 3, `lint` (schema and tag checks). A new vault that lints clean.
2. **Knowledge.** `change` with every op, `search` with filters and facets, `context`, the guard's knowledge rules. Then [[repo-link]], [[wiki-edit]], and [[wiki-query]] work.
3. **Work.** `work`, the events, the statuses, `sessions`, every hook, the edit rule. Then [[spec-work]] runs from a sentence to a `completed` event, with a session document for each session and subagent.
4. **Views.** `views`, `vault sync --views`. The views of a fixed vault match their golden files.
5. **The pipeline.** `source`, `match`, the two agents. Then [[wiki-ingest]], [[wiki-sync]], and [[repo-ingest]] work.
6. **Migration.** `migrate`, and its tests.
7. **Obsidian.** Styles, the views folder, sync on change, the tag navigator, the repository panel, the work buttons, the migration notice, the graph modes. Each is checked live in a second Obsidian instance ([[#Checking the Obsidian plugin]]).
8. **Across tags.** [[wiki-map]]; the deep run of [[wiki-review]].
9. **Release.** The version goes to 7.0.0 in every manifest; the design pages hold no departure the build did not record.

## Checking the Obsidian plugin

Run a second Obsidian instance with its own data folder, so the user's app is never touched: copy `obsidian-<version>.asar` from `~/Library/Application Support/obsidian/` into the folder, write `obsidian.json` there with one vault, and run `open -n -a Obsidian --args --user-data-dir=<folder> --remote-debugging-port=9333`. Then evaluate JavaScript and take screenshots through the DevTools protocol at `localhost:9333/json/list`, after `Page.bringToFront`.

Check, on a migrated copy of a real vault: that `file.hasTag` in an inline Base renders the lists the view writer expects; that the excluded files keep `views/` out of the graph; that the `obsidian://search` links of a tag view open the search; and that a click on a tag folder opens its view.
