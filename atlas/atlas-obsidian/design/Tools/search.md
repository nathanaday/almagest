
# search

> Ranked search over every typed document in the vault. Takes a [[Query]], returns [[Hits]].

One search serves every need: finding a wiki page, finding the repository a request means, and finding the thread for a piece of work.

- The candidates are every document with a known `type`. Files without one (the scratchpad, the inbox, your own notes) are not searched.
- `scope` includes every scope below it. A document with several scopes (a stub) matches when any of them is inside.
- `state` filters on the derived state in the [[Doc Ref]]: `{stage: [stub, spec, tasks]}` finds open threads; `{status: [proposed]}` finds changes that wait for you.
- Ranking: BM25 over title (weight 3), aliases (3), description (2), and body (1). An empty `text` with filters lists the matches, newest `updated` first.

The server keeps no index file. It reads the vault per call and may cache parsed files in memory by modification time.

Refusals: a `scope` that names no scope page; a `type` that is not a document type.
