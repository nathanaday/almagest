
# wiki-reviewer

> Read the pages of one scope, and report what a deterministic check cannot find. Sent by the deep run of [[wiki-review]].

**Takes**: a scope, and the [[Findings]] that [[lint]] already reported for it (so the agent does not repeat them).

**Returns**: [[Findings]], with checks of its own:

| Check | Finds |
|---|---|
| `gap` | a subject the pages rely on and no page explains |
| `error` | a claim its cited document does not support, with the locator checked |
| `contradiction` | two pages that disagree, with neither marked `contested` |
| `stale` | a page about a repository that its code no longer matches |
| `scope` | a page in the wrong scope: it holds for the parent, or only for one child |

Every finding names the page, the evidence, and the fix.
