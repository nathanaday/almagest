# Context Brief

> The output of [[context]]: everything an agent needs to work in one repository, or under a set of tags.

```yaml
tags:                            # the tags asked for, or that the repository holds and defines
  - tag: work/p3/p3-edge
    count: 41                    # documents that hold it
pages: [{Doc Ref}, …]            # the pages of those tags and every tag above them, top first; each with its body
repositories: [{Doc Ref}, …]     # the repositories that hold every tag (for a repository: itself)
policies:                        # every policy that applies, the most specific first
  - ref: {Doc Ref}
    strength: must
    via: [work/p3, go]           # the policy's tags
work: [{Doc Ref}, …]             # open or started plans and open stubs that hold the tags or name the repository
instructions:                    # for a repository: its own agent files
  - path: "~/code/p3-edge/CLAUDE.md"
    content: "…"                 # bounded; the path is always given
repository:                      # for a repository: facts from git, now
  path: "~/code/p3-edge"
  remote: "git@github.com:acme/p3-edge.git"
  branch: main
  head: 4ac19e2
  dirty: ["internal/score.go", "…"]   # files changed and not committed
  ahead: 2                       # commits ahead of the remote's branch, from the last fetch
  behind_remote: 0
  recent: [{commit: 4ac19e2, time: 2026-09-28T17:02:11, subject: "Score boxes by motion"}]
  described: 9e41c07
  behind: 12                     # commits from described to head
```

The context is a fact: two correct runs return the same context. Which policies bind a piece of work is a judgment, made by the conventions step of [[spec-write]] and [[spec-split]].
