# Vault Status

> The state of the vault in one read. The output of [[vault]], and the source of the session-start context.

```yaml
vault: {id, name, path, layout: 3, tags_mode: known}
documents: {source: 18, repository: 9, topic: 107, stub: 6, spec: 14, event: 58}
topics: {concept: 40, entity: 55, policy: 12, overview: 7, draft: 3, contested: 1}
tags:                                  # every tag with its count, the most used first
  - {tag: work/p3, count: 64, page: {Doc Ref}}
  - {tag: school/cs513, count: 34, page: {Doc Ref}}
work:
  stubs: 6
  plans: {open: 5, started: 2, blocked: 1}
  active: [{Doc Ref}]                  # plans a live session has started
sessions:
  running: [{Doc Ref}]
  waiting: [{Doc Ref}]                 # these wait for you
inbox: [{name: "DINOv2.pdf", size: "2.1 MB", kind: pdf}]
pending: [{Doc Ref}]                   # documents the wiki has not absorbed
changes:
  proposed: [{Doc Ref}]
  recent: [{Doc Ref}]                  # the last five applied, with their summaries
recent: [{Doc Ref}]                    # the last ten events
mentions: [{doc: {Doc Ref}, line: 12, text: "@atlas add these to the wiki"}]
problems: 2                            # the count of errors a quick lint finds
```
