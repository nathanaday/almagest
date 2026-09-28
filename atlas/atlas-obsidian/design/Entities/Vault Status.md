
# Vault Status

> The state of the vault in one read. The output of [[vault]], and the source of the session-start context.

```yaml
vault: {id, name, path}
scopes: {areas: 4, repositories: 9}
wiki: {pages: {concept: 40, entity: 55, policy: 12, source: 18}, draft: 3, contested: 1}
threads:
  open: {stub: 3, spec: 1, tasks: 2}
  active: [{Doc Ref}]                  # stubs with active: true
sessions:
  running: [{Doc Ref}]
  waiting: [{Doc Ref}]                 # these wait for you
inbox: [{name: "DINOv2.pdf", size: "2.1 MB", kind: pdf}]
pending: [{Doc Ref}]                   # documents the wiki has not absorbed
changes:
  proposed: [{Doc Ref}]
  recent: [{Doc Ref}]                  # the last five applied, with their summaries
mentions: [{doc: {Doc Ref}, line: 12, text: "@atlas add these to the wiki"}]
problems: 2                            # the count of errors a quick lint finds
```
