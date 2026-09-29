# Item Map

> What one chunk says, as subjects and claims. [[wiki-extract]] returns it; [[match]] takes one or more.

```yaml
doc: doc-p2x7nd
chunk: 2
summary: "Two to four sentences on what this chunk covers."
items:
  - kind: concept                         # concept | entity | policy
    name: "Self-supervised learning"
    aliases: [SSL]
    description: "One sentence, as the document states it."
    tags: [ml/self-supervised]            # suggested: tags that exist and fit; the drafter decides
    strength: ""                          # policy only: must | should | may
    claims:
      - text: "DINOv2 trains ViT models with no labels on 142M curated images."
        locator: "p. 3"
    source: doc-p2x7nd
questions: []                             # what the chunk leaves unclear
partial: false                            # true when the worker could not read the whole chunk
reason: ""                                # why, when partial
```

- An item is a subject the chunk says something about. A claim is one statement, with its locator.
- `summary` feeds the `## Structure` section of a source.
- Claims carry locators because the drafter must cite. Aliases are there because matching needs them.
- The worker suggests tags from the vocabulary it was given ([[wiki-extract]]); it never invents one. The drafter picks the tags of each write.
