# match

> Join the subjects of one or more [[Item Map]]s, and match each one against the topics. Returns a [[Match Map]].

Two inputs:

| Input | Used by | Matches |
|---|---|---|
| `items: [Item Map, …]` | [[wiki-sync]], after extraction | new subjects against every topic |
| `docs: [id, …]`, or `tags: [t]` with `across: true` | [[wiki-map]] | topics against each other, across the child tags of a tag |

Optional `tags` limits the candidate topics to those that hold every tag given.

## The steps, all code

1. **Normalize** each name and alias: lower case, trimmed, runs of space and punctuation made one space, a trailing plural `s` dropped when the singular also occurs.
2. **Join** items across chunks: two items are one subject when a normalized name or alias is equal and the kind is equal. The subject keeps every item, so every claim and locator survives.
3. **Hit**: a topic whose normalized title or alias equals a normalized name or alias of the subject. The kind may differ (an item of kind `concept` hits an `entity` topic); the Match Map keeps both kinds, and the drafter decides.
4. **Near**: no hit, and a topic scores above the threshold on BM25 over name, aliases, and description. The best five are the `neighbors`.
5. **New**: nothing above the threshold. The best three below it are still given as `neighbors`, so the drafter can link related topics.

With `tags: [t]` and `across: true`, each topic that holds `t` is compared only with topics that hold a different child tag of `t`. Hits and near pairs then mean "two parts of `t` hold one subject": the candidates for a bridge, or for widening a topic's tag to `t`.

The threshold is a constant in code, tuned on real vaults. The Match Map carries every score, so the drafter can see how close a call was.

Refusals: an Item Map with an item of no kind or no name.
