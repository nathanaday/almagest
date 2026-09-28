
# Match Map

> Each subject of one or more Item Maps, joined across chunks and matched against the wiki. The output of [[match]].

```yaml
subjects:
  - key: "self-supervised learning"       # the normalized name
    type: concept
    items: [{Item}, …]                    # every item naming this subject, from every chunk
    match: hit                            # hit | near | new
    page: {Doc Ref}                       # hit: the page whose title or alias equals the name
    neighbors:                            # near and new: the closest pages, best first
      - ref: {Doc Ref}
        score: 0.71
```

| `match` | Means |
|---|---|
| `hit` | a page's title or alias equals the subject's name or one of its aliases |
| `near` | no equal name, but a page scores above the threshold on name and description |
| `new` | nothing scores above the threshold |

Code decides all three. Whether a `near` neighbor is the same subject is a judgment, made by [[wiki-draft]].
