# Doc Ref

> The one way a tool names a document. Every tool that returns documents returns Doc Refs.

```yaml
id: doc-k3m9qa
type: topic
kind: concept                                      # topic, spec, and event; empty for the others
title: Self-supervised learning
path: wiki/documents/Self-supervised learning.md   # relative to the vault
tags: [ml/self-supervised, vision]
description: "Training a model on data with no labels, from a signal in the data itself."
status: stable                                     # the type's status; empty for a repository or an event
state: {}                                          # the type's derived state, below
```

`state` by type:

| Type | `state` |
|---|---|
| `source` | `pending`, `media`, `authority` |
| `repository` | `path`, `defines`, `behind` (commits since `described`) |
| `topic` | `defines` (an overview), `strength` (a policy), `sources` (count) |
| `stub` | `priority`, `became` |
| `spec` | `parent`, `root`, `repositories`, `priority`, `blocked`, `active`, `parts`, `ready` (open, and every dependency done) |
| `event` | `subject`, `at`, `session` |
| `session` | `status`, `specs` |
| `change` | `status`, `counts` |

A tool takes a document as an id, or as a title that resolves to one document. A title that resolves to none or to two is refused, and the refusal lists the candidates.
