---
name: repo-unlink
description: "Unlink a repository from the vault: mark its document unlinked, and decide what happens to the plans and topics that depend on it. The document stays, so every link to it holds. The repository on disk is never touched. Use for unlink, remove this repository, forget this repo, the repository moved away. Linking is repo-link."
---

# repo-unlink

A repository can leave the vault while its code stays on disk. The repository document
stays too, marked `unlinked`, so the plans that name it and the topics that cite it keep
a live link. Agents no longer work in the old folder. The open plans that name the
repository and the topics under its tag must go somewhere, so this skill asks once, with
a recommendation for each group, and then makes the change and the `work` writes.

Tools: `context`, `search`, `work`, `change`. References:
[changes.md](../atlas/references/changes.md), [work.md](../atlas/references/work.md).

## Procedure

1. Call `context` with `repository`: its own tag (`defines`), and the open work that
   names it.
2. Count the two groups:
   - the open and started plans that name it: `work` list with `repository`;
   - the topics under its own tag: `search` with `tags: [<its tag>]` and
     `types: [topic]`.
3. Ask one question with the counts and a recommended answer for each group:
   - open plans: keep them and take the repository out of their `repositories`
     (recommended), or drop the plans;
   - topics under its tag: keep them (recommended; they still describe the system, and
     the tag and the repository document remain), or remove the ones the user names.
4. Build the change: a modify of the repository document with
   `fields: {unlinked: true}`. Code empties `path`, drops the git facts, and turns the
   lead callout into `[!repository-missing]`. Add a remove for each topic the user chose
   to remove, with `redirect` to a document that covers its subject when one exists.
   Never remove the repository document: `change` refuses it while a spec names it.
5. For the plans, per the answer: `work` set with `set: {doc, repositories}` (the list
   without this repository), or `work` drop with `doc` and `reason`.

## Gate

Propose the change. Show the preview and the plan moves together. Wait for the yes,
then apply the change and make the `work` writes. Apply runs sync, which takes the path
out of `.claude/settings.local.json`. Nothing touches the folder on disk.

## Hand off

None.
