
# Text Blob

> One chunk of a document, ready for [[wiki-extract]]. The output of [[source]] read.

```yaml
doc: src-p2x7nd               # the document's id (the first draft called it source_id)
type: source
title: DINOv2
scope: rep-h6t2vc
authority: primary
chunk: {Chunk}
content: "…"                  # the text of the chunk, with the lead callout and code-owned fields removed
file: ""                      # for a PDF or an image: the captured file, which the worker reads with Read
pages: ""                     # for a PDF: the page range to read
```

A PDF's text is not extracted by code. The Text Blob names the file and the pages, and the worker reads them with the host's Read tool, which reads PDFs well. Every other document comes as `content`.

Any document can be a Text Blob: a source, a spec, a receipt, a session's summary. That is how one pipeline serves ingest, thread learning, and saving a conversation.
