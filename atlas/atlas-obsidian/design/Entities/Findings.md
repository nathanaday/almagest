
# Findings

> The output of [[lint]]: what is wrong in the vault, and which tool or skill fixes it.

```yaml
findings:
  - check: dead-link
    severity: error                    # error | warning | info
    doc: {Doc Ref}
    message: "links [[ViT]], which resolves to nothing"
    fix: "wiki-edit: create the page, or change the link"
```

The checks:

| Check | Severity | Finds |
|---|---|---|
| `schema` | error | a document that breaks its type's schema |
| `duplicate-title` | error | two documents with one title or alias |
| `dead-link` | error | a link that resolves to nothing |
| `scope` | error | a `scope` or `parent` that names no scope page, or a loop of parents |
| `repository-path` | error | a repository page whose path is gone or is no git work tree |
| `thread` | error | a second spec or receipt, a task of no thread, a receipt with no valid outcome |
| `orphan` | warning | a knowledge page that no other document links to |
| `uncited` | warning | a knowledge page with empty `sources` |
| `repository-behind` | info | a repository page more than 50 commits behind its head |
| `pending` | info | a pending document older than a week |
| `change-stale` | info | a change proposed more than a day ago |
| `session-lost` | info | a lost session that lists an open task |

Lint reads and never writes. A fix is a change, a thread write, or your edit.
