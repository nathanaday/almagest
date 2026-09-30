# The conventions step

Find the policies of the wiki that apply to a piece of work, so the work follows them.
The spec-write skill runs this step once for a plan; the spec-split skill runs it for
each part.

1. Call `context` with `repository` for each repository the plan names in
   `repositories`, or with `tags` when the plan names none. `policies` lists every
   policy whose tags the repository holds, the most specific first (the most tags, then
   the deepest), then the policies with no tags. Each carries its `strength` and `via`,
   the policy's tags that make it apply.
2. Read each candidate policy's `## Rule` and `## Applies when`.
3. Decide for each: does it bind this work? A `must` policy that applies binds the
   work; a `should` policy binds it unless the spec records why not; a `may` policy is
   a choice.
4. Keep the ones that bind the work, each with one line on why:
   `- [[Sign commits]] (must): every commit of this plan lands on main.`
5. Write them under `## Conventions` in the spec. The spec links each policy by title.

The spec-run skill follows them. The spec-review agent checks the work against them.
Which policies bind is judgment; the list of candidates is a fact, so never skip the
`context` call.
