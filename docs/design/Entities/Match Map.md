# Match Map

> Each subject of one or more Item Maps, joined across chunks and matched against the topics. The output of [[match]].

```yaml
subjects:
  - key: "self-supervised learning"       # the normalized name
    kind: concept
    items: [{Item}, …]                    # every item naming this subject, from every chunk
    match: hit                            # hit | near | new
    page: {Doc Ref}                       # hit: the topic whose title or alias equals the name
    neighbors:                            # near and new: the closest topics, best first
      - ref: {Doc Ref}
        score: 0.71
```

| `match` | Means |
|---|---|
| `hit` | a topic's title or alias equals the subject's name or one of its aliases |
| `near` | no equal name, but a topic scores above the threshold on name and description |
| `new` | nothing scores above the threshold |

Code decides all three. Whether a `near` neighbor is the same subject is a judgment, made by [[wiki-draft]].
