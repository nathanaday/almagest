# Work View

> A stub or a spec with everything around it. The output of [[work]] show and of every work write. [[work]] list returns the Board.

```yaml
doc: {Doc Ref}                         # the stub or spec asked for
root: {Doc Ref}                        # the top plan of its tree; the doc itself at the top
ancestors: [{Doc Ref}]                 # from the root down to the parent
parts:                                 # the child plans, in order
  - ref: {Doc Ref}
    depends: [doc-…]
    ready: true                        # open, and every dependency done
events: [{Doc Ref}]                    # the doc's events, newest first
result: {Doc Ref}                      # the completed event, when done
from: {Doc Ref}                        # the stub it came from
became: [{Doc Ref}]                    # for a resolved stub
implements: [{Doc Ref}]
sessions: [{Doc Ref}]                  # every session that worked on it, newest first
changes: [{Doc Ref}]                   # every change that served it
next: "done"                           # write | start <plan> | done | none
```

`next` is a fact:

| The doc | `next` |
|---|---|
| an open stub | `write`: make it a spec, a topic, or something else |
| a plan with an empty `## Done when` | `write` |
| an open plan with no parts | `start <the plan>`; the skill may split it first ([[spec-split]]) |
| a plan with a ready part | `start <the first ready part>` |
| a started leaf plan, or a plan whose parts are all done or dropped | `done` |
| a done or dropped plan, a resolved or dropped stub, a design | `none` |

The Board:

```yaml
board:
  active: [{Doc Ref}]                  # plans a live session has started
  started: [{Doc Ref}]                 # by priority, then the last event
  ready: [{Doc Ref}]                   # open plans with every dependency done
  blocked: [{Doc Ref}]
  stubs: [{Doc Ref}]                   # open, by priority
done: [{Doc Ref}]                      # the last ten completed or dropped plans
```
