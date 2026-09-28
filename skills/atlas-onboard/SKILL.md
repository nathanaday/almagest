---
name: atlas-onboard
description: "Make a new Atlas vault: ask its name, folder, and how to organize it, then link its first repositories. Use for set up atlas, new vault, start a vault here, onboard, make this folder a vault."
---

# atlas-onboard

A vault is one Obsidian vault and one git repository that holds the wiki, the threads,
the sessions, and the changes of the user's work. This skill asks the three questions a
vault needs, makes it, and offers to link the first repositories.

Tools: `vault` init. Skills: [repo-link](../repo-link/SKILL.md).

## Procedure

1. Ask for the vault's name and folder in one question. The default folder is the
   working directory when it is empty. A folder that already holds notes, or that is
   already the root of a git repository, is adopted as it is. A folder inside another
   repository is refused: the vault must be the root of its own history.
2. Ask the area question, with option 3 as the default:
   1. "I prefer a lot of areas, for the most order" (`many`);
   2. "I like to keep things simple" (`few`);
   3. "I don't know yet, or I'll create them myself" (`manual`).
3. Ask for one or two sentences on what the vault is for. They become the body of
   `Atlas.md`, the context every session reads first.
4. Call `vault` with `action: init`, the name, the path, `areas`, and the description.
5. Offer to link repositories. The user names paths, or a folder that holds
   repositories: then list the git work trees one level below it (`ls` and a check for
   `.git`) and let the user pick. Hand the list to repo-link, which links them in one
   change.
6. Tell the user to open the vault in Obsidian (`atlas open --register`) and to turn on
   the Atlas plugin under Community plugins once. The vault works without the plugin;
   the plugin adds colors, the sessions pane, and the Apply button.

## Gate

`vault` init writes only after steps 1 to 3 are answered. repo-link has its own gate.

## Hand off

[repo-link](../repo-link/SKILL.md), then [repo-ingest](../repo-ingest/SKILL.md) for each
repository the user wants described now; [wiki-ingest](../wiki-ingest/SKILL.md) when the
user has files to add.
