
# atlas-onboard

> Make a new vault, ask how to organize it, and link its first repositories.

**Use for**: set up atlas, new vault, start a vault here, onboard.

**Tools**: `vault` init. **References**: none.

## Procedure

1. Ask for the vault's name and folder in one question. The default folder is the working directory when it is empty.
2. Ask the area question, with option 3 as the default:
   1. "I prefer a lot of areas, for the most order" (`many`);
   2. "I like to keep things simple" (`few`);
   3. "I don't know yet, or I'll create them myself" (`manual`).
3. Ask for one or two sentences on what the vault is for. They become the body of `Atlas.md`, the context every session reads.
4. Call `vault` init with the name, the path, `areas`, and the description. A folder that already holds notes, or is already a git repository, is adopted as it is.
5. Offer to link repositories. The user names paths, or a folder that holds repositories (then list the git work trees one level below it and let the user pick). Hand the list to [[repo-link]], which links them in one change.
6. Tell the user to open the vault in Obsidian (`atlas-obsidian open`) and turn on the Atlas plugin.

## Gate

`vault` init writes only after steps 1 to 3 are answered. [[repo-link]] has its own gate.

## Hand off

[[repo-link]], then [[repo-ingest]] for each repository the user wants described now; [[wiki-ingest]] when the user has files to add.
