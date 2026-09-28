
# wiki-review

> The health of the wiki. Quick: the deterministic checks, read and explained. Deep: reviewers that read the pages for what code cannot check.

**Use for**: lint, wiki health, audit the wiki, review the wiki, deep review, what is wrong with the wiki.

**Tools**: `lint`, `vault`. **Agents**: [[wiki-audit]]. **References**: none.

## Procedure

1. **Quick**: call `lint`. Group the [[Findings]] by check, most severe first, each group with its count and its fix.
2. **Deep**, on request: send one [[wiki-audit]] per scope with more than about 30 pages (waves of eight), each with the lint findings for its scope. Merge what they return with the lint findings.
3. Recommend the next step for each group:
   - wiki findings → [[wiki-edit]];
   - `repository-behind` → [[repo-ingest]];
   - `pending` → [[wiki-sync]];
   - `thread` findings → [[atlas]], for the quick moves, or [[thread-work]].

## Gate

None. The skill writes nothing.

## Hand off

The skills in step 3.
