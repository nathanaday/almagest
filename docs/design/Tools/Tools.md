# Tools

A tool is a fact or a commit. Code owns every question that two correct runs must answer the same way, and every path that changes bytes. No tool writes prose: a tool that files text files the text the model gives it.

There are eight tools, served by one MCP server named `atlas`. Tools are nouns and stay few, because every tool's description sits in every session's context.

| Tool | Takes | Returns | Writes |
|---|---|---|---|
| [[vault]] | nothing; a name and path for `init`; a mention to close | [[Vault Status]] | `init`, `sync`, `mention` |
| [[search]] | [[Query]] | [[Hits]] | nothing |
| [[context]] | a repository, tags, or a path | [[Context Brief]] | nothing |
| [[match]] | [[Item Map]]s, or document ids | [[Match Map]] | nothing |
| [[source]] | [[Capture Request]]; a document and a chunk | captured [[Doc Ref]]s; [[Chunk]]s; a [[Text Blob]] | `capture`: one commit |
| [[change]] | [[Wiki Change Plan]]; a change id | [[Change Preview]] | `propose`: the change document; `apply`, `undo`: one commit each |
| [[work]] | [[Work Write]]; a document id | [[Work View]]; the Board | every write: one commit |
| [[lint]] | tags, optional | [[Findings]] | nothing |

Four tools only read: `search`, `context`, `match`, and `lint`. Four write: `vault`, `source`, `change`, and `work`. The writers are the only code paths that change the vault, apart from the hooks, which write session documents and a few linked fields, and `vault sync`, which heals derived fields and writes the views.

By job ([[Atlas 7#Three jobs, three owners]]):

- **Storage**: `change`, `work`, `source` capture, `vault` init and sync.
- **Lookup**: `search`, `context`, `match`, `source` chunks and read, `lint`, `vault` status.
- **Navigation**: `vault sync` writes the views. No tool reads them.

## Conventions

1. **One action field.** A tool with more than one action takes `action` and the fields of that action. The default action is the read.
2. **The vault.** Every tool takes an optional `vault` (a path or a name from `~/.atlas/config.json`). Without it, the server resolves: the vault at or above the working directory (the nearest `Atlas.md` of type `vault`); then the vault whose repository document holds the working directory; then it refuses, naming both checks.
3. **Ids in, Doc Refs out.** A tool takes ids, or titles that resolve to exactly one document. It returns [[Doc Ref]]s, never bare paths.
4. **The same filters everywhere.** Every tool that lists documents takes the same filters, with the same meaning: `types`, `kinds`, `tags` (all of them), `status`. See [[Query]].
5. **Refusals teach.** A refusal names the rule and the call that would succeed: "Tune the threshold depends on Collect clips, which is open; call work done on Collect clips first".
6. **Commits.** `source` capture, `change` apply and undo, and every `work` write take `.git/atlas.lock`, recover a change a crash left `applying`, commit a dirty tree as `snapshot`, and end in one commit ([[Vault Layout#Git]]). `change` propose and reject, `vault` sync and mention, and the hooks write without a commit, under the same lock; the next snapshot keeps what they wrote.
7. **Sync.** Every `work` write, `change` apply and undo, and capture end in the sync of the derived fields they touched, then the views ([[Views#Freshness]]).
8. **No session id.** No tool needs one. The hooks link each call to its session ([[Sessions#Links]]), and the agent writes its own session document's prose with Edit.
9. **A layout below 3.** Every write refuses and names `atlas-obsidian vault migrate` ([[Migration]]). Reads work.

## One backend, three front ends

An action lives once, as a function in the core. The MCP tool, the CLI command, and the Obsidian plugin reach the same function. Every tool action has a CLI command of the same name:

```text
atlas-obsidian vault [status|init|sync [--views]|mention|migrate [--dry-run]]
atlas-obsidian search TEXT [--type T]... [--kind K]... [--tag T]... [--status S]... [--limit N]
atlas-obsidian context [REPOSITORY] [--tag T]... [--path P] [--json]
atlas-obsidian match --items FILE.json | --docs ID... [--tags T]... [--across]
atlas-obsidian source capture [--inbox NAME]... | [--text FILE --title T] | [--repository R] [--tag T]... [--resolves STUB]
atlas-obsidian source chunks DOC
atlas-obsidian source read DOC CHUNK
atlas-obsidian change propose FILE.json | show ID | apply ID | reject ID --reason R | undo ID
atlas-obsidian work [list] | show DOC | stub TEXT... | spec FILE.json | promote STUB FILE.json | start SPEC [--take]
                    | done SPEC FILE.md | drop DOC --reason R | reopen DOC | block SPEC --reason R | unblock SPEC
                    | resolve STUB --became DOC... | note DOC --text T | set DOC ...
atlas-obsidian lint [--tag T]... [--json]
atlas-obsidian hook EVENT                        (the hooks; reads the event JSON on stdin)
```

The CLI adds commands that no tool needs: `setup`, `doctor`, `version`, `vault migrate`, and `open` (open a vault or a document in Obsidian; `--register` adds a vault that Obsidian does not know). The guard refuses `change … apply`, `vault migrate`, and `hook` from an agent's shell ([[Hooks#guard]]).

## What is not a tool

| Wanted | Where it lives | Why |
|---|---|---|
| extract items from text | [[wiki-extract]], an agent | judgment |
| decide and write pages | [[wiki-draft]], an agent | judgment |
| which policies bind this work | [[context]] gives the candidates; the conventions step of [[spec-write]] decides | candidates are a fact; relevance is judgment |
| reading a document | the host's Read, Grep, and Glob | they already work |
| the board, the tree, the timeline | the views ([[Views]]) | navigation is for people; agents use `search` and `work` list |
