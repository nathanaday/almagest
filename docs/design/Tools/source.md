# source

> Bring an outside document into the vault, and read any document in chunks.

| Action | Takes | Returns | Writes |
|---|---|---|---|
| `capture` | [[Capture Request]] | the captured sources, each with its [[Chunk]]s | one `capture` commit |
| `chunks` | a document id | [[Chunk]]s | nothing |
| `read` | a document id and a chunk index | a [[Text Blob]] | nothing |

## capture

For each file or text:

1. Hash it (sha256). When a source already holds that hash, return it with `duplicate` set.
2. Copy it to `wiki/assets/<source id>.<ext>`. Never edit that file again.
3. Write the source document ([[Source Document]]): the fields code owns, `authority: unknown`, the tags given, the title (the file name without its extension, or the `title` given), a description of `Captured, not yet ingested`, and a body with only the embed.
4. Remove the inbox file.
5. When `resolves` names a stub: set the source's `from`, add the source to the stub's `became`, and write a `resolved` event.
6. Commit all of it as `capture: <titles>`, then sync and the views.

For `repository: <id>`, capture writes a snapshot of the repository at its head as a markdown file: the tree to three levels, the instruction files, the manifests (`go.mod`, `package.json`, …), the docs, and every TODO and FIXME line with its location. The locator is `<repository>@<commit>`, the title is `<repository> @ <short commit>` (`p3-cloud @ 4ac19e2`), and the tags are the tag the repository defines.

A captured source is [[Changes#Pending documents|pending]] until a change absorbs it.

## chunks and read

`chunks` splits any document by the rules in [[Chunk]]. `read` returns one chunk as a [[Text Blob]]. For a PDF or an image, the Text Blob names the file and the pages instead of holding text, and the worker reads them with Read.

`read` takes any document, not only a source. That is how a spec, a completion event, or a session's summary enters the wiki pipeline.

Refusals: an inbox name that is not in `inbox/`; a file over 200 MB; a repository id that is not a repository document; `resolves` on a document that is not an open stub; a tag that breaks [[Documents#Form]], or a new tag in `tags: known` mode without `new_tags: true`.
