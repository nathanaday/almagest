# Chunk

> One part of a document that one worker reads with care. [[source]] chunks returns the list.

```yaml
doc: doc-p2x7nd
index: 2
count: 4
locator: "pages 21-40"        # or "lines 1201-1980 (§ Methods, § Results)", or "whole"
size: 20                      # pages, or lines
```

| Document | Chunks |
|---|---|
| PDF | 20 pages each, the most one Read takes |
| image | one chunk, `whole` |
| markdown | whole sections, joined up to 800 lines; a longer section splits at its subheadings |
| text | 800 lines, cut at a blank line near the limit |
| a spec, an event, or anything under 800 lines | one chunk, `whole` |
