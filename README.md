# Almagest

**Build a wiki with your coding agent, then actually read it, learn from it, and use it.**

Almagest turns an Obsidian vault into a knowledge base that Claude Code or Codex builds
with you. The agent ingests your papers, notes, and code into a stable, cited wiki. The
vault is laid out for you, the person who reads it, and every edit waits for your yes.

Status: early, in active development. macOS and Linux.

## Why Almagest

There are many LLM-wiki and second-brain projects. Most make it easy to build the wiki.
But the result is structured for the LLM, and it soon overwhelms the person who owns it.
You have built a large knowledge base, and the graph looks great. Now what?

Almagest is about the human side: understanding and working with what you built.

- **A wiki you can read.** The documents the agent maintains live in `source-core/`.
  What you read lives in `wiki-view/`: a home page, a timeline, a library, and a
  navigation page for each tag.
- **A librarian.** Ask for the material on a subject, and an agent pulls together the
  pages that serve your question, in reading order, for you to read and mark up.
- **Your own words, kept.** Your journals are yours. Agents read them and never change
  them, and an ingest never rewrites them. You publish a journal into the wiki when you
  choose, and the wiki cites it by name and date.
- **A vault that never feels fragile.** Agents generate many documents. Each edit to the
  wiki arrives as a change you approve or cancel. Safe delete moves a file to `trash/`
  only when nothing links it, so nothing is lost by accident.
- **One tool, done well.** Build the wiki, query it, and enjoy using it.

## Features

### The tool palette

One pane in Obsidian for everything Almagest does: Ingest, Wiki lint, Checkout, Publish,
Wikify, and Safe delete. It also shows what waits for you: the changes to approve, the
files to ingest, the journals with unpublished writing, and the checkouts with edits to
return.

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
of them in `checkout/` with a reading list. Read and mark up the copies. Return proposes
your edits to the originals as one change, and a ledger lists every checkout.

### Wikify a note (experimental)

Take any draft you are working on. The agent marks what the wiki already knows, with a
link, and the subjects that are new and worth a page. Each mark is a small bubble: Accept
or Ignore a link, Create or Ignore a new subject. The wikified note is a copy in your
scratchpad, and it never enters the wiki by itself.

### Safe delete

Not sure whether you can delete a page? Safe delete moves it to `trash/` when nothing
links it. When something does, it shows the links, and an agent can repoint them for
you. Empty `trash/` yourself when you like.

### A light touch on Obsidian

Installing Almagest does not change how your Obsidian looks or behaves. It adds its own
callouts, widgets inside its documents, ribbon buttons, and panes of its own: the
palette, a tag navigator, and a sessions pane. With the Duet plugin, the agents work in
Obsidian beside you; without it, they start in your terminal.

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
