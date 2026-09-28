
# Hits

> The output of [[search]]: Doc Refs, best first.

```yaml
hits:
  - ref: {Doc Ref}
    score: 7.42
    snippet: "…the line of the body that matched best…"
total: 31            # matches before the limit
```

Ranking is BM25 over four fields with weights: title 3, aliases 3, description 2, body 1. Code computes it on each call over the files on disk, so no index file exists to go stale.
