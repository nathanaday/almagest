
# Entities

Two kinds of entity move through Atlas:

- **Documents**: the thirteen types in [[Document Types]]. They live in the vault.
- **Data**: what a tool takes and what it returns. Data lives only in a call. When data must last, a tool writes it into a document.

Every tool names one data entity on each side. [[Entities.canvas|The entity flow]] shows how they connect.

## Data entities

| Entity | Is | Produced by | Consumed by |
|---|---|---|---|
| [[Doc Ref]] | the reference to one document | every tool that returns documents | every tool that takes documents |
| [[Query]] | what to search for | the model | [[search]] |
| [[Hits]] | ranked Doc Refs | [[search]] | the model |
| [[Context Chain]] | a scope, its ancestors, its policies, its threads, its repository facts | [[context]] | the model; the conventions step |
| [[Capture Request]] | inbox files, pasted text, or a repository to capture | the model | [[source]] capture |
| [[Chunk]] | one part of a document | [[source]] capture and chunks | [[source]] read |
| [[Text Blob]] | the text of one chunk | [[source]] read | [[wiki-extract]] |
| [[Item Map]] | the subjects and claims of one chunk | [[wiki-extract]] | [[match]] |
| [[Match Map]] | subjects joined across chunks, matched to pages | [[match]] | [[wiki-draft]] |
| [[Wiki Change Plan]] | the writes of one change | [[wiki-draft]] and the skill | [[change]] propose |
| [[Change Preview]] | the preview and status of a change document | [[change]] | the model, which shows it to the user |
| [[Thread Write]] | one action on a thread | the model | [[thread]] |
| [[Thread View]] | a thread and all its documents; the Board | [[thread]] | the model |
| [[Session Note]] | the agent's prose for its session document | the model | [[session]], then the hook |
| [[Vault Status]] | the state of the vault in one read | [[vault]] | the session-start hook; the model |
| [[Findings]] | what is wrong, and the fix | [[lint]] | [[wiki-review]]; the model |

## Rules

1. A tool takes ids, or titles that resolve to one document. It returns Doc Refs, never bare paths.
2. Data that must outlive a call goes into a document: a Wiki Change Plan becomes a change document; a Session Note becomes part of a session document.
3. Workers return data, never documents. [[wiki-extract]] returns an Item Map; [[wiki-draft]] returns part of a Wiki Change Plan. Only the skill that sent them calls a tool that writes.
