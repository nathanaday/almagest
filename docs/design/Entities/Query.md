# Query

> The input of [[search]].

```yaml
text: "remote update"                 # free text; may be empty when the filters say enough
types: [topic, source]                # empty: the six document types
kinds: [policy]                       # topic, spec, or event kinds; empty: every kind
tags: [work/p3, go]                   # a document must hold every one; empty: no filter
status: [open, started]               # empty: every status
repository: doc-h6t2vc                # optional: documents that name it or hold its tag
limit: 20
```

Every tool that lists documents takes the same filters with the same meaning ([[Tools#Conventions]]).

Examples:

| Need | Query |
|---|---|
| Is there a plan for this work? | `text: "vehicle false alarms", types: [spec], kinds: [plan], status: [open, started]` |
| Is there a stub for this idea? | `text: "smaller backbone", types: [stub], status: [open]` |
| Which repository does "p3 cloud front end" mean? | `text: "p3 cloud front end", types: [repository]` |
| What do we know about my CS513 self-driving project? | `tags: [school/cs513, self-driving, project]` |
| Which policies bind Go work in p3? | `types: [topic], kinds: [policy], tags: [work/p3, go]` (or [[context]] with the repository) |
| What happened on p3-edge this week? | `types: [event], repository: p3-edge` |
