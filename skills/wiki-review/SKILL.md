---
name: wiki-review
description: "The health of the wiki. Quick: the deterministic checks of lint, read and explained. Deep: reviewer agents that read the pages for gaps, errors, contradictions, and stale or misplaced pages. Use for lint, wiki health, audit the wiki, review the wiki, deep review, what is wrong with the wiki."
---

# wiki-review

Lint finds what code can check. Deep review sends readers for what it cannot. This
skill writes nothing; each finding names the skill that fixes it.

Tools: `lint`, `vault`. Agents: [wiki-audit](../../agents/wiki-audit.md).

## Procedure

1. **Quick**: call `lint`. Group the findings by check, most severe first, each group
   with its count and its fix.
2. **Deep**, on request: send one [wiki-audit](../../agents/wiki-audit.md) per scope with
   more than about 30 pages (waves of eight), each with the scope and the lint findings
   for it. Merge what they return with the lint findings.
3. Recommend the next step for each group:
   - wiki findings → [wiki-edit](../wiki-edit/SKILL.md);
   - `repository-behind` → [repo-ingest](../repo-ingest/SKILL.md);
   - `pending` → [wiki-sync](../wiki-sync/SKILL.md);
   - `thread` findings → [atlas](../atlas/SKILL.md) for the quick moves, or
     [thread-work](../thread-work/SKILL.md).

## Gate

None. The skill writes nothing.

## Hand off

The skills in step 3.
