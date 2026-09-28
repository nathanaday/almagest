
# Document Types

Every document in the vault has a type and an id. The type sets its folder, its schema, and who may write it. Each type's schema is on the page for its folder.

## The types

| Type | Id prefix | Folder | Schema | Written by |
|---|---|---|---|---|
| `vault` | `vlt` | `Atlas.md` at the root | [[Vault#Atlas.md]] | `atlas init`; then the user |
| `area` | `are` | `wiki/areas/` | [[Wiki#Area]] | `change` |
| `repository` | `rep` | `wiki/repositories/` | [[Wiki#Repository]] | `change` |
| `concept` | `con` | `wiki/concepts/` | [[Wiki#Concept]] | `change` |
| `entity` | `ent` | `wiki/entities/` | [[Wiki#Entity]] | `change` |
| `policy` | `pol` | `wiki/policies/` | [[Wiki#Policy]] | `change` |
| `source` | `src` | `wiki/sources/` | [[Wiki#Source]] | `source` capture, then `change` |
| `stub` | `thr` | `threads/<thread>/` | [[Threads#Stub]] | `thread`; the model and the user edit the prose |
| `spec` | `spc` | `threads/<thread>/` | [[Threads#Spec]] | `thread`; prose as above |
| `task` | `tsk` | `threads/<thread>/` | [[Threads#Task]] | `thread`; prose as above |
| `receipt` | `rcp` | `threads/<thread>/` | [[Threads#Receipt]] | `thread`; prose as above |
| `session` | `ses` | `sessions/<yyyy-mm>/` | [[Sessions#The session document]] | hooks; `session` for the prose |
| `change` | `chg` | `changes/<yyyy-mm>/` | [[Changes#The change document]] | `change` |

The stub's id is the thread's id. A thread is its stub and the documents that name it.

## Families

- **Scope pages**: `vault`, `area`, `repository`. They form the [[Atlas V2#The context graph|context graph]]. Every other document points at one or more of them through `scope`.
- **Knowledge pages**: `concept`, `entity`, `policy`, `source`. They are the wiki's content. Each one cites its sources.
- **Thread documents**: `stub`, `spec`, `task`, `receipt`. They are one line of work, from the first sentence to the result.
- **Record documents**: `session`, `change`. Code writes them as the work happens. They say what ran and what changed.

## Fields every document has

```yaml
id: con-k3m9qa        # minted by code; never changes
type: concept         # one of the types above
created: 2026-09-27   # set by code
updated: 2026-09-27   # set by code when a tool or hook writes, or the model edits
```

- The file name is the title. There is no `title` field. The one exception is `Atlas.md`, whose title is its `name`.
- An id is the prefix, a hyphen, and six characters of lowercase base32 (`k3m9qa`). Code mints it and checks that no document holds it.
- Other fields are the type's. A field a schema does not name is the user's. Code keeps it and never reads it.

## What a type's schema says

Each schema on the folder pages gives:

1. the fields, with which ones code owns;
2. the sections of the body, in order;
3. the rules that `change`, `thread`, or `lint` enforce.

A document that breaks its schema is a finding of [[lint]], never an error that stops a tool. A tool refuses only to write a document that breaks its schema.
