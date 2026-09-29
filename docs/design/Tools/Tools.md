
# Tools

A tool is a fact or a commit. Code owns every question that two correct runs must answer the same way, and every path that changes bytes. No tool writes prose: a tool that files text files the text the model gives it.

There are eight tools, served by one MCP server named `atlas`. Tools are nouns and stay few, because every tool's description sits in every session's context.

| Tool | Takes | Returns | Writes |
|---|---|---|---|
| [[vault]] | nothing; a name and path for `init`; a mention to close | [[Vault Status]] | `init`, `sync`, `mention` |
| [[search]] | [[Query]] | [[Hits]] | nothing |
| [[context]] | a scope or a path | [[Context Chain]] | nothing |
| [[match]] | [[Item Map]]s, or page ids | [[Match Map]] | nothing |
| [[source]] | [[Capture Request]]; a document and a chunk | captured [[Doc Ref]]s; [[Chunk]]s; a [[Text Blob]] | `capture`: one commit |
| [[change]] | [[Wiki Change Plan]]; a change id | [[Change Preview]] | `propose`: the change document; `apply`, `undo`: one commit each |
| [[thread]] | [[Thread Write]]; a thread id | [[Thread View]]; the Board | every write: one commit |
| [[lint]] | a scope, optional | [[Findings]] | nothing |

Four read, four write. The writers are the only code paths that change the vault, apart from the hooks, which write session documents and a few linked fields, and `vault sync`, which heals derived fields.

## Conventions

1. **One action field.** A tool with more than one action takes `action` and the fields of that action. The default action is the read.
2. **The vault.** Every tool takes an optional `vault` (a path or a name from `~/.atlas/config.json`). Without it, the server resolves: the vault at or above the working directory (the nearest `Atlas.md` of type `vault`); then the vault that links a repository holding the working directory; then it refuses, naming both checks.
3. **Ids in, Doc Refs out.** A tool takes ids, or titles that resolve to exactly one document. It returns [[Doc Ref]]s, never bare paths.
4. **Refusals teach.** A refusal names the rule and the call that would succeed: "thread T2 depends on T1, which is open; call thread task T1 done first".
5. **Commits.** `source` capture, `change` apply and undo, and every `thread` write take `.git/atlas.lock`, recover a change a crash left `applying`, commit a dirty tree as `snapshot`, and end in one commit ([[Vault Layout#Git]]). `change` propose and reject, `vault` sync and mention, and the hooks write without a commit, under the same lock; the next snapshot keeps what they wrote.
6. **Sync.** Every `thread` write and every `change` apply ends in the sync of derived fields that it touched.
7. **No session id.** No tool needs one. The hooks link each call to its session ([[Sessions#Links]]), and the agent writes its own session document's prose with Edit.

## One backend, three front ends

An action lives once, as a function in the core. The MCP tool, the CLI command, and the Obsidian plugin reach the same function. Every tool action has a CLI command of the same name:

```text
atlas-obsidian vault [status|init|sync|mention]
atlas-obsidian search TEXT [--type T]... [--scope S] [--state K=V]... [--limit N]
atlas-obsidian context [SCOPE] [--path P]
atlas-obsidian match --items FILE.json | --pages ID... [--within S] [--siblings]
atlas-obsidian source capture [--inbox NAME]... | [--text FILE --title T] | [--repository R] [--scope S]
atlas-obsidian source chunks DOC
atlas-obsidian source read DOC CHUNK
atlas-obsidian change propose FILE.json | show ID | apply ID | reject ID --reason R | undo ID
atlas-obsidian thread [list] | show T | open TEXT... | attach T | file T PART | tasks T FILE.json | task ID DO | set T ... | reopen T
atlas-obsidian lint [SCOPE] [--json]
atlas-obsidian hook EVENT                        (the hooks; reads the event JSON on stdin)
```

The CLI adds commands that no tool needs: `setup`, `doctor`, `version`, and `open` (open a vault or a document in Obsidian).

## What is not a tool

| Wanted | Where it lives | Why |
|---|---|---|
| "Map Text": extract items from text | [[wiki-extract]], an agent | judgment |
| "Wiki Draft": decide and write pages | [[wiki-draft]], an agent | judgment |
| "Get Conventions for Task" | [[context]] gives the candidates; the conventions step decides | candidates are a fact; relevance is judgment |
| "Thread search" | [[search]] with `types: [stub]` | one search over every document |
| reading a page | the host's Read, Grep, and Glob | they already work |
| the board, the index, the log | `Threads.base`, `Wiki.base`, `Changes.base` | Obsidian renders them from the frontmatter |
