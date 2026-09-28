
# match

> Join the subjects of one or more [[Item Map]]s, and match each one against the wiki. Returns a [[Match Map]]. See [[match.canvas|the match flow]].

Two inputs:

| Input | Used by | Matches |
|---|---|---|
| `items: [Item Map, …]` | [[wiki-sync]], after extraction | new subjects against every knowledge page |
| `pages: [id, …]`, `siblings: true` | [[wiki-rollup]] | existing pages against the pages of their sibling scopes |

Optional `within: <scope>` limits the candidate pages to that scope and the scopes below it.

## The steps, all code

1. **Normalize** each name and alias: lower case, trimmed, runs of space and punctuation made one space, a trailing plural `s` dropped when the singular also occurs.
2. **Join** items across chunks: two items are one subject when a normalized name or alias is equal and the type is equal. The subject keeps every item, so every claim and locator survives.
3. **Hit**: a knowledge page whose normalized title or alias equals a normalized name or alias of the subject. Type may differ (an item typed `concept` hits an `entity` page); the Match Map keeps both types, and the drafter decides.
4. **Near**: no hit, and a page scores above the threshold on BM25 over name, aliases, and description. The best five are the `neighbors`.
5. **New**: nothing above the threshold. The best three below it are still given as `neighbors`, so the drafter can link related pages.

With `siblings: true`, a page is compared only with pages whose scope is a sibling of its own (another child of the same parent). Hits and near pairs then mean "two children hold one subject": the candidates for a bridge or an upgrade.

The threshold is a constant in code, tuned on real vaults. The Match Map carries every score, so the drafter can see how close a call was.

Refusals: an Item Map with an item of no type or no name.
