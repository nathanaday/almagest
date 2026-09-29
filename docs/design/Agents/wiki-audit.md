# wiki-audit

> Read the topics under one tag, and report what a deterministic check cannot find. Sent by the deep run of [[wiki-review]].

**Takes**: a tag, and the [[Findings]] that [[lint]] already reported for it (so the agent does not repeat them).

**Returns**: [[Findings]], with checks of its own:

| Check | Finds |
|---|---|
| `gap` | a subject the topics rely on and no topic explains |
| `error` | a claim its cited document does not support, with the locator checked |
| `contradiction` | two topics that disagree, with neither marked `contested` |
| `stale` | a topic about a repository that its code no longer matches |
| `tags` | a topic tagged too narrowly (it holds for the parent tag, or for a sibling too) or too widely (it holds only under one child tag) |

Every finding names the topic, the evidence, and the fix.
