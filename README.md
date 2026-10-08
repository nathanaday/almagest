# Almagest

<img src="docs/Bartolomeu_Velho_1568.jpg" alt="Geocentric model of the universe" width="480">

> *Ptolemy proposed a geocentric model of the universe in his 2nd century work **Almagest***

> *Art by Bartolomeu Velho, 1568. Public domain. [Source](https://commons.wikimedia.org/w/index.php?curid=3672259)*

---

# Build an LLM wiki you can actually use

**Why Almagest?**

_Another day, another LLM second brain plugin on Github..._

It's true! 

There are thousands of LLM-wiki and second-brain projects, and most of them are pretty good. You can painlessly transform mountains of notes, documents, and entire codebases into a fabric of markdown documents, where all the wiki links work, and your obsidian knowledge graph has never been bigger. It all works fine. My agents used it without issue. But I found the end result overwhelming. I could not navigate, read, or orientate myself in my own knowledge base.

Never has it been this easy to aggregate so much information into your own personal notes. But since you, the human, were not deeply involved in its creation, you can have all the notes in the world and still feel lost. 

Think of Almagest as the same LLM wiki concept that works great with Obsidian, with extra attention and polish for the human side. 

## Features

### A view for Humans and a structure for Agents

- The tool builds your `wiki-view`: a home page, timeline, library, and a navigation page for each tag.
- All raw sources, change logs, etc., live under `tool/` which you don't need to open

### The tool palette

One pane in Obsidian that leads to everything Almagest does. Its home lists the areas
(Changes, Ingest, Wiki health, Journals, Library, Agents, and This note), each with one
line on where it stands and a count when something waits for you. Select one to open its
page, with its numbers, its actions, and its lists.

### Ingest: a stable, accurate wiki

Drop papers, PDFs, or notes into `ingest/`, and press Ingest. Each ingest becomes one
document that you watch as it works: each step appears as the agent takes it, then the
pages it proposes, with one line on why. You read it, edit it if you like, and press
Approve or Cancel. The new pages cite their sources, and a copy of each source stays in
the vault.

Link your code repositories, and the agent writes a page for each. An agent started in
the vault finds the repository you mean and works in it, and its session leaves a
record of what it did.

### Journals

`journals/` holds your own writing, one volume per folder, such as `journals/cs566-notes/`.
When you want the wiki to learn from a volume, press Publish. Almagest captures the whole
volume as one edition, a source named like "User Journal CS566 Notes - 6 October 2026
Edition", and the agent ingests it as it would any source. Every edition stays in the
vault, and each volume keeps a publication history.

### The librarian

Ask to "check out all the material on reinforcement learning". The librarian finds the
relevant pages, follows their links only as far as they stay relevant, and puts copies
of them in `checkout/` with an index in reading order. Read and mark up the copies.
Return proposes your edits to the originals as one change, and keeps the checkout in
`tool/returned/` as you left it. A ledger lists every checkout.

### Wikify a note (experimental)

Take any draft you are working on. The agent marks what the wiki already knows, with a
link, and the subjects that are new and worth a page. Each mark is a small bubble: Accept
or Ignore a link, Create or Ignore a new subject. The wikified note is a copy in your
scratchpad, and it never enters the wiki by itself.

### Safe delete

Not sure whether you can delete a page? Safe delete moves it to `tool/trash/` when nothing
links it. When something does, it shows the links, and an agent can repoint them for
you. Empty `tool/trash/` yourself when you like.

### A light touch on Obsidian

Installing Almagest does not change how your Obsidian behaves. It adds its own callouts,
widgets inside its documents, and one pane of its own, the palette, with one ribbon button.
In the file explorer it colors the folders you use: `wiki-view/` in cyan, `journals/` and
`ingest/` in purple, and `tool/` dimmed (a setting turns this off). With the Duet plugin,
the agents work in Obsidian beside you; without it, they start in your terminal.

## Quickstart

You need macOS or Linux with `git` and `curl`, Claude Code (or Codex), and Obsidian.

1. **Install the agent plugin** in Claude Code, then restart Claude Code:

   ```bash
   claude plugin marketplace add nathanaday/almagest
   claude plugin install almagest@nathanaday-almagest
   ```

   Its first session installs the `almagest` program for your system, checked against
   the checksum the plugin carries. You build nothing. For Codex, see the
   [guide](docs/guide.md#codex).

2. **Make a vault.** Start Claude Code in an empty folder and say **"set up almagest"**.
   The agent asks a few questions, makes the vault, and offers to link your code
   repositories.

3. **Open it in Obsidian** (Open folder as vault), and install **Almagest** from the
   community plugins (`obsidian://show-plugin?id=almagest`) for the palette and the
   Approve buttons.

4. **Ingest something.** Put a paper or a few notes in `ingest/`, then press Ingest in
   the palette, or say **"ingest my files"** to your agent in the vault.

5. **Approve.** Read the ingest document in `changes/`, then press Approve. Open
   `wiki-view/` to read your wiki.

`almagest doctor` checks the whole install when something does not work.

## Coming next

- **Socratic learning:** an agent that quizzes you on a checkout, to learn it.
- **Mastery:** a score for each page, and the focus areas you are working to master.
- **Guided journaling:** an agent that helps you reflect on your project and priorities.

## Learn more

- [The user guide](docs/guide.md): what to ask, install and update, the files in a vault,
  each feature in depth, and the settings.
- [SECURITY.md](SECURITY.md): what agents may and may not do, and how the program is
  installed and verified.
- [CLAUDE.md](CLAUDE.md): notes for anyone who works on this code.
- [obsidian-almagest](https://github.com/nathanaday/obsidian-almagest): the Obsidian
  plugin.

## License

MIT. See [LICENSE](LICENSE).
