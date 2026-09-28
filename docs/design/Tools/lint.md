
# lint

> The health check. Takes an optional scope, returns [[Findings]]. Reads, and never writes.

The checks and their severities are listed in [[Findings]]. Lint runs over every typed document. It also reads every other markdown file as a link target, so a wiki page may link your scratchpad without a dead-link finding.

- With a scope, lint checks that scope's documents and the scopes below it.
- A quick run (the `problems` count in [[Vault Status]]) runs only the error checks that read frontmatter.
- A new vault lints clean. A test holds the template to that.

[[wiki-review]] reads the Findings, and [[wiki-edit]] fixes the wiki ones through a change.
