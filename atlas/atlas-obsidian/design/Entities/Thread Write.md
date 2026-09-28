
# Thread Write

> The input of [[thread]]. One form per action. Each form writes documents and returns a [[Thread View]].

```yaml
open:   {text, title?, scope: [ids], priority?, inbox?, mention?}   # a stub, in the user's words
attach: {thread}                                            # this session works on the thread
file:   {thread, part: spec | receipt, text, outcome?}      # outcome: completed | killed, for a receipt
tasks:  {thread, tasks: [{title, text, repository, depends?: [titles], order?}]}
task:   {task, do: start | done | drop | reopen, result?}   # result: required for done
set:    {thread, title? , priority?, blocked?, scope?}
reopen: {thread}
```

`inbox` names a note in `inbox/` that the stub replaces: the tool removes it in the same commit. `mention` names a mention line to close ([[Obsidian Plugin#Mentions]]).

`text` and `result` are the model's prose. The tool writes them into the document under the lead callout and writes no prose of its own.
