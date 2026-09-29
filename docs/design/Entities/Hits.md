# Hits

> The output of [[search]]: Doc Refs, best first, and the facets of every match.

```yaml
hits:
  - ref: {Doc Ref}
    score: 7.42
    snippet: "…the line of the body that matched best…"
total: 31                    # matches before the limit
facets:                      # counts over all the matches, before the limit
  tags: {self-driving: 12, project: 5, paper: 9}   # the tags the matches hold, beyond those asked for
  types: {topic: 18, source: 9, spec: 4}
  status: {stable: 15, open: 3, started: 1}
```

Ranking is BM25 over five fields with weights: title 3, aliases 3, tags 2, description 2, body 1. Code computes it on each call over the files on disk, so no index file exists to go stale.

The agent reads `facets.tags` to narrow a broad query: a tag that splits the hits in two is the next filter to try, or the question to ask the user.
