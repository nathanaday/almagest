# Almagest user guide

The reference for Almagest: what to ask, how it installs and updates, what lives in a
vault, each feature in depth, and the settings. Start with the [README](../README.md).

## Contents

- [What to ask](#what-to-ask)
- [Install and update](#install-and-update)
- [Your vault](#your-vault)
- [Journals](#journals), [Checkouts](#checkouts), [Wikify a note](#wikify-a-note-experimental), [The Obsidian plugin](#the-obsidian-plugin)
- [Settings](#settings)
- [Design principles](#design-principles) and [Repository layout](#repository-layout)

## What to ask

Start your agent in the vault and ask in plain words. The `almagest` skill routes each
request to the skill that does it.

- "Link the repository at ~/code/p3-edge under the tag work/p3." The agent proposes a
  change; you say yes, or press Approve in the change document in Obsidian. The agent
  cannot apply a change that writes until you send a prompt in the session that
  proposed it. A change with no writes
  that only marks sources as absorbed applies at once. This gate holds against the almagest
  tools; it is not a sandbox against an agent's shell. [SECURITY.md](../SECURITY.md) gives
  the full rule.
- "Describe p3-edge in the wiki." The agent snapshots the code and proposes the pages.
- "In p3-edge, score boxes by motion." The agent finds the repository, reads its
  instruction files and the policies of the wiki that apply to it, and edits the code.
  On long work it writes where the work stands under `## Progress` in its session
  document.
- "Note this: try a smaller backbone on p3-edge." The agent writes the idea in a note in
  `scratchpad/`.
- "Start agent" in Obsidian opens a terminal with your agent in the vault. The agent
  preferences choose the agent and the terminal (see
  [Agent preferences](#agent-preferences)). The Agents page of the palette shows the
  open sessions and the ones that closed in the last two hours, with Resume.
- "Ingest my files." Files you dropped in `ingest/` become cited wiki pages. The agent
  writes each step in one work document in `changes/`, then proposes the pages into the
  same document. You decide once, at the end.
- "What waits for me?" The agent lists the proposed changes, the running work
  documents, the files in `ingest/`, the sources that wait for the wiki, and the
  checkouts with edits to return.
- The Almagest palette in Obsidian starts an ingest, a wiki lint, a safe delete, a
  checkout, the publish of a journal, or a wikify of the open note with one button. See
  [The Obsidian plugin](#the-obsidian-plugin).
- "Publish my journal." The agent asks you to press Publish in the palette: only you
  publish a journal. See [Journals](#journals).
- "What do we know about my cs513 self-driving project?" The agent searches the
  documents that hold both tags.
- "Check out the material on reinforcement learning." The agent chooses the documents
  that serve the request and copies them into `checkout/`, with a reading list. See
  [Checkouts](#checkouts).
- "Wikify this note." (experimental) The agent marks a copy of the note with what the
  wiki knows and the subjects worth a topic. You accept or ignore each mark. See
  [Wikify a note](#wikify-a-note-experimental).

A **work document** is a change document that starts before the agent knows its writes.
Its status is `running`. The agent adds one line under `## Progress` for each step
(captured, extracted 9 of 12 chunks, matched, drafted), then proposes its writes into it.
A proposed change opens with a `## Summary`: one line per write, with the reason. Cancel
on a running document stops the agent.

The same actions work from a shell:

```bash
almagest vault                      # the state of the vault
almagest search "remote update" --tag work/p3
almagest change show chg-r8m3tb     # a proposed change
almagest change start --kind ingest --file paper.pdf   # a work document for an ingest
almagest change progress chg-r8m3tb "captured 1 source"  # a step, in one line
almagest vault trash scratchpad/Draft.md   # safe delete; exits 2 when files link it
almagest journal list               # the journal volumes, each with its latest edition
almagest journal publish cs566-notes   # publish a volume as a new edition
almagest checkout                   # the checkouts, newest first
almagest checkout candidates "reinforcement learning" --tag ml   # the documents a request may need
almagest checkout make order.json   # copy the documents an order names (request, name, documents, notes)
almagest checkout return "2026-10-06 Reinforcement learning"     # propose the edits of the copies
almagest wikify start Drafts/Notes.md   # copy a note to scratchpad/Notes · wikified.md
almagest wikify mark "scratchpad/Notes · wikified.md" marks.json   # write marks into the copy
almagest lint                       # the health check
almagest vault snapshot             # commit your hand edits now
```

In Obsidian, `wiki-view/` holds the notes that code writes: Home, Timeline, Library,
Repositories, one timeline per month under `wiki-view/timeline/`, and one view per tag
under `wiki-view/nav/`. `tool/sessions/Sessions.base` shows what runs now, and
`changes/Changes.base` lists the changes that wait for you.

## Install and update

### Check the install

`almagest doctor` checks the binary, the plugin, and every vault. It also starts the
plugin's MCP server as each agent runs it, and fails when the server does not list the
binary's tools. Agents in a session run `almagest` as a bare command. To run it in your
own shell, add its folder to `PATH` in your shell profile (doctor prints the line):

```bash
export PATH="$HOME/.almagest/bin:$PATH"
```

### How the binary is installed

The plugin carries a launcher, `bin/almagest`, which its MCP server and hooks run. The
launcher runs the binary of the plugin's own version, `~/.almagest/bin/<version>/almagest`.
When that file is missing, the launcher:

1. downloads the binary for your system from the GitHub release of that version,
   `https://github.com/nathanaday/almagest/releases/download/<version>/almagest-<version>-<os>-<arch>`,
   over HTTPS only;
2. checks its sha256 against the checksum that the plugin pins
   ([`release/checksums.txt`](../release/checksums.txt), also inside the launcher), and
   installs nothing when they differ;
3. installs it at `~/.almagest/bin/<version>/almagest`, points the link
   `~/.almagest/bin/almagest` at it, and adds one line to `~/.almagest/install.log`: the
   time, the version, the path, the sha256, and the URL.

It writes nothing else: no `sudo`, no shell profile, no system folder. The release CI
builds each binary from the tagged commit and publishes it only when its bytes match the
checksums that the commit pins, and it attests each file's provenance:

```bash
gh attestation verify ~/.almagest/bin/<version>/almagest --repo nathanaday/almagest
```

To use a binary of your own, set `ALMAGEST_BIN` to its path. To turn the download off,
set `ALMAGEST_NO_DOWNLOAD=1`; then install with `make install` from a clone of this
repository, which builds into the same folder. To remove Almagest from the machine,
uninstall the plugin and delete `~/.almagest/`.

### Update

Each Claude Code account holds its own copy of the plugin. Turn on auto-update for the
`nathanaday-almagest` marketplace in `/plugin`, or update by hand:

```bash
claude plugin marketplace update nathanaday-almagest
claude plugin update almagest@nathanaday-almagest
```

The next session installs the binary of the new version. The marketplace pins the
plugin to its release tag, so you never get a commit between releases. Almagest for
Obsidian updates through Obsidian's community plugins.

**A vault from 11.0.** Almagest 11.0 kept `sessions/`, `source-core/`, and `trash/` at the
vault's root; later releases keep them in `tool/`. An agent session in such a vault says
so, and every Almagest command and agent edit there waits. Migrate the vault yourself,
once, after you update the agent plugin:

```bash
almagest vault migrate --dry-run --vault ~/notes/work   # lists what moves; writes nothing
almagest vault migrate --vault ~/notes/work
```

In a session, type `! almagest vault migrate`; in Obsidian, press **Migrate the vault** in
the Almagest palette. An agent cannot run it. The migration
commits your hand edits first, then moves every file of the three folders into `tool/`
in one commit, and points the links, Bases, bookmarks, and Obsidian settings that name a
moved path at its new place. It leaves prose, code, the captured originals, and the
contents of the trash as they were. A change that waits for your answer still applies
after it. Undo takes back no change applied before the migration, since its documents
moved. Then update Almagest for Obsidian and start a new agent session.

**A second Claude Code account.** Claude Code keeps each account's plugins in its config
folder: `~/.claude`, or the folder that `CLAUDE_CONFIG_DIR` names. Install the plugin with
that variable set:

```bash
CLAUDE_CONFIG_DIR="$HOME/.claude-other" claude plugin install almagest@nathanaday-almagest
```

Every account on the machine shares the binaries (`~/.almagest/bin/`) and one list of
vaults (`~/.almagest/config.json`), so each account finds the same vaults.

### Codex

```bash
codex plugin marketplace add nathanaday/almagest
codex plugin add almagest@nathanaday-almagest
```

Codex runs a plugin's hooks only after you trust them. Without the hooks, the guard and
the session record are off. Open `/hooks` in Codex, trust the `almagest` hooks, and start
a new session. Each install or update that changes the hooks needs your trust again.
`doctor` says how many hooks Codex runs. Codex's MCP server entry holds the same launcher
as `bin/almagest`.

To update the plugin in Codex, fetch the marketplace again and add the plugin again:

```bash
codex plugin marketplace upgrade nathanaday-almagest
codex plugin remove almagest@nathanaday-almagest
codex plugin add almagest@nathanaday-almagest
```

Some parts of Almagest do not work on Codex yet:

- **No read-only agents.** Codex does not load a plugin's agents, so wiki-sync,
  wiki-review, and repo-ingest cannot send their workers, and the guard's read-only rule
  never applies.
- **No `waiting` status.** Codex has no Notification event, so a Codex session never
  shows `waiting`.
- **Linked repositories.** Almagest grants Claude Code write access to linked repositories
  through the vault's `.claude/settings.local.json`. Codex reads
  `sandbox_workspace_write.writable_roots` instead, and Almagest does not write it, so a
  Codex session may refuse a write in a linked repository, or ask before it.
- **`--allow-vault`** has no effect with `--agent codex`.

The Almagest design vault tracks these limits.

To try the plugin from a clone of this repository without installing it, start Claude
Code with `claude --plugin-dir /path/to/almagest`.

## Your vault

### Make a vault

Start Claude Code in an empty folder and say "set up almagest". Or from a shell:

```bash
almagest vault init --path ~/notes/work --name Work --tagging open \
  --description "Work notes: the p3 product and the tools around it."
almagest open --register --vault ~/notes/work   # opens it in Obsidian
```

A command that acts on a vault takes `--vault` (a folder, or a vault's name), else the
vault that `ALMAGEST_VAULT` names, else the vault above the working folder.

### Files in a vault

A vault keeps what you use at its root, and what Almagest keeps for itself in `tool/`,
which you need not open. Almagest writes these files and folders:

- `Almagest.md`, the vault's own document.
- `ingest/`: files for the wiki to learn from.
- `tool/source-core/documents/`: the documents of the wiki (topics, sources, and
  repositories), which the agent maintains through changes.
- `tool/source-core/originals/`: the captured originals and your attachments.
- `scratchpad/`: your notes, the ideas you ask an agent to note for later, and the
  wikified copies of your notes (see [Wikify a note](#wikify-a-note-experimental)).
- `journals/`: your own writing, one volume per folder. No agent edits it. See
  [Journals](#journals).
- `journals/<volume>/Journal · <volume>.md`: the publication history of a volume, which
  code writes at each publish.
- `changes/`: the change documents, which you read and approve.
- `tool/sessions/`: one document per agent session. A session that runs in a Duet
  conversation links the conversation's note, in its `conversation` property and its lead
  callout.
- `checkout/`: the librarian's checkouts, one folder each, and the ledger,
  `checkout/Checkout · Ledger.md`.
  Code writes it, and you read and edit the copies. No agent edits it. See
  [Checkouts](#checkouts).
- `tool/trash/`: what safe delete and a change's remove took out, under
  `tool/trash/<date>/<old path>`. Git keeps it. Empty it yourself.
- `wiki-view/`: the notes that code writes for reading. Each sync writes them again.
- `.obsidian/app.json`: `vault init` sends new attachments to `tool/source-core/originals/`
  (unless you chose a folder) and keeps `wiki-view/` and `tool/trash/` out of Obsidian's
  graph and search.
  It keeps every other key.
- `.obsidian/plugins/almagest/`: Almagest for Obsidian, when you install it from the
  community plugins. No Almagest tool writes it.
- `.claude/settings.local.json`: the linked repositories, for Claude Code.
- `.git/info/exclude`: `wiki-view/`, `.claude/settings.local.json`, Obsidian's workspace
  and graph files, `.DS_Store`, and Almagest's temporary `.almagest-*` files stay out of the
  vault's history.
- `.almagest/config.json`: the vault's agent preferences. Git commits it, so a shared vault
  shares it; see [SECURITY.md](../SECURITY.md).

Search and lint skip `scratchpad/`, `journals/`, and `checkout/`, but a link
to a note there still resolves. Almagest skips `tool/trash/` entirely.

## Journals

A journal holds your own thoughts and writing. Agents read it and never change it, and
an ingest never rewrites it. You decide when the wiki learns from it.

- **Volumes.** Each folder directly under `journals/` is a volume, and its subfolders
  are sections. The volume's notes are every `.md` file under the folder, except its
  publication history. The folder name gives the volume's name: `cs566-notes` reads
  "CS566 Notes".
- **Publish.** Press Publish next to the volume on the palette's Journals page, or run
  `almagest journal publish <volume>`. Almagest copies the volume into one source, an
  edition, in one commit. The palette then starts a work document and an agent that
  absorbs the edition into the wiki; you approve its change as usual. Publish refuses a
  volume with no note, and a volume with no change since its latest edition. An agent
  cannot publish: the guard refuses the command from its shell.
- **Editions.** An edition's title is `User Journal <Name> - <D Month YYYY> Edition`,
  such as "User Journal CS566 Notes - 6 October 2026 Edition". A second edition on one
  day ends in " (2)". The edition holds each note under a heading with its path in the
  volume, without its frontmatter. Its fields are `origin: journal`, `authority:
  primary`, `volume`, `edition` (the date), and `journal_hash`. It keeps the tags of the
  volume's latest edition. Every edition stays in `tool/source-core/originals/`, and topics
  cite the edition, not the notes.
- **Publication history.** Each publish writes `Journal · <volume>.md` at the volume's
  root: a table of the volume's editions. No document can take a title that begins with
  "Journal · ", so a link to the note names one file. Code owns the note; an edit there is lost at
  the next publish.
- **Changes to publish.** A volume has changes when its notes differ from its latest
  edition, or when it has notes and no edition. The date alone does not count as a
  change. Home, the palette, `journal list`, and the agent's opening context name such
  volumes.

## Checkouts

Ask an agent for the material on a subject, and it acts as a librarian (the
`wiki-checkout` skill): it chooses the documents that serve your request, puts them in
reading order, and checks out a copy of each for you to read and mark up.

- **The choice.** `checkout candidates` ranks the documents with the search, takes the
  best hits, and adds the documents linked to or from them, up to two links away. The
  agent reads each candidate's description, and the document itself when the
  description does not settle it. It keeps the documents that serve the request, and
  stops following a branch where its documents stop serving it. It aims for a sitting
  or a week of reading, at most about 30 documents. When more serve, it asks you once
  to narrow the request. A checkout holds at most 60 documents.
- **The folder.** `checkout/<date> <name>/` holds the copies and a reading list. Code
  writes it in one commit. A second checkout with the same date and name ends in
  " (2)".
- **The copies.** A copy is `<Title> (checkout).md`: the original's text below a callout
  that names the original. The copy takes its own name so that a `[[Title]]` link in the
  wiki still names one file, the original. A link to another document of the checkout
  points at that document's copy; every other link points at the wiki. Edit the copies
  as you like. No agent edits `checkout/`.
- **The reading list.** `Checkout · <folder>.md` holds your request, the documents in reading
  order with one line each on why they are there, and the agent's notes on what it left
  out.
- **The ledger.** `checkout/Checkout · Ledger.md` lists every checkout, newest first: the date, the
  request, the count of documents, the count of edited copies, and the date of its
  return. Code writes it again at each checkout and each return, so an edit there is
  lost.
- **Return.** Press Return next to the checkout on the palette's Library page, or run
  `almagest checkout return <folder>`. Almagest proposes one change, "Return <folder>",
  with a modify of each original whose copy you edited. The links to copies point at the
  originals again. You decide in the change document, as for every change. Return skips a
  copy whose original changed since the checkout, and names it; its edits stay in the
  copy. Return carries a copy's text, not its frontmatter.

## Wikify a note (experimental)

Wikify shows what the wiki knows in a note of yours, and which of its subjects the wiki
lacks. It works on a copy, and nothing enters the wiki until you create a topic through
a change.

- **The copy.** Press Wikify this note on the palette's This note page, or ask an agent to
  wikify a note. `wikify start` copies the note to `scratchpad/<name> · wikified.md` (with
  " (2)" when that name is taken). The original stays as it is. The copy lies in
  `scratchpad/`, so a copy of a journal note never joins the volume's edition. Wikify
  refuses a file that is not markdown, `Almagest.md`, and the folders that code writes:
  `tool/source-core/`, `changes/`, `tool/sessions/`, `wiki-view/`, `tool/trash/`, and `.obsidian/`.
- **The marks.** The agent (the `wiki-wikify` skill) matches the note's subjects
  against the wiki, then calls `wikify mark` once. A mark is inline text:
  `{{link:<Title>|<phrase>}}` where the phrase names a document of the wiki, and
  `{{new:<Title>|<phrase>}}` where it names a subject worth a topic. Each mark replaces
  the first mention of its phrase, as whole words and in any case, outside the
  frontmatter, headings, table rows, code, links, URLs, and other marks. The agent marks a few new
  subjects at most, and never marks a phrase inside a quote of another person's words.
  Almagest commits nothing; the next quiet snapshot keeps the copy.
- **The bubbles.** The plugin shows each mark as a bubble: the phrase, then `→ Title`
  for a link or `+ Title` for a new subject, and buttons. It does this in live preview
  and in reading view.
  - **Accept** (a link) turns the mark into `[[Title|phrase]]`, or `[[Title]]` when the
    phrase is the title.
  - **Ignore** turns the mark back into its phrase.
  - **Create** (a new subject) starts a draft work document and an agent that drafts
    the topic and proposes it into that document. You approve it as any change.
  - **Link** appears on a new subject once its topic exists, and turns the mark into a
    link.
  - A link whose document is gone (deleted or renamed since the mark) shows "no note"
    and offers Ignore only.
  - The command "Accept every link mark in this note" accepts every link whose document
    exists, at once.

## The Obsidian plugin

The vault works without the plugin. With it, Obsidian adds:

- **The Almagest palette** in the right sidebar (the Almagest ribbon button, or the command
  "Open the tool palette"). Its home lists the areas of Almagest, each with one line on
  where it stands and a count when something waits for you. Select an area to open its
  page: what it is, its numbers, its actions, and its lists.
  - **Changes**: the changes to review, and the running work documents with their last
    step.
  - **Ingest**: the files in `ingest/`. **Ingest** starts a work document for them, opens
    it, and starts an agent that reports into it.
  - **Wiki health**: **Run wiki lint** lists the first findings, and **Repair with an
    agent** starts a repair work document and an agent that proposes the repairs into it.
    **Sync the vault** writes the views and the statuses again (the command "Sync the
    vault" does the same).
  - **Journals**: each volume, marked "changed" when it has writing to publish.
    **Publish** runs `almagest journal publish`, then starts a work document and an agent
    that absorbs the edition. See [Journals](#journals).
  - **Library**: **Check out material** asks for your request and starts the librarian.
    **Return** next to a checkout runs `almagest checkout return` and opens the change;
    it is on when a copy is edited and the checkout is not returned. See
    [Checkouts](#checkouts).
  - **Agents**: **Start an agent**, the agents Almagest started that still work, and the
    agent sessions of the vault as message threads: the open ones, each with its state
    and its last progress line, then the ones that closed in the last two hours, folded
    away, with **Resume**. A thread opens the session's Duet conversation when it runs in
    one, else the session's document. A session that needs you counts on the home row.
  - **This note**: **Wikify this note** (experimental) copies the open note to
    `scratchpad/`, opens the copy, and starts an agent that marks it (see
    [Wikify a note](#wikify-a-note-experimental)). **Safe delete this note**
    runs `almagest vault trash` on it. When nothing links it, it moves to `tool/trash/`; a
    topic, a source, or a repository leaves through a change that applies at once, so
    `change undo` in a terminal brings it back, and no agent can undo it. When files link
    it, nothing moves, and a list names the links. For a knowledge document that
    documents link, the list offers **Resolve with an agent**: the agent points each link
    in a document elsewhere and proposes the remove. A link in your own notes (the
    scratchpad, `journals/`, `checkout/`, and the like) is yours to fix; while one stays,
    the agent proposes no remove, and you run Safe delete again after you fix it.

  The setting **Agent conversations** chooses where an agent works: **Duet
  (recommended)**, in a conversation note of the vault through the Duet plugin, or
  **Terminal (configurable)**, in a new terminal with the agent and terminal settings of
  Almagest. Every feature works with either. While Duet is the choice but is not installed
  or not on, agents start in a terminal, the settings name what Duet needs, and a tip
  recommends Duet at the top of `Almagest.md` and in each new terminal. The plugin draws
  the tip in `Almagest.md` and never writes it into the file. Choose Terminal, and the tip
  goes away. Resume of a closed terminal session always opens a terminal.
- **Wikify bubbles** in a wikified copy: Accept, Ignore, Create, and Link on each mark.
- **Approve and Cancel** in each change document. Approve applies the change, as
  `almagest change apply` does in a terminal. Cancel asks for an optional reason
  and rejects the change. After the decision, the document shows the result. A running
  work document shows its kind, its last progress line, and Cancel.
- **Quiet snapshots.** After two minutes with no file change, the plugin commits your
  edits to the vault's git history. Set the period in the Almagest settings; 0 turns it
  off. Every Almagest write also commits your edits first, so you need not commit by hand.
- **The repository panel** in each repository document: the branch, the head, and the
  uncommitted files of the linked repository.
- Colors and icons for the callouts of Almagest documents.
- **Folder colors** in the file explorer: what you read (`wiki-view/`) in cyan, what you
  write and add (`journals/`, `ingest/`) in purple, and what Almagest keeps for itself
  (`tool/`) dimmed. The setting "Color Almagest's folders" turns them off.
- **Migrate an 11.0 vault.** In a vault that keeps `sessions/`, `source-core/`, and
  `trash/` at its root, a notice and the palette offer the migration: the palette shows
  how many files move and change, and **Migrate the vault** runs `almagest vault migrate`,
  which moves them into `tool/` in one commit. See [Update](#update).
- A sync of the views a few seconds after you edit a note.

## Settings

### Agent preferences

Start agent reads four preferences, and Resume reads `terminal` and `terminal_command`:

| Key                     | Values                                                  | Default    |
| ----------------------- | ------------------------------------------------------- | ---------- |
| `agent`                 | `claude`, `codex`                                       | `claude`   |
| `agent_commands.<agent>`| the command as you type it in a shell, such as a shell function | the agent's name |
| `terminal`              | `terminal`, `iterm`, `wezterm`, `ghostty`, `custom`     | `terminal` |
| `terminal_command`      | for `custom`: a command with `{command}` for the agent's command | none |

Two files hold them. `~/.almagest/config.json` holds them for every vault. `.almagest/config.json`
in a vault overrides them, key by key: when both files set a key, the vault's value wins.
Set them in the Almagest settings in Obsidian, or from a shell:

```bash
almagest config set terminal wezterm --global        # every vault
almagest config set agent_commands.claude claude-work  # this vault: another account
almagest config                                      # the result, and where each value comes from
almagest config unset agent_commands.claude          # back to the global value
```

The terminal runs the command in your login shell, so your `PATH` and shell functions
apply. [TESTED.md](../TESTED.md) lists the agent and terminal pairs we tested.

### Environment variables

| Variable | What it does |
| --- | --- |
| `ALMAGEST_HOME` | The machine folder: the binaries, `config.json` with the list of vaults, the global preferences, and `install.log`. Default `~/.almagest`. |
| `ALMAGEST_VAULT` | The vault a command, the MCP server, or a hook uses when the call names none: a folder or a vault's name. |
| `ALMAGEST_BIN` | A binary of your own, which the launcher runs in place of the plugin's version; it then downloads nothing. |
| `ALMAGEST_NO_DOWNLOAD` | Set to `1`, the launcher never downloads a binary. |
| `ALMAGEST_HOOK_LOG` | A file that gets every hook event in full, your prompts included. Use it only to debug. |
| `CLAUDE_CONFIG_DIR` | Claude Code's config folder, which `setup` and `doctor` read. Default `~/.claude`. |
| `CLAUDE_PROJECT_DIR` | The folder in which `almagest mcp` looks for the vault, when set. |
| `OBSIDIAN_CONFIG_DIR` | The folder of Obsidian's `obsidian.json`, which `open --register` edits. |
| `CODEX_HOME` | Codex's own folder. Almagest does not read it, but the `codex` commands that `setup` and `doctor` run do. |

## Design principles

These rules hold everywhere in Almagest:

- Everything is a document with an id and a type. No database and no state folder.
- Code owns what code can derive: ids, statuses, links, hashes, git facts, the first
  callout of each document, the views. The model writes prose.
- Knowledge changes only through a change document, and an agent's apply of a change that
  writes waits for your turn.
- An agent edits a linked repository directly. The session record lists the repositories
  each session touched.
- Hooks keep a document for every session, so the record does not depend on the model.

## Repository layout

- The binary: `cmd/almagest/` and `internal/`, one package per part.
  `internal/mcpserver` serves the nine tools: `vault`, `search`, `context`, `match`,
  `source`, `change`, `checkout`, `wikify`, and `lint`. `internal/hooks` serves the nine hooks,
  and `internal/cli` every command.
- The agent plugin: `skills/` (fourteen skills), `agents/` (three read-only agents),
  `hooks/hooks.json`, `.mcp.json`, `.claude-plugin/`, `.codex-plugin/`, and
  `bin/almagest`, the launcher. `.agents/plugins/marketplace.json` is the Codex
  marketplace entry.
- The release: `release/checksums.txt` (the pinned sha256 of each binary),
  `internal/release` (the launcher's template and `make pin`), and
  `.github/workflows/release.yml`.
- The Obsidian plugin is in its own repository, `obsidian-almagest`.
- Notes for agents that work on this code: [CLAUDE.md](../CLAUDE.md).
