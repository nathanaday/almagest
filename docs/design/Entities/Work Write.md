# Work Write

> The input of [[work]]. One form per action. Each form writes documents and events, and returns a [[Work View]].

```yaml
stub:    {text, title?, tags?, priority?, inbox?, new_tags?}
spec:    {specs: [{title, kind, text, description, tags?, parent?, repositories?, depends?, order?,
                   priority?, implements?, supersedes?, from?}], resolve?, new_tags?}
         # parent and depends may name a spec of the same call by its title
         # resolve: true closes the `from` stub with every spec of the call in `became`
promote: {stub, kind, text, description?, title?, tags?, repositories?, parent?, depends?, priority?}
start:   {spec, take?}                 # take: start a plan another live session holds
done:    {spec, result: {delivered, verified, follow_ups, learned}}
drop:    {doc, reason}
reopen:  {doc, reason?}
block:   {spec, reason}                # one line
unblock: {spec}
resolve: {stub, became: [ids], text?}
note:    {doc, text}
set:     {doc, title?, description?, tags?, aliases?, priority?, parent?, repositories?, depends?,
          order?, implements?, supersedes?, new_tags?}
```

`inbox` names a note in `inbox/` that the stub replaces: the tool removes it in the same commit.

`text`, `result`, and `reason` are the model's prose. The tool writes them into the document's sections, or into the event's, and writes no prose of its own.
