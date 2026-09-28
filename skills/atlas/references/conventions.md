# The conventions step

Find the policies of the wiki that apply to a thread or a task, so the work follows
them. The thread-spec skill runs it once for the thread; the thread-plan skill runs it
for each task.

1. Call `context` for the scope: the thread's repositories and areas, or the task's
   repository. `policies` lists every policy on the chain from the vault down, the
   nearest scope first, with its strength.
2. Read each candidate policy's `## Rule` and `## Applies when`.
3. Decide for each: does it apply to this work? A `must` policy that applies binds the
   work; a `should` policy binds it unless the work records why not; a `may` policy is
   a choice.
4. Keep the ones that apply, each with one line on why:
   `- [[Sign commits]] (must): every commit of this task lands on main.`
5. Write them under `## Conventions` in the spec or in the task.

The thread-run skill follows them. The thread-review agent checks the work against them.
Relevance is judgment; the list of candidates is a fact, so never skip the `context`
call.
