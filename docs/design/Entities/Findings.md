# Findings

> The output of [[lint]]: what is wrong in the vault, and which tool or skill fixes it.

```yaml
findings:
  - check: dead-link
    severity: error                    # error | warning | info
    doc: {Doc Ref}
    message: "links [[ViT]], which resolves to nothing"
    fix: "wiki-edit: create the topic, or change the link"
```

The checks:

| Check | Severity | Finds |
|---|---|---|
| `schema` | error | a document that breaks its type's schema |
| `duplicate-title` | error | two documents with one title or alias, or a title that begins with `Tag · ` or `View · ` |
| `dead-link` | error | a link that resolves to nothing |
| `tag` | error | a tag that breaks the form; two documents that define one tag; a tag page that does not hold its tag's parent |
| `repository-path` | error | a repository whose path is gone or is no git work tree |
| `spec` | error | a `parent` or `depends` loop; a `depends` that is not a sibling; a done plan with an open part |
| `event` | error | an event whose subject is gone; an event kind that does not fit its subject's type |
| `misplaced` | error | a typed document outside `wiki/documents/` |
| `untyped` | warning | a markdown file with no type in `wiki/documents/` |
| `orphan` | warning | a topic that no other document links to and whose tags no other document holds |
| `uncited` | warning | a topic with empty `sources` |
| `stale` | warning | a topic whose `refreshed` is older than a change to a document it cites, or to the repository whose tag it holds |
| `tag-near` | info | two tags whose documents are nearly the same set, or whose names differ only by a plural or a separator |
| `leaf-repositories` | info | a leaf plan that names more than one repository |
| `repository-behind` | info | a repository more than 50 commits past `described` |
| `pending` | info | a pending document older than a week |
| `change-stale` | info | a change proposed more than a day ago |
| `session-lost` | info | a lost session that had started a plan still `started` |

Lint reads and never writes. A fix is a change, a work write, or your edit.
