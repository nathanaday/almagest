---
name: wiki-review
description: "The health of the wiki. Quick: the deterministic checks of lint, read and explained. Deep: reviewer agents that read the topics for gaps, errors, contradictions, and stale or mistagged topics. Use for lint, wiki health, audit the wiki, review the wiki, deep review, what is wrong with the wiki, tidy the tags. The repairs are wiki-edit."
---

# wiki-review

Lint finds what code can check. Deep review sends readers for what it cannot. This
skill writes nothing; each finding names the skill that fixes it.

Tools: `lint`, `vault`. Agents: [wiki-audit](../../agents/wiki-audit.md).

## Procedure

1. **Quick**: call `lint`, with `tags` when the user names a part of the wiki (a
   document must hold every tag). Group the findings by `check`, most severe first,
   each group with its count and its `fix`.
2. **Deep**, on request: send one [wiki-audit](../../agents/wiki-audit.md) per tag with
   more than about 30 topics (waves of eight), each with the tag and the lint findings
   for it. Take the top-level tags first, from the tag list in `vault`; split a large
   tag into its child tags. Merge what the agents return with the lint findings.
3. **Tags**: read the `tag-near` findings. Each names two tags that differ only by a
   plural or a separator, or that hold the same documents. Say which pairs look like
   one tag.
4. Recommend the next step for each group:
   - knowledge findings (`dead-link`, `uncited`, `stale`, `orphan`, and the audit's
     findings) → [wiki-edit](../wiki-edit/SKILL.md);
   - `schema` and `duplicate-title` → the skill the finding's `fix` names, which
     depends on the document's type;
   - `tag-near` → [wiki-edit](../wiki-edit/SKILL.md), with a `retag` that merges the
     two, when they mean one thing;
   - `repository-behind` → [repo-ingest](../repo-ingest/SKILL.md);
   - `repository-path` → [repo-link](../repo-link/SKILL.md) with the new path,
     [repo-unlink](../repo-unlink/SKILL.md), or [wiki-edit](../wiki-edit/SKILL.md), as
     the finding's `fix` says;
   - `pending` → [wiki-sync](../wiki-sync/SKILL.md);
   - `untyped` → [wiki-ingest](../wiki-ingest/SKILL.md);
   - `thread`, `spec`, `task`, `section`, `requirement`, and `event` findings →
     [atlas](../atlas/SKILL.md) for the quick moves, or
     [thread-work](../thread-work/SKILL.md) on the thread.

## Gate

None. The skill writes nothing.

## Hand off

The skills in step 4.
