# wiki-review

> The health of the wiki. Quick: the deterministic checks, read and explained. Deep: reviewers that read the topics for what code cannot check.

**Use for**: lint, wiki health, audit the wiki, review the wiki, deep review, what is wrong with the wiki, tidy the tags.

**Tools**: `lint`, `vault`. **Agents**: [[wiki-audit]]. **References**: none.

## Procedure

1. **Quick**: call `lint`. Group the [[Findings]] by check, most severe first, each group with its count and its fix.
2. **Deep**, on request: send one [[wiki-audit]] per tag with more than about 30 topics (waves of eight), each with the lint findings for its tag. Take top-level tags first; split a large tag into its child tags. Merge what they return with the lint findings.
3. Recommend the next step for each group:
   - knowledge findings (`dead-link`, `uncited`, `stale`, `orphan`, the audit's findings) → [[wiki-edit]];
   - `tag-near` → [[wiki-edit]], with a `retag` that merges the two;
   - `repository-behind` → [[repo-ingest]];
   - `pending` → [[wiki-sync]];
   - `spec` and `event` findings → [[atlas]], for the quick moves, or [[spec-work]];
   - `untyped` → [[wiki-ingest]].

## Gate

None. The skill writes nothing.

## Hand off

The skills in step 3.
