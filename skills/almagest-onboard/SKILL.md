---
name: almagest-onboard
description: "Make a new Almagest vault: ask its name, its folder, and how freely to tag, then link its first repositories. Use for set up almagest, new vault, start a vault here, onboard, make this folder a vault. Linking a repository to a vault that exists is repo-link."
---

# almagest-onboard

A vault is one Obsidian vault and one git repository that holds every document of the
user's work: the sources, repositories, and topics of the wiki, the sessions, and the
changes. This skill asks the three questions a vault needs, makes it, and offers to link
the first repositories.

Tools: `vault` init. Skills: [repo-link](../repo-link/SKILL.md).

## Procedure

1. Ask for the vault's name and folder in one question. The default folder is the
   working directory when it is empty. A folder that already holds notes, or that is
   already the root of a git repository, is adopted as it is. A folder inside another
   repository is refused: the vault must be the root of its own history.
2. Ask the tag question, with option 1 as the default:
   1. "Tag freely; I will tidy later" (`open`): the agent adds a tag when no tag that
      exists fits, and names each new tag in the preview;
   2. "Use my tags; ask before a new one" (`known`): the agent uses only tags that some
      document holds, and asks before it adds one.
3. Ask for one or two sentences on what the vault is for. They become the body of
   `Almagest.md`, the context every session reads first.
4. Call `vault` with `action: init`, `name`, `path`, `tagging` (`open` or `known`), and
   `description`.
5. Offer to link repositories. The user names paths, or a folder that holds
   repositories: then list the git work trees one level below it (`ls` and a check for
   `.git`) and let the user pick. Hand the list to repo-link, which links them in one
   change and proposes the tags of each.
6. Tell the user to open the vault in Obsidian (`almagest open --register`, or
   Open folder as vault) and, for its interface, to install Almagest from Obsidian's
   community plugins (obsidian://show-plugin?id=almagest).
   The vault works without the plugin. The plugin adds the colors and icons of the
   callouts, the Approve and Cancel buttons in each change document, the Almagest palette
   (Ingest, Wiki lint, Journals, Checkouts, the agent sessions, and Safe delete), the
   repository panel, and quiet snapshots of the user's edits.

## Gate

`vault` init writes only after steps 1 to 3 are answered. repo-link has its own gate.

## Hand off

[repo-link](../repo-link/SKILL.md), then [repo-ingest](../repo-ingest/SKILL.md) for each
repository the user wants described now; [wiki-ingest](../wiki-ingest/SKILL.md) when the
user has files to add.
