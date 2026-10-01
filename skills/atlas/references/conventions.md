# The conventions step

Find the policies of the wiki that apply to a piece of work, so the work follows them.
The thread-spec skill runs this step once for a thread.

1. Call `context` with `repository` for each repository the work touches, or with `tags`
   when it touches none. `policies` lists every policy whose tags the repository holds,
   the most specific first (the most tags, then the deepest), then the policies with no
   tags. Each carries its `strength` and `via`, the policy's tags that make it apply.
2. Read each candidate policy's `## Rule` and `## Applies when`.
3. Decide for each: does it bind this work? A `must` policy that applies binds the
   work; a `should` policy binds it unless the spec records why not; a `may` policy is
   a choice.
4. Keep the ones that bind the work, each with one line on why:
   `- [[Sign commits]] (must): every commit of this thread lands on main.`
5. Write them under `## Rules` in the spec. The spec links each policy by title.

The thread-run skill follows them. The thread-audit agent checks the work against them.
Which policies bind is your decision; the list of candidates is a fact, so never skip
the `context` call.
