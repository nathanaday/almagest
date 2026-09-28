
# Thread View

> A thread with all its documents. The output of [[thread]] show and of every thread write. [[thread]] list returns the Board.

```yaml
stub: {Doc Ref}                        # with state: stage, outcome, active, tasks, priority, blocked
spec: {Doc Ref}                        # or empty
tasks:
  - ref: {Doc Ref}                     # with state: status, active, order, repository
    depends: [tsk-…]
    ready: true                        # open, and every dependency done
receipt: {Doc Ref}                     # or empty
sessions: [{Doc Ref}]                  # every session that worked on it, newest first
changes: [{Doc Ref}]                   # every change that served it
next: "tasks"                          # the next step: spec | tasks | task T2 | receipt | none
```

The Board:

```yaml
board:
  - stage: tasks
    threads: [{Doc Ref}, …]            # priority first, then the last update
  - stage: spec
    threads: [...]
  - stage: stub
    threads: [...]
closed: [{Doc Ref}, …]                 # the last ten
```

`next` is a fact: the furthest missing document, or the first ready task.
