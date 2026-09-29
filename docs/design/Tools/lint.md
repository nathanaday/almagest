# lint

> The health check. Takes optional tags, returns [[Findings]]. Reads, and never writes.

The checks and their severities are listed in [[Findings]]. Lint runs over every typed document, and over the session and change documents. It also reads every other markdown file as a link target, so a document may link your scratchpad without a dead-link finding.

- With tags, lint checks the documents that hold every tag, and the checks that span the vault (duplicate titles, tag pages) for them only.
- A quick run (the `problems` count in [[Vault Status]]) runs only the error checks that read frontmatter.
- A new vault lints clean. A test holds the template to that.

[[wiki-review]] reads the Findings, [[wiki-edit]] fixes the knowledge ones through a change, and `work` fixes the work ones.
