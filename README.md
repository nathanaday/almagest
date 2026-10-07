# Almagest

**A wiki in your Obsidian vault that your coding agents write, and you approve.**

Almagest turns an Obsidian vault into a knowledge base for long projects. You keep working
with Claude Code or Codex as you do now. The agent reads your papers, notes, and code,
and writes what it learns into the vault as linked, cited pages. Nothing changes in the
wiki until you approve it.

Status: early and in active development (11.0). macOS and Linux. Almagest was named Atlas
before 11.0.

## What Almagest does

Long projects with agents tend to fail in one of two ways. You delegate, and you lose track
of what was decided and why. Or you spend your energy keeping notes and documents current.
Almagest gives that work to the agent, and keeps you in charge of it:

- **The agent writes the wiki.** Drop a paper, a PDF, or a set of notes into `ingest/`, and
  the agent turns them into topic pages that cite their sources. Point it at a code
  repository, and it writes a page that describes the code.
- **You approve every edit.** Each edit arrives as a *change document*: a list of the pages
  the agent wants to create or modify, the full new text of each, and one line on why.
  You press Approve, or say yes in the chat. Until then, the wiki stays as it is.
- **Every session leaves a record.** Each agent session gets a document: what it worked on,
  which repositories it touched, and where the work stands. Next week, you and the next
  agent can pick up from there.
- **Your own writing stays yours.** Agents read your journals and never edit them. You
  decide when a journal goes into the wiki.

## Why it is different

Most AI plugins for Obsidian put a chat beside your notes, write text into the note you
have open, or search your notes through an embedding index. Almagest takes another
approach.

- **It uses the agent you already have.** Almagest is a plugin for Claude Code and Codex.
  The agent that edits your code also keeps your wiki, in the same session. Obsidian
  holds no API key. Almagest itself makes one network request: the download of its
  own program from GitHub, the first time.
- **Edits are reviewed, not trusted.** An agent cannot apply a change that writes until
  you reply in its session. Code enforces this rule through hooks, not the prompt, so a
  confused agent cannot skip it. Your safe delete, your journal publish, and the vault
  migration run only when you start them.
- **Plain files and git, no database.** Every page is a Markdown file with an id and a
  type. Every write is a git commit, so you can read the history and undo any change.
  There is no vector store and no hidden state to fall out of step.
- **Code keeps the structure; the model writes the prose.** Ids, links, tags, citations,
  the hashes of captured sources, and the overview pages come from code. A rename
  rewrites every link to the page. A lint check finds broken links and topics with no
  source. So the wiki stays consistent as it grows to hundreds of pages.
- **Citations you can check.** A source is captured once into the vault, with its
  original file and a hash. Topics cite the source, so each claim leads back to where it
  came from.
- **Knowledge and code in one place.** Repository pages link the wiki to the code
  repositories on your machine. An agent started in the vault finds the repository you
  mean through its tags, and works in it.

## How it works

```text
 what goes in            what your agent does          what you get
 ──────────────────      ───────────────────────       ──────────────────────────────
 ingest/ papers, notes ─▶ captures and drafts pages ─▶ a change ─▶ you approve ─▶ wiki
 code repositories     ─▶ reads and edits the code  ─▶ a session record
 journals/ (yours)     ─▶ reads only                ─▶ an edition, when you publish
```

The vault holds a few kinds of document:

- **Topics**: what you know, one subject per page, with citations.
- **Sources**: the papers, articles, and notes that topics cite. The original file stays
  in the vault.
- **Repositories**: one page per code repository on your machine, linked by its path.
- **Changes**: the edits an agent proposes, and their result after you decide.
- **Sessions**: one record per agent session.

Tags sort the wiki. A page may hold many tags, so a page tagged `cs513` and
`self-driving` shows under both, and a search for both finds it.

## Features

- **Ingest.** Turn the files in `ingest/` into cited topic pages. The agent reports each
  step in one work document, then proposes all the pages into it, so you decide once.
- **Repository pages.** Link a code repository, and the agent describes it. Code keeps
  the branch, the last commit, and the remote current.
- **Changes you approve.** Approve or Cancel in the change document, or in the chat. Undo
  takes back an applied change.
- **Search and context.** Search the wiki by words and tags, and give an agent the pages
  a task needs.
- **Lint and repair.** A health check for broken links, topics with no source, and
  topics whose sources changed after them. An agent can propose the repairs.
- **Safe delete.** Move a file to `trash/` only when nothing links it, or let an agent
  repoint the links first.
- **Journals.** Your own writing, which agents never edit. Publish a volume when you
  want the wiki to learn from it. See [Journals](#journals).
- **Checkouts.** Ask a librarian agent for the material on a subject. It picks the pages
  that serve your request, in reading order, and copies them for you to read and mark up.
  Return proposes your edits as one change. See [Checkouts](#checkouts).
- **Wikify a note** (experimental). Mark a copy of any note with what the wiki already
  knows and the subjects it lacks, then accept or ignore each mark. See
  [Wikify a note](#wikify-a-note-experimental).
- **Views.** Code writes overview notes: a home page, a timeline, a library, the
  repositories, and one page per tag.
- **Almagest for Obsidian** (optional). A tool palette, Approve and Cancel buttons in each
  change, a tag navigator, a sessions pane, and automatic snapshots of your hand edits.
  See [The Obsidian plugin](#the-obsidian-plugin).

## Quickstart

You need macOS or Linux with `git` and `curl`, Claude Code (or Codex), and Obsidian.

1. **Install the agent plugin** in Claude Code:

   ```bash
   claude plugin marketplace add nathanaday/almagest
   claude plugin install almagest@nathanaday-almagest
   ```

   Restart Claude Code. The plugin's first session installs the `almagest` program for
   your system and checks it against the checksum the plugin carries. You build nothing.
   For Codex, see [Codex](#codex).

2. **Make a vault.** Start Claude Code in an empty folder and say **"set up almagest"**.
   The agent asks a few questions, makes the vault, and offers to link your code
   repositories.

3. **Open it in Obsidian** (Open folder as vault). For the palette and the Approve
   buttons, install **Almagest** from Obsidian's community plugins
   (`obsidian://show-plugin?id=almagest`). The vault works without it.

4. **Give it something to learn.** Put a paper or a few notes in the vault's `ingest/`
   folder, start Claude Code in the vault, and say **"ingest my files"**.

5. **Review and approve.** The agent writes a change document in `changes/`. Read it, edit
   it if you like, then press Approve (or say yes). The new pages appear in
   `source-core/documents/`, and the views in `wiki-view/` show them.

Then ask questions about what the wiki holds, link more repositories, or ask "what waits
for me?". [What to ask](#what-to-ask) lists more. `almagest doctor` checks the whole install
when something does not work.

## What to ask

Start your agent in the vault and ask in plain words. The `almagest` skill routes each
request to the skill that does it.

- "Link the repository at ~/code/p3-edge under the tag work/p3." The agent proposes a
  change; you say yes, or press Approve in the change document in Obsidian. The agent
  cannot apply a change that writes until you send a prompt in the session that
  proposed it. A change with no writes
  that only marks sources as absorbed applies at once. This gate holds against the almagest
  tools; it is not a sandbox against an agent's shell. [SECURITY.md](SECURITY.md) gives
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
  [Agent preferences](#agent-preferences)). The sessions pane in the right sidebar shows
  the open sessions and the ones that closed in the last two hours, with Resume.
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
under `wiki-view/nav/`. `sessions/Sessions.base` shows what runs now, and
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
   ([`release/checksums.txt`](release/checksums.txt), also inside the launcher), and
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
Obsidian updates through Obsidian's community plugins. A vault of an earlier release
needs `almagest vault migrate` (below), which only you run.

**A second Claude Code account.** Claude Code keeps each account's plugins in its config
folder: `~/.claude`, or the folder that `CLAUDE_CONFIG_DIR` names. Install the plugin with
that variable set:

```bash
CLAUDE_CONFIG_DIR="$HOME/.claude-other" claude plugin install almagest@nathanaday-almagest
```

Every account on the machine shares the binaries (`~/.almagest/bin/`) and one list of
vaults (`~/.almagest/config.json`), so each account finds the same vaults.

**Before 11.0** the project was Atlas (`atlas-obsidian`), with `~/.atlas/`. The first
command of 11.0 copies `~/.atlas/config.json`, so your vaults carry over. Uninstall the
`atlas-obsidian` plugin, and delete `~/.atlas/` once every vault is migrated.

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

Besides `source-core/documents/`, Almagest writes these files and folders in a vault:

- `Almagest.md`, the vault's own document.
- `ingest/`: files for the wiki to learn from.
- `source-core/originals/`: the captured originals and your attachments.
- `scratchpad/`: your notes, the ideas you ask an agent to note for later, and the
  wikified copies of your notes (see [Wikify a note](#wikify-a-note-experimental)).
- `journals/`: your own writing, one volume per folder. No agent edits it. See
  [Journals](#journals).
- `journals/<volume>/Journal · <volume>.md`: the publication history of a volume, which
  code writes at each publish.
- `sessions/` and `changes/`: the session and change documents.
- `checkout/`: the librarian's checkouts, one folder each, and the ledger,
  `checkout/Checkout · Ledger.md`.
  Code writes it, and you read and edit the copies. No agent edits it. See
  [Checkouts](#checkouts).
- `trash/`: what safe delete and a change's remove took out, under
  `trash/<date>/<old path>`. Git keeps it. Empty it yourself.
- `wiki-view/`: the notes that code writes for reading. Each sync writes them again.
- `threads/`, in a vault that the 9.0 migration moved: the thread and chord documents of
  8.x.
- `.obsidian/app.json`: `vault init` sends new attachments to `source-core/originals/`
  (unless you chose a folder) and keeps `wiki-view/` and `trash/` out of Obsidian's
  graph and search.
  It keeps every other key.
- `.obsidian/plugins/almagest/`: Almagest for Obsidian, when you install it from the
  community plugins. No Almagest tool writes it.
- `.claude/settings.local.json`: the linked repositories, for Claude Code.
- `.git/info/exclude`: `wiki-view/`, `.claude/settings.local.json`, Obsidian's workspace
  and graph files, `.DS_Store`, and Almagest's temporary `.almagest-*` files stay out of the
  vault's history.
- `.almagest/config.json`: the vault's agent preferences. Git commits it, so a shared vault
  shares it; see [SECURITY.md](SECURITY.md).

Search and lint skip `scratchpad/`, `journals/`, `threads/`, and `checkout/`, but a link
to a note there still resolves. Almagest skips `trash/` entirely.

### Migrate a vault from an earlier release

A vault made by an earlier release (Atlas 8.x, 9.0, or 10.x) needs one migration to the
11.0 layout. Almagest for Obsidian shows a notice that opens it. From a shell,
`almagest vault migrate --dry-run` lists the moves, and `almagest vault migrate` makes
every step in one commit. Only you run it; the guard refuses it from an agent.

**From 10.0** (the rename from Atlas to Almagest), the migration renames only what code
wrote:

- `Atlas.md` becomes `Almagest.md`, and `.atlas/` becomes `.almagest/`;
- the `atlas-change` and `atlas-repo` blocks, the change documents' class, and the
  callouts of checkouts and publication histories take the name Almagest;
- links to `[[Atlas]]` point at `[[Almagest]]`, unless another note is titled Atlas.

Your own text stays as you wrote it, even where it says "Atlas". The old Atlas plugin in
`.obsidian/plugins/atlas/` stays too: turn it off and remove it in Obsidian, then install
Almagest from the community plugins. The migration's report reminds you.

**From 9.0**, it first renames the folders of 10.0:

| 9.0 | 10.0 |
| --- | --- |
| `wiki/documents/` | `source-core/documents/` |
| `wiki/assets/` | `source-core/originals/` |
| `inbox/` | `ingest/` |
| `views/` | `wiki-view/`, written again by code; a note of yours there moves to `ingest/` |
| `views/tags/` | `wiki-view/nav/` |

- Links, embeds, and Bases that name a moved folder follow the move. Prose that names a
  folder stays as you wrote it.
- A source captured from the old inbox gets `origin: ingest`.
- New attachments go to `source-core/originals/`, and `wiki-view/` stays out of
  Obsidian's search, unless you chose other settings.

**From 8.x**, it first takes the step to 9.0:

- every stub, spec, task list, verification, chord, and event document moves from
  `wiki/documents/` to `threads/`, with its file name and content unchanged;
- every canvas in `chords/` moves to `threads/`, and `chords/` goes;
- topics and repositories lose their `## Threads` section; topics and sources lose
  `from`; sessions lose `threads`, `specs`, `work`, `checked`, and `events`; changes
  lose `work`;
- the vault document loses `wikify`.

The dry run of a 9.0 or 8.x vault lists its first step only, since the next ones read
what it moves. The migration refuses while a change is proposed: apply or reject it
first, with the release that made the vault. It also refuses when a file already exists
where it would move one. A vault older than 8.0 migrates with release 8.1.1 (tag
`threads-final`) first.

## Features in depth

### Journals

A journal holds your own thoughts and writing. Agents read it and never change it, and
an ingest never rewrites it. You decide when the wiki learns from it.

- **Volumes.** Each folder directly under `journals/` is a volume, and its subfolders
  are sections. The volume's notes are every `.md` file under the folder, except its
  publication history. The folder name gives the volume's name: `cs566-notes` reads
  "CS566 Notes".
- **Publish.** Press Publish next to a volume in the palette, or run
  `almagest journal publish <volume>`. Almagest copies the volume into one source,
  an edition, in one commit. The palette then starts a work document and an agent that
  absorbs the edition into the wiki; you approve its change as usual. Publish refuses a
  volume with no note, and a volume with no change since its latest edition. An agent
  cannot publish: the guard refuses the command from its shell.
- **Editions.** An edition's title is `User Journal <Name> - <D Month YYYY> Edition`,
  such as "User Journal CS566 Notes - 6 October 2026 Edition". A second edition on one
  day ends in " (2)". The edition holds each note under a heading with its path in the
  volume, without its frontmatter. Its fields are `origin: journal`, `authority:
  primary`, `volume`, `edition` (the date), and `journal_hash`. It keeps the tags of the
  volume's latest edition. Every edition stays in `source-core/originals/`, and topics
  cite the edition, not the notes.
- **Publication history.** Each publish writes `Journal · <volume>.md` at the volume's
  root: a table of the volume's editions. No document can take a title that begins with
  "Journal · ", so a link to the note names one file. Code owns the note; an edit there is lost at
  the next publish.
- **Changes to publish.** A volume has changes when its notes differ from its latest
  edition, or when it has notes and no edition. The date alone does not count as a
  change. Home, the palette, `journal list`, and the agent's opening context name such
  volumes.

### Checkouts

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
- **Return.** Press Return next to the checkout in the palette, or run `almagest
  checkout return <folder>`. Almagest proposes one change, "Return <folder>", with a modify
  of each original whose copy you edited. The links to copies point at the originals
  again. You decide in the change document, as for every change. Return skips a copy
  whose original changed since the checkout, and names it; its edits stay in the copy.
  Return carries a copy's text, not its frontmatter.

### Wikify a note (experimental)

Wikify shows what the wiki knows in a note of yours, and which of its subjects the wiki
lacks. It works on a copy, and nothing enters the wiki until you create a topic through
a change.

- **The copy.** Press Wikify this note in the palette, or ask an agent to wikify a
  note. `wikify start` copies the note to `scratchpad/<name> · wikified.md` (with
  " (2)" when that name is taken). The original stays as it is. The copy lies in
  `scratchpad/`, so a copy of a journal note never joins the volume's edition. Wikify
  refuses a file that is not markdown, `Almagest.md`, and the folders that code writes:
  `source-core/`, `changes/`, `sessions/`, `wiki-view/`, `trash/`, and `.obsidian/`.
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

### The Obsidian plugin

The vault works without the plugin. With it, Obsidian adds:

- **The Almagest palette** in the right sidebar (the Almagest ribbon button, or the command
  "Open the Almagest palette"). It shows the proposed changes, the running work documents,
  the files in `ingest/`, the pending sources, the live sessions, the files in `trash/`,
  the journal volumes, the checkouts, and the lint problems. Its actions:
  - **Ingest** starts a work document for the files in `ingest/`, opens it, and starts
    an agent that reports into it.
  - **Wiki lint** runs `lint` and lists the first findings. **Repair with an agent**
    starts a repair work document and an agent that proposes the repairs into it.
  - **Safe delete this file** runs `almagest vault trash` on the open file. When
    no file links it, the file moves to `trash/`; a topic, a source, or a repository
    leaves through a change that applies at once, so `change undo` in a terminal brings
    it back; an agent cannot undo it. When files link it, nothing moves, and a list
    names the links. For a knowledge document that documents link, the list offers
    **Resolve with an agent**: the agent points each link in a document elsewhere and
    proposes the remove. A link in your own notes (the scratchpad, `journals/`,
    `checkout/`, `threads/`, and the like) is yours to fix; while one stays, the agent
    proposes no remove, and you run Safe delete again after you fix it.
  - **Publish** next to a journal volume (marked when the volume has changes) runs
    `almagest journal publish`, then starts a work document and an agent that
    absorbs the edition. See [Journals](#journals).
  - **Checkout** asks for your request and starts an agent that checks out the
    material on it.
  - **Return** next to a checkout runs `almagest checkout return` and opens the
    change. It is on when a copy is edited and the checkout is not returned. See
    [Checkouts](#checkouts).
  - **Wikify this note** (experimental) copies the open note to `scratchpad/`, opens
    the copy, and starts an agent that marks it. See
    [Wikify a note](#wikify-a-note-experimental).

  The palette starts an agent through the Duet plugin. Without Duet, it starts your
  agent in a terminal (see [Agent preferences](#agent-preferences)) with the same
  message.
- **Wikify bubbles** in a wikified copy: Accept, Ignore, Create, and Link on each mark.
- **Approve and Cancel** in each change document. Approve applies the change, as
  `almagest change apply` does in a terminal. Cancel asks for an optional reason
  and rejects the change. After the decision, the document shows the result. A running
  work document shows its kind, its last progress line, and Cancel.
- **Quiet snapshots.** After two minutes with no file change, the plugin commits your
  edits to the vault's git history. Set the period in the Almagest settings; 0 turns it
  off. Every Almagest write also commits your edits first, so you need not commit by hand.
- **The Almagest navigator** in the left sidebar, which narrows the documents one tag at a
  time.
- **The sessions pane** in the right sidebar, with Resume, and the **Start agent**
  command.
- **The repository panel** in each repository document: the branch, the head, and the
  uncommitted files of the linked repository.
- Colors and icons for the callouts of Almagest documents.
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
apply. [TESTED.md](TESTED.md) lists the agent and terminal pairs we tested.

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
- `v7-design/`: the design pages of 7.0, kept for reference.
- Notes for agents that work on this code: [CLAUDE.md](CLAUDE.md).

## License

MIT. See [LICENSE](LICENSE).
