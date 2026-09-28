
# Context Chain

> The output of [[context]]: everything an agent needs to work in one scope. It is how an agent that starts in the vault reaches a repository's context.

```yaml
scope: {Doc Ref}                 # the area or repository asked for; the vault when empty
chain: [{Doc Ref}, …]            # from the vault down to the scope; each with its body
children: [{Doc Ref}, …]         # areas and repositories directly below the scope
repositories: [{Doc Ref}, …]     # every repository at or below the scope
policies:                        # every policy on the chain, nearest scope first
  - ref: {Doc Ref}
    strength: must
    from: rep-h6t2vc             # the scope that holds it
threads: [{Doc Ref}, …]          # open threads whose scope is on the chain or below it
instructions:                    # for a repository: its own agent files
  - path: "~/code/p3-cloud/CLAUDE.md"
    content: "…"                 # bounded; the path is always given
repository:                      # for a repository: facts from git, now
  path: "~/code/p3-cloud"
  remote: "git@github.com:acme/p3-cloud.git"
  branch: main
  head: 4ac19e2
  dirty: 3                       # files changed and not committed
  described: 9e41c07
  behind: 12                     # commits from described to head
```

The chain is a fact: two correct runs return the same chain. Which policies apply to a task is a judgment, made by the conventions step of [[thread-spec]] and [[thread-tasks]].
