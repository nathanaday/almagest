# The conventions step

Find the policies of the wiki that apply to a piece of work in a repository, so the work
follows them. The almagest skill runs this step before it changes code in a linked
repository.

1. Call `context` with `repository` for each repository the work touches, or with `tags`
   when it touches none. `policies` lists every policy whose tags the repository holds,
   the most specific first (the most tags, then the deepest), then the policies with no
   tags. Each carries its `strength` and `via`, the policy's tags that make it apply.
2. Read each candidate policy's `## Rule` and `## Applies when`.
3. Decide for each: does it bind this work? A `must` policy that applies binds the
   work; a `should` policy binds it unless you tell the user why not; a `may` policy is
   a choice.
4. Follow the ones that bind the work.

Which policies bind is your decision; the list of candidates is a fact, so never skip
the `context` call.
