
# Query

> The input of [[search]].

```yaml
text: "remote update"                 # free text; may be empty when the filters say enough
types: [concept, entity, policy]      # empty: every type
scope: are-w4q8ze                     # this scope and every scope below it; empty: the whole vault
state: {stage: [stub, spec, tasks]}   # optional; filters on Doc Ref state
limit: 20
```

Examples:

| Need | Query |
|---|---|
| Is there a thread for this work? | `text: "vehicle false alarms", types: [stub], state: {stage: [stub, spec, tasks]}` |
| Which repository does "p3 cloud front end" mean? | `text: "p3 cloud front end", types: [repository, area]` |
| What does the wiki know about OTA updates in p3? | `text: "over the air update", scope: p3` |
