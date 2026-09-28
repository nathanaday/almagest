
# Item Map

> What one chunk says, as subjects and claims. [[wiki-extract]] returns it; [[match]] takes one or more.

```yaml
doc: src-p2x7nd
chunk: 2
summary: "Two to four sentences on what this chunk covers."
items:
  - type: concept                         # concept | entity | policy
    name: "Self-supervised learning"
    aliases: [SSL]
    description: "One sentence, as the document states it."
    kind: ""                              # entity only: person | organization | tool | component | …
    strength: ""                          # policy only: must | should | may
    claims:
      - text: "DINOv2 trains ViT models with no labels on 142M curated images."
        locator: "p. 3"
    source: src-p2x7nd
questions: []                             # what the chunk leaves unclear
```

- An item is a subject the chunk says something about. A claim is one statement, with its locator.
- `summary` feeds the `## Structure` section of a source page.
- The first draft had `type`, `source_id`, and `description`. Claims and locators are added, because the drafter must cite, and aliases, because matching needs them.
