# Text Blob

> One chunk of a document, ready for [[wiki-extract]]. The output of [[source]] read.

```yaml
doc: doc-p2x7nd               # the document's id
type: source
kind: ""
title: DINOv2
tags: [ml/self-supervised, paper]
authority: primary            # a source only
chunk: {Chunk}
content: "…"                  # the text of the chunk, without the lead callout, the code's sections, and code-owned fields
file: ""                      # for a PDF or an image: the captured file, which the worker reads with Read
pages: ""                     # for a PDF: the page range to read
```

A PDF's text is not extracted by code. The Text Blob names the file and the pages, and the worker reads them with the host's Read tool, which reads PDFs and images well. Every other document comes as `content`.

Any document can be a Text Blob: a source, a spec, a completion event, a session's summary. That is how one pipeline serves ingest, learning from work, and saving a conversation.
