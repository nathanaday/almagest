
# Doc Ref

> The one way a tool names a document. Every tool that returns documents returns Doc Refs.

```yaml
id: con-k3m9qa
type: concept
title: Self-supervised learning
path: wiki/concepts/Self-supervised learning.md    # relative to the vault
scope: are-w4q8ze                                  # the scope's id; "" for the vault
description: "Training a model on data with no labels, from a signal in the data itself."
state: {}                                          # the type's derived state, below
```

`state` by type:

| Type | `state` |
|---|---|
| `stub` | `stage`, `outcome`, `active`, `tasks`, `priority`, `blocked` |
| `task` | `status`, `active`, `order`, `repository` |
| `session` | `status`, `thread`, `task` |
| `change` | `status`, `counts` |
| `source` | `pending` |
| `repository` | `path`, `behind` (commits since `described`) |
| other types | `status`, where the type has one |

A tool takes a document as an id, or as a title that resolves to one document. A title that resolves to none or to two is refused, and the refusal lists the candidates.
