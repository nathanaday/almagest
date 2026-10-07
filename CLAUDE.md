# almagest

Almagest: the Go module `github.com/nathanaday/almagest` (binary `almagest`) and the
agent plugin `almagest` in the repository's own marketplace. The Obsidian plugin lives in
its own repository, `obsidian-almagest` (`~/projects/software/obsidian-almagest`), and
reaches users only through Obsidian's community plugins. Read `README.md` and
`docs/guide.md` first. This file holds what the code and those do not say.

- **The binary** serves the MCP tools (`internal/mcpserver`), the hooks
  (`internal/hooks`), and the CLI (`internal/cli`). The vault, its index, and the write
  transaction are `internal/vault`; changes are `internal/change`.
- **The agent plugin** is the skills, the read-only agents, the hooks, and the launcher
  (`bin/almagest`), which installs the binary of the plugin's own version from the
  GitHub release, checked against the sha256 it pins (`release/checksums.txt`).
- **The Obsidian plugin** runs the CLI for every read and write. Duet
  (`~/projects/software/obsidian-duet`) hosts the agents in the editor when it is on;
  Almagest starts them through Duet's API and does not copy its code.

The plans behind the features are notes in the SoftwareProjects vault:
`scratchpad/Almagest 10 Strategy.md` (the palette, work documents, safe delete,
journals, checkouts, wikify) and `scratchpad/Distribution and Rename Plan.md` (the
release, the launcher, and the Obsidian plugin's repository).

## Sources of truth

| Thing | Location |
|---|---|
| The design: rules, document types, tools, hooks, skills | the SoftwareProjects vault (`~/Vaults/SoftwareProjects`), tag `tools/almagest`: the design pages are sources there (start with `Almagest 7`), absorbed into topics |
| Each skill's contract | `skills/<name>/SKILL.md`, `skills/almagest/references/` |
| Each read-only agent | `agents/<name>.md` |
| The schemas of the six types | `internal/schema/schema.go` |

The design pages are the spec. When the code departs from them, the reason is below.

## Design decisions

- **`tool/` holds what Almagest keeps for itself** (layout 8): `tool/source-core/`,
  `tool/sessions/`, and `tool/trash/`. The root holds what the user uses: `wiki-view/`,
  `journals/`, `ingest/`, `changes/`, `checkout/`, and `scratchpad/`. Every folder name
  is a constant in `internal/vault/vault.go`; `internal/schema` repeats two, since it
  cannot import `vault`. A folder check uses `vault.InFolder`, never the first path part.
- **Only the user migrates a vault.** `vault migrate` (`internal/migrate`) takes layout 7
  (`vault.LayoutBeforeTool`, the 11.0 layout) to 8 in one commit: it moves the three
  folders and rewrites a path only in a link, a Base, a `base` block, a canvas,
  `bookmarks.json`, and `app.json`. A closing fence must be as long as its opener, so the
  writes that a change document fences in five backticks keep their history. It does
  not refuse a pending change: a change names documents by id, and its base ignores the
  code sections that a sync rewrites. The guard refuses the command to an agent. In a
  vault of another layout, no hook writes (so no session record lands in a folder the
  layout lacks), the opening context names the fix, and the guard refuses every agent
  edit, since its rules name this layout's folders. `vault.CheckLayout` names the
  migration for layout 7 (`ErrMigrate`) and the update for any other (`ErrLayout`).
- **The setting is `tagging`, not `tags`.** `Almagest.md` holds `tagging: open | known`,
  and `vault init` takes `tagging`. `tags` is Obsidian's own property.
- **A change's base ignores what code derives** (`change.BaseHash`): code-owned fields,
  the lead callout, the code sections. A sync between a proposal and its apply refreshes
  a repository's git facts, and that was a conflict on every apply. Apply
  keeps the document's current code-owned fields (`keepDerived`).
- **A session records its agent's process and conversation.** The hooks walk up from
  their own process to the nearest `claude` or `codex` and keep its id in `pid`, and keep
  `transcript_path` in `transcript`. A sync ends a live session whose process is gone,
  and the palette shows a session as open only while its process runs. Resume
  reads the conversation's first `cwd` and its config folder from the transcript, and
  finds the transcript of an older session by its id under `~/.claude*/projects/`.
- **Resume names a config folder only when it is not `~/.claude`.** Claude Code keys its
  login to the folder it was told: `CLAUDE_CONFIG_DIR=~/.claude claude` reports "Not
  logged in" on a machine logged in through the default. Verified 2026-10-01 on 2.1.286.
- **Agent preferences live in two files, and the binary merges them.** `~/.almagest/config.json`
  (`preferences`) and `<vault>/.almagest/config.json` (`almagest.vault-config.v1`); the vault
  wins per key (`vault.Merge`). Both decode strictly, so a typo is an error. The plugin
  reads and writes them only through `almagest config --json`, never the files.
  
- **A terminal launch fails where no one sees it** (osascript and `open` exit after the
  spawn), so `openTerminal` checks for the app first. `scripts/probe-launch.mjs` of `obsidian-almagest`
  opens a real terminal with a probe; record each result in TESTED.md.
- **A tag page that does not hold its tag's parent is a warning.** No document is lost
  by it; lint names the fix.
- **`journals/` is the user's.** The guard refuses every agent edit under it. The index
  keeps its notes as link targets only (`vault.Unread`, with the scratchpad and `checkout/`), so search, lint, a rename's link rewrite, and a retag skip them. The
  index skips `wiki-view/` and `tool/trash/` entirely.
- **Packages:** the design's context package is `internal/brief`, since `context` is a
  standard Go package. `internal/derive` writes the code-owned parts of sources,
  repositories, and topics (lead callouts, the `almagest-repo` block, git facts).
- **Recovery needs the paths.** A change document gets a code-owned `paths` field while
  its apply is in flight: every path the apply may write. The field, not the status, is
  the mark: the file says `applied` for the derive step and keeps `paths` until the
  commit lands, and the commit records the final document, without `paths`, from the
  index (`Tx.Stage`). The derive step adds each path to `paths` before it writes it
  (`inFlight`). When the `Almagest-Change` commit exists, recovery takes the document from
  that commit, after it commits an edit made to the document since. Otherwise it records
  its base revision (HEAD as the crash left it) in a `recovering` field before anything
  else, commits the listed paths that differ from `HEAD` as `recovery: N files as found
  after a crash` (`CommitOnly`: those paths alone), puts the listed local documents
  (`Vault.Local`: a `.md` path that `Vault.Contain` accepts, outside `.git`, `.obsidian`,
  and `.claude` in any case) back from the base, and sets the
  change to proposed without `paths` or `recovering`. So a recovery that a crash stops
  partway does the same again.
  A `recovering` value that is not the full id of a commit in the history (or `none` in a
  repository with no commit) stops recovery before it changes anything.
  Both ways it unstages what it restored and the change document. Frontmatter comes from
  a pull or a shell too, and recovery runs at every session start.
- **A write that ends without its commit is put back.** `vault.Tx` keeps each path's
  bytes, or a link's target, before its first touch (`Keep`), and refuses a path it cannot
  keep. Every write defers `tx.End(&err)`: a failure before the commit rolls back and the
  error says so, like a failed `Commit`: "the vault is back as it was", or the paths it
  could not put back. A path saved since the write wrote it (Obsidian, an agent's Edit)
  stays as saved, and the message names it; `Tx.Settle` records what a git checkout (undo) left at its kept paths, so their later saves count too. Rollback
  unstages once the commit staged the paths, or once `Tx.Indexed` said a git checkout did
  (undo). 
- **`writeAtomic` syncs** the file before the rename and the folder after it. On macOS
  that is `F_FULLFSYNC`, about 0.2 s for a write. `writeAtomicIf` runs a check
  after the sync, right before the rename.
- **A derived write keeps a newer save.** Every sync (derive and git facts) writes
  through a `vault.Guard`: `NewGuard(idx, v or tx)`, whose `Write` calls
  `WriteIfUnchanged` with the bytes the index read. It compares before the write and again right
  before the rename, skips a file saved in between, and records it in `Skipped`; the next
  sync derives it. `SyncSettings` merges again when the harness wrote the file meanwhile.
- **The views sync never deletes a user's note.** A `.md` file in `wiki-view/` that no
  view stands for and that lacks `views.Notice` moves to `ingest/` under a free name, and
  every write says so: `moved_from_wiki_view` in the result of the source and change
  tools and in the JSON of each CLI write (with a line on stderr), `strays` in a sync, and a notice in the plugin.
- **Every path built from input is contained.** `Vault.Contain` refuses an absolute or
  unclean path, `..`, `.git` in any case, a name over 255 bytes, and a path whose folders
  or final link resolve outside the vault. `Vault.Write`, `WriteIfChanged`, and `Remove`
  run it by default, so a caller cannot forget it; `vault.Tx` checks before it marks a
  path, and the moves of the misplaced-file sync check both paths.
  The one unchecked writer, `WriteMachineIfChanged`, takes only fixed paths under
  `.obsidian/` and `.claude/`, so a user who links `.obsidian` to a shared folder keeps a
  working vault; `settings.go` and `prefs.go` write their machine files with
  `writeAtomic` directly. A file named for capture passes `Vault.IngestFile` (a regular
  file kept in `ingest/`), and a repository's files and a source's captured file are
  read through `os.Root`. Go 1.24's `os.Root` has no rename, so writes check the path
  before the atomic rename instead.
- **Each MCP tool handler recovers a panic** (`safe`), so one bad call returns an error
  and the server keeps serving.
- **The change tool keeps the gate, not the guard.** `change.Apply` takes a `Gate`; the
  MCP server passes `sessions.UserAnswered`, which reads the change's `session` and that
  session's `last_prompt`. It judges the document Apply resolved, under the lock, so no
  other name for the change and no other working folder gets past it. The CLI passes
  none: the terminal and Approve in the change document are the user's, since Approve
  runs `change apply` as the user. A change with no writes, which only absorbs sources,
  needs no answer and applies at once. Any other change with no `session` waits for
  Obsidian or the terminal; `change propose` from the CLI records no session.
- **A change from a session that ended gets its own refusal.** When the proposing
  session is `ended` or `lost` and had no prompt after the proposal, `UserAnswered` says
  so: the user presses Approve in the change document, or the agent proposes again with
  `supersedes`. A live session without a prompt gets the plain "wait for the yes".
- **The change widget holds no data.** Every change document that code writes holds an
  empty `almagest-change` block right after its lead callout (`change.Widget`). The plugin
  draws it from the note's frontmatter: the kind, the last progress line, and Cancel
  while `running`; Approve and Cancel while `proposed`; a line for `applying`, and the result for `applied`, `rejected`, `superseded`, and `undone`.
  Approve saves the open note, then runs `change apply`; Cancel runs `change reject`,
  and an empty reason becomes "cancelled in Obsidian". The body stays the record.
- **A work document is a change that starts before its writes.** `change start` (MCP
  action `start`) writes a change with status `running`, a `kind` (`ingest`, `repair`,
  or `draft`), `files` (the names in `ingest/`, each checked by `Vault.IngestFile`), and
  the sections Files, Progress, Notes, Absorbed, and Writes. It commits nothing, and the
  touched hook links it to the session as it does a proposal. `change progress` adds
  `- HH:MM text` under `## Progress`, cut to 200 characters (`MaxProgress`). `propose`
  with `id` (`Plan.ID`) writes the plan into that document: it keeps `created`, `kind`,
  `files`, `session`, `## Files`, and `## Progress`, and sets `proposed`. Progress and a
  proposal into a change that is not running refuse (`running`); a rejected one says
  "the user cancelled <title> (<reason>); stop the work", which the skills obey at once.
  `reject` takes a running change too: the widget's Cancel. Apply still takes only a
  proposed change.
- **Every proposed change has a `## Summary`**, which code writes below the widget from
  the plan: one line per write, with the write's optional `why` (`Write.Why`, cut to
  160 characters). It links only the documents the change leaves in place: a removed
  title and the old title of a rename are plain text. Lint skips dead links in a change
  that is neither `proposed` nor `running`, since such a record may name a document a
  later change removed. Every change document carries `cssclasses: [almagest-change]`
  (`change.CSSClass`), and the lead callout comes from the frontmatter (`leadFor`).
- **A remove moves the document to `tool/trash/`.** Apply picks the place with
  `vault.TrashPath` (`tool/trash/<date>/<vault path>`, with " (2)" before the extension when
  the place is taken) and lists it in `paths`, so recovery and undo cover it: undo puts
  the document back and takes the trash copy away. The preview's write carries the
  place in `trash`.
- **Safe delete is the user's** (`core.Trash`, `vault trash PATH`, CLI only; the plugin
  runs it). `Index.Backlinks` counts every typed document, note, and misplaced document,
  and the markdown kept as link targets only (the scratchpad, `journals/`, `checkout/`), by frontmatter links and body links. The records (changes and
  sessions) do not count: they name what they touched and keep the name after a delete. With backlinks it moves nothing and exits 2 (`exitError`), after it prints
  them (JSON `{"trash": {"path", "backlinks", "moved"}}`). A knowledge document with
  none leaves through a change ("Delete <title>", one remove) that it applies at once
  with no gate, so the record and undo are a change's; an apply that fails rejects the
  change, so no proposal of it stays. Any other file moves in a commit `trash: <path>`.
  A path in another case meets the rules of the file on disk (`Vault.Spelled`). The
  palette offers "Resolve with an agent" only for a knowledge document that knowledge
  documents link; a link in any other file is the user's to fix, and the agent's
  message then says to propose no remove. It refuses `Almagest.md`, `.obsidian/`, `.claude/`, `.almagest/`,
  `changes/`, `tool/sessions/`, `wiki-view/`, `tool/trash/`, a shipped Base, a folder, and a path
  outside the vault. The index skips `tool/trash/`, and the guard refuses agent edits in it.
- **A journal volume is a folder directly under `journals/`** (`journal.Volumes`). Its
  notes are every `.md` file under it, in path order, except its history note,
  `Journal · <folder>.md`; dot files, dot folders, and symbolic links are skipped. `journal.Name`
  turns the folder name into the volume's name: `-` and `_` become spaces, a word of
  letters and digits is upper case, any other word starts with a capital (`cs566-notes`
  → `CS566 Notes`).
- **An edition is one markdown text.** `# <title>`, then for each note `## <its path in
  the volume, without .md>` and its body, frontmatter dropped. `journal_hash` is the
  sha256 of the notes alone (each path and trimmed body), so neither the date, the
  title, nor a note's frontmatter changes it. Publish refuses a volume that does not
  exist, holds no note, or has the hash of its latest edition ("<volume> has no change
  since <title>"). The latest edition is the source with that `volume` and the latest
  `captured`.
- **Publish captures through `source.Capture`.** `Request.Journal` (`json:"-"`, so no
  tool call sets it) makes the text an edition: `origin: journal`, `authority: primary`,
  `locator: journals/<volume>`, and the code fields `volume`, `edition` (the date), and
  `journal_hash`. The title is `User Journal <Name> - <D Month YYYY> Edition`; capture
  numbers a second edition of one day (" (2)"). The tags of the latest edition carry
  forward, with `NewTags` set. Publish refuses a folder name that is not clean (`.`,
  `..`, a slash). One commit holds the source, its original in
  `tool/source-core/originals/`, and the publication history. The lead callout says
  "Captured <date> from the journal `<volume>`".
- **Code's notes take reserved titles.** A note that code writes beside the user's files
  takes a title with a prefix of `vault.ReservedPrefixes` (`Tag · `, `View · `,
  `Checkout · `, `Journal · `), which no document takes (proposals, captures, and lint
  refuse it). So a link to the note names one file: the ledger is
  `checkout/Checkout · Ledger.md`, a reading list `Checkout · <folder name>.md`, and a
  volume's history `Journal · <folder>.md`.
- **The publication history is code's.** `journal.HistoryNote` opens with its own
  notice (`HistoryNotice`, not `views.Notice`), then a heading, one line on how editions
  reach the wiki, and an inline Base of the sources with that `volume` (the folder name
  quoted for the Base's filter; file.name,
  edition, measure, status). Publish writes it in its commit. A full `vault sync` (not
  the views-only sync) writes it for a volume that has an edition and lacks the note
  (`WriteMissingHistories`), and commits nothing; the next snapshot keeps it.
- **Status names the volumes to publish.** `vault --json` adds `journals`: per volume
  `volume`, `name`, `notes`, `edition` (the latest edition's title, or ""), and
  `changed` (the hash differs from the latest edition's, or the volume has notes and no
  edition). Home lists each changed volume under what waits, and the opening context
  adds "Journals to publish: <volumes>". `journal list` prints the same.
- **A copy takes the name `<Title> (checkout)`.** A copy with its original's name would
  make `[[Title]]` name two files. `Index.Linked` returns nil for a target with two
  paths, so every frontmatter link to the original would stop resolving, and Obsidian
  could open the copy from a link in the wiki. The index keeps `checkout/` as link
  targets only (`vault.Unread`), so search, lint, a rename's link rewrite, and a retag
  skip it. `Index.Backlinks` counts its links, so a copy's `checkout_of` keeps safe
  delete from moving the original.
- **Code ranks the candidates; the librarian chooses.** `checkout.Candidates` runs the
  search (BM25F, with the request's tags and types; topics and repositories by default),
  takes the 12 best hits as seeds at distance 0, and walks links out and in, frontmatter
  and body (`neighbors`), to distance 2 (`MaxDistance`). A document it reaches scores
  the larger of its own search score and 0.6 (`decay`) × the score of the document it
  was reached from, which `via` names. The list is sorted by score and cut to `limit`
  (default 40). Code does not judge relevance: the skill reads the descriptions, reads
  a document when its description does not settle it, and stops a branch where its
  documents stop serving. `make` takes at most 60 documents (`MaxDocuments`); the skill
  aims for about 30.
- **A copy records its original's state.** Its frontmatter holds `checkout_of` (a link to
  the original), `checkout_id`, `checkout_base` (`change.BaseHash` of the original, the
  base a change write takes), `checkout_hash` (`doc.ContentHash` of the copy's body as
  written), `checked_out`, and `description`. The body is a code callout, then the
  original's body without its lead. A copy is edited when the hash of its body, callout
  removed, differs from `checkout_hash`; a reflow is not an edit (`ContentHash` folds
  whitespace).
- **Links follow the copies, and go back on return.** `toCopies` points each body link
  to a document of the checkout, by title or alias, at its copy
  (`[[checkout/<folder>/<Title> (checkout)|<text>]]`, the old text kept as the alias).
  Every other link keeps naming the wiki, and so does a link that holds a path.
  `toWiki` reverses it on return: a link whose text is the original's title goes back
  bare (`[[Title]]`); any other text stays as the alias. Frontmatter links are not
  rewritten.
- **Return proposes, and skips what changed.** `checkout.Return` makes one `modify`
  write per edited copy: the copy's body with `toWiki` applied, `base` =
  `checkout_base`, why "edited in the checkout <name>". It leaves out a copy whose
  original is gone or whose `BaseHash` differs from `checkout_base`, and names it in
  `skipped`. It proposes one change, "Return <folder>", with no `session`, so only the
  user applies it (Approve, or `change apply` in a terminal). It sets `returned` in the
  reading list and writes the ledger in a commit of its own. When that commit fails
  after the change is proposed, Return returns the change with a `warning`, not an
  error, so the user sees the change; the palette shows the warning. A checkout is returned
  while its return change (`return_change`) is proposed or applied, and a second return
  is refused then; a rejected, superseded, or undone return change frees it. The
  change's title is "Return <name>" (the folder without its date). With no edited copy it
  changes nothing and returns an error that says so. Return carries the body only; an
  edit of a copy's frontmatter does not return.
- **The reading list and the ledger are code's.** `make` writes the reading list,
  `Checkout · <folder name>.md` (`request`, `checked_out`, `documents`, `returned`; a
  callout, `## Request`, `## Reading order`, `## Notes`), and the ledger,
  `checkout/Checkout · Ledger.md`, a table of every checkout,
  newest first, written again at each make and return from the reading lists
  (`checkout.List`). The guard refuses agent edits under `checkout/`; a read-only agent
  may call `checkout` `candidates` and `list`.
- **Status names the checkouts.** `vault --json` adds `checkouts`, the entries of
  `checkout list`: per checkout `folder`, `request`, `date`, `documents`, `edited` (the
  count of edited copies), and `returned`.
- **A wikified copy lies in `scratchpad/`, not beside its note.** `wikify.Start` writes
  `scratchpad/<name> · wikified.md` (`wikify.Suffix`), and `<name> · wikified (2).md`
  when that name is taken. The strategy note put the copy beside the note, but a copy
  of a journal note there would join the volume's next edition, since an edition holds
  every `.md` file under the volume. Start never changes the original. It refuses a
  file that is not markdown, `Almagest.md`, and a note under `tool/source-core/`, `changes/`,
  `tool/sessions/`, `wiki-view/`, `tool/trash/`, or `.obsidian/`; a note in `journals/` passes.
- **A mark is inline text**: `{{link:<Title>|<phrase>}}` where the phrase names a
  document, `{{new:<Title>|<phrase>}}` where it names a subject worth a topic
  (`wikify.Text`). The plugin parses this syntax; change it in both places.
- **A mark lands on the first free mention.** `wikify.Place` takes the marks in the
  order given and replaces, for each, the first occurrence of its phrase as whole words
  and without regard to case, and keeps the note's own spelling. A mention is not free
  in the frontmatter, a heading, a table row (a mark's | would split a cell), a code span or fence, a comment, a wikilink or markdown
  link, a URL, or a mark (`find`), so a later phrase cannot land inside an earlier mark.
  A phrase is one line with no `|` or braces. Place writes the note once and returns
  `placed` and `missing`. A bad mark refuses the whole call before it writes.
- **A new title the wiki holds becomes a link.** `resolve` gives a `link` the title of
  the document it names (by id, title, or alias) and refuses a link that names none. A
  `new` title passes `doc.CleanTitle` and `doc.CheckTitle`; when a document holds it,
  the mark is a link to that document.
- **`wikify mark` takes only a wikified copy**: a `.md` note directly in `scratchpad/`
  whose name holds ` · wikified`. It refuses any other note, so the tool cannot write a
  knowledge document, a journal note, or the original. The guard sees the tool (the
  PreToolUse matcher names `wikify`), and a read-only agent may call neither action.
  The guard refuses an agent's edit of a wikified copy (a name with ` · wikified` in
  `scratchpad/`), so only `wikify mark` writes its marks.
- **Wikify commits nothing.** The copy is the user's scratch: the next quiet snapshot
  keeps it, as it keeps any hand edit, and the plugin's Accept and Ignore edit the copy
  as the user does. `Start` and `Place` hold the vault lock while they read and write.
- **A link mark can go stale.** Its document may be deleted or renamed after the mark.
  The plugin resolves the title as Obsidian does (`getFirstLinkpathDest`); a title that
  names no note gives the bubble the state `gone`, with Ignore only, and the bulk Accept
  leaves such marks and counts them in its notice.
- **A work document may be a `draft`.** `change start --kind draft` starts the document
  that wiki-edit fills with the one topic that Create asked for; its default title is
  "Draft a topic". The schema's change `kind` and the search's kinds list `draft`.
- **Status shows the work and the trash.** `vault --json` adds `changes.running` (the
  running work documents) and `trash` (the count of files under `tool/trash/`, `.DS_Store`
  aside). Home lists each running work document with what waits.
- **The plugin commits hand edits as quiet snapshots.** After `snapshotQuietSeconds`
  (default 120; 0 turns it off) with no create, modify, delete, or rename outside the
  config folder and `wiki-view/`, it runs `vault snapshot`, which commits every hand
  edit as one snapshot (`core.Snapshot`, `vault.CommitSnapshot`). `vault snapshot` is CLI
  only, not an MCP action, and takes the lock without waiting (`LockWithin(0)`): when a
  write holds it, the command fails at once and the plugin tries again after the next
  quiet period. The plugin shows no notice for a failed snapshot. `core.Snapshot` runs
  `vault.Recover` first, so an apply that a crash stopped is put back, not committed as
  a hand edit. Every write still commits a snapshot first. The skills tell agents never to report the vault's git
  state.
- **The Obsidian plugin reads the JSON of the CLI.** `obsidian-almagest` runs the binary
  for every read and write: `vault --json`, `change apply`, `vault trash`, `journal
  publish`, `checkout return`, `wikify start`, `vault snapshot`. A change to that JSON or
  to a command's flags is a change of the plugin's contract.
- **Times carry seconds.** `proposed` and `last_prompt` are `2006-01-02T15:04:05`. With
  minutes, a yes typed in the minute of the proposal would not open the gate.
- **The gate counts only the user's turns.** A host sends a subagent's hand-back and a
  background task's notice as a UserPromptSubmit (`<agent-message …>`,
  `<task-notification …>`). The prompt hook sets `last_prompt` only for a real prompt
  (`hooks.UserTurn`). The live test found this hole.
- **The Stop hook blocks once for the agent's own debts.** An empty `## Description` in
  a session that touched a repository or proposed a change returns `decision: block` with
  the reason, once per session (`reminded`), and never while `stop_hook_active`. A
  change that waits for the user is a `systemMessage`, because the user acts on it.
- **The description follows the section.** Every hook event copies the first line of
  `## Description` into `description`, whatever tool wrote it.
- **An edit in a linked repository needs no ceremony.** A rule that asked for one blocked everyday work. The guard
  judges only files inside a vault, by the vault above the file.
- **Shell writes reach the record, not the guard.** The touched hook adds a repository to the session's `repositories` when a Bash
  command with a write mark (a redirect, `sed -i`, `git commit`, …) runs in it or names
  it. The skills tell the agent to change files with Edit and Write.
- **The shell runs neither the apply nor the undo command of `change`, `vault trash`, `journal publish`, nor `almagest hook`.** The guard
  refuses them, in every folder: four skip the gate or undo the user's act, `journal
  publish` is the user's decision (CLI only, no MCP action; the plugin's Publish runs
  it), and `hook` forges a user's turn. The `change` tool's `undo` refuses a change with
  no `session`: safe delete, Return, and a terminal's proposal are the user's own acts,
  so only the user undoes them, in a terminal.
  `journal list` passes. It is no sandbox; a shell can still write any file. To
  try one by hand from a session, type the command with `!`. `shellCommands`
  (`internal/hooks/shell.go`) reads the line as bash and zsh do: quotes, `$'…'` escapes
  (`\x{…}` too), backslashes and backslash-newlines, brace lists and ranges,
  separators, process substitutions (`<(…)` runs a command of its own), and redirects
  whose targets leave the words (`&>`, `{fd}>`, `<<<`, quoted targets); zsh's `=name`
  counts as the name. A quoted word or a heredoc body is read as a command only in a
  pipeline that runs a string (`runsString`: a runner such as sh, bash, eval, or ssh,
  in any case, where the command name stands, after assignments and wrappers like sudo,
  xargs, or timeout; `env -S`; find's `-exec sh`), as in `sh -c "…"` or `echo "…" | sh`;
  elsewhere a commit message or a grep pattern that names a refused subcommand is text,
  and a refusal of quoted text says why it counted. Brace expansion stops past 256
  words, counted before it expands. The binary's name compares without case, and
  options drop out; an option's value stays, so `apply` and `trash` count anywhere after their subcommand. A word built at run time (a variable, `$(…)`, a glob, xargs)
  is out of its reach. `config set` and `config unset` of `terminal_command` or
  `agent_commands` are refused too.
- **The guard takes the vault above the file**, not the vault of the session's folder,
  so a session outside the vault gets the same refusals. A file in no vault meets only
  the rule on the home's `config.json`.
- **The guard judges the path the disk names.** `canonical` resolves links on the part
  that exists and spells each part as its folder entry, so `ALMAGEST.md`,
  `Source-Core/documents/…`, a repository in another case, or a link into the vault meet
  the rule of the real file.
  A session document's code sections come from `doc.SectionOffsets`, which skips fenced
  headings, and an Edit, a MultiEdit, or a patch is judged by the document it leaves (`Input.leaves`,
  `codeChanged`): the frontmatter, the lead, and each code section come out as they went
  in, with each code heading as often as before by `doc.Headings`. An Edit that applies is judged by that alone, so its anchor may hold a
  code heading it keeps; its old text matches with curly and straight quotes as one, as
  the host's Edit may. A Codex patch is read as Codex 0.155.1 reads it: file markers with
  whitespace before them, context and removed lines that match with trailing whitespace
  ignored, every match counting, a hunk with neither at the end of the file; a hunk that
  matches nowhere is refused and the refusal says so. A move is judged as a delete of
  its source and a new file at its target.
- **The files that decide what runs are the user's.** The guard refuses an edit of the
  home's `config.json` and the vault's `.almagest/config.json` (`terminal_command`,
  `agent_commands`), and of anything under `.obsidian/plugins/almagest/`; fixed names
  compare without case, so a folder that does not exist yet cannot carry another case.
- **A read-only agent makes only the calls that read.** `readActions` lists them per
  tool; any other action, or a tool the list does not know, is a write.
- **The PreToolUse matcher anchors itself** (`^(…)$`) and names the almagest server of both
  hosts (`mcp__plugin_almagest_almagest__…`, Codex's `mcp__almagest__…`): hosts test it
  unanchored.
- **Every hook waits for the lock half its timeout at most** (`hooks.Deadlines`,
  `Vault.LockWithin`, which bounds the in-process mutex too), then exits 1 with the held
  lock named, before the host kills it. The plugin test holds each deadline against
  `hooks.json`; SessionEnd's timeout is 3 s.
- **The host's own quiet agents are workers.** `Explore`, `Plan`, `claude-code-guide`,
  and `statusline-setup` get a line under `## Subagents`, like the plugin's three, and no
  document (`quietAgents`). `statusline-setup` has Edit; its edits meet the path rules
  like any agent's but stay off the session's record. Only the plugin's three are refused
  writes.
- **The harness settings keep no memory.** `SyncSettings(drop)` adds every repository
  path the documents name and removes only the paths the documents named before a write
  and no longer do. Apply and undo pass their before-list.
- **Machine files stay out of git** through `.git/info/exclude` (`vault.Excluded`:
  `wiki-view/`, `.claude/settings.local.json`, `.obsidian/workspace.json`,
  `.obsidian/workspace-mobile.json`, `.obsidian/graph.json`, `.DS_Store`, and the
  temporary `.almagest-*` files), so
  init edits no file of the user's. `EnsureFolders` rewrites the entries on every write,
  and untracks an excluded file that an older vault tracked, in a commit of its own.
- **Titles also drop `[ ] # ^`**, which break a wikilink, `→`, which splits the old and
  new titles of a change heading, and control characters. A title the caller gives holds
  at most 150 bytes (`doc.CheckTitle`), so the titles code derives from it fit a file
  name; a captured file's name is cut instead, and one that cleans to nothing takes the
  source's id. " · " stays allowed: the heading parser finds the id at the end.
- **A create or a rename checks the disk** (`Vault.Occupied`), because the index compares
  titles by lower case only, and APFS also folds Unicode forms: "Café" in NFC and in NFD
  is one file. Capture numbers such a title; every other writer refuses it.
- **Match merges the hits of one kind on one document** (`mergeHits`), so one drafter
  writes that document. A near or a new subject is never merged, and neither are two
  subjects of different kinds that hit one document.
- **The launcher runs the binary of the plugin's own version.** `bin/almagest` (made by
  `make pin` from `internal/release/launcher.sh`) holds the version and the pinned
  checksums inline. It runs `$ALMAGEST_BIN` when set, else
  `$ALMAGEST_HOME/bin/<version>/almagest`; when that file is missing it downloads the
  release asset over HTTPS, installs it only when its sha256 matches, points
  `~/.almagest/bin/almagest` at it, and appends to `~/.almagest/install.log`. It never
  searches PATH, so another tool's binary never runs in its place, and drops `~/go/bin`:
  a `go install` pins no version. A hook with no binary passes (the session start says
  why); any other call fails with the reason. `ALMAGEST_RELEASE_BASE` changes the
  download's source for the tests; the checksums still decide what installs. Claude Code
  puts `bin/` on the Bash tool's PATH only, so `.mcp.json` and `hooks.json` name it by
  `${CLAUDE_PLUGIN_ROOT}`. The Obsidian plugin runs its `binaryPath` setting, else
  `~/.almagest/bin/almagest`.
- **A release builds the same bytes on every machine.** `make release` builds each
  platform with `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, and the exact toolchain
  (`TOOLCHAIN`, through `GOTOOLCHAIN`), so the sha256 of each binary is known before the
  tag: `make pin` writes them to `release/checksums.txt` and into the launcher. The
  release workflow builds again from the tag and publishes only when the bytes match.
- **One rule chooses the vault** (`vault.Select`): the vault a call names (`--vault`, or
  a tool's `vault` input), else `$ALMAGEST_VAULT` (a path or a machine-file name), else the
  vault above the working folder. The CLI, the MCP server, and the hooks call it; a bad
  `ALMAGEST_VAULT` is an error in the first two, and a hook falls back to the folder.
- **`--help` and `-h` run nothing.** `Run` answers them from the command's usage entry
  before dispatch, since `parse` would read `--help` as an option and `setup`, `vault
  init`, `hook`, and `mcp` act at once. A test (`usage_test.go`) holds every option a
  command reads to its usage entry.
- **Codex runs the launcher inline.** `.codex-plugin/plugin.json` holds the almagest
  server as `/bin/sh -c <the launcher> almagest mcp`, no `cwd`, and `env_vars` for
  `ALMAGEST_BIN`, `ALMAGEST_HOME`, `ALMAGEST_VAULT`, and `ALMAGEST_NO_DOWNLOAD`. Codex
  0.155.1 expands no placeholder in a plugin's MCP config, ignores a root `plugin.json`,
  and resolves a `cwd` against the plugin's cache folder, where the server would find no
  vault; so the launcher carries its version and checksums instead of reading a file
  beside it, and holds no `${`. `make pin` writes both copies from one template, and
  `internal/plugin` runs each through every case. Claude Code keeps reading `.mcp.json`.
- **`doctor` starts each host's server as the host runs it** (`host.Entry`,
  `host.Probe`): Codex's transport from `codex mcp list --json`, Claude Code's
  `.mcp.json` from the install path with `${CLAUDE_PLUGIN_ROOT}` expanded, with only
  `HOME`, `PATH`, `env`, and `env_vars`, and compares the tools with
  `mcpserver.ToolNames`. A `${` in the command fails; a script in the args may hold its
  own. Codex hook trust comes from `codex app-server`'s `hooks/list`
  (`host.CodexHookTrust`), since only Codex knows the trusted hash; `setup --agent
  codex` prints the same line on every run.

## Host facts, verified live on Claude Code 2.1.283 (2026-09-28)

- Hook events carry `session_id`, `cwd`, `hook_event_name`, `prompt_id`, and
  `permission_mode`. SessionStart has `source`; SessionEnd has `reason`; Stop has
  `stop_hook_active` and `last_assistant_message`.
- A subagent's SubagentStart, SubagentStop, and tool events carry `agent_id` and
  `agent_type` (`almagest:wiki-audit`), and the parent's `session_id`.
- PostToolUse gives `tool_response` for every tool. For an MCP tool it is a JSON string
  that holds the result's JSON; `hooks.decodeAll` reads both.
- `cwd` follows the agent's `cd`: after `cd repo && …`, later events carry the
  repository's folder.
- Plugin tools are `mcp__plugin_almagest_almagest__<tool>`; skills are
  `almagest:<skill>`.
- `ALMAGEST_HOOK_LOG=<file>` appends every hook event the binary receives, one JSON line
  each. Use it to check a host's events.

Obsidian 1.13.7, verified live (2026-10-01): in a Base filter, an expression on a
property a note lacks fails, and the note drops out: `!x` and `!x || x == ""` both hide a
note with no `x`. `x.isEmpty()` is true for a missing property and for `""`. A hidden window renders nothing, so a DevTools screenshot hangs; bring the
second instance to the front by its pid with `osascript` first.

Obsidian 1.13.7, verified live (2026-09-28): a hidden window pauses timers and rendering.

To drive Obsidian without touching the user's app, run a second instance with its own data
folder: copy `obsidian-<version>.asar` from `~/Library/Application Support/obsidian/` into
the folder, write `obsidian.json` there with one vault, and run
`open -n -a Obsidian --args --user-data-dir=<folder> --remote-debugging-port=9333`. Then
evaluate JavaScript and take screenshots through the DevTools protocol at
`localhost:9333/json/list`, after `Page.bringToFront`. The settings open as a second page
of that list; choose the page whose title does not begin with "Settings". Point the
plugin at the build under test: set `binaryPath` in the plugin's settings.

Verified with Codex 0.155.1 in a scratch `CODEX_HOME` (2026-10-04): the plugin's server
entry, its start in a vault, and `doctor`'s server and hook trust lines (TESTED.md).

Verified live in Obsidian (TESTED.md): Start agent, Resume from the palette, and the
settings tab.

Verified by script in a separate Obsidian: `obsidian-almagest`'s end-to-end suite builds
this binary from a checkout beside it and drives the plugin against it (its TESTED.md).

Not yet verified: Codex's hook events in a session (the guard reads `apply_patch` paths;
the rest is untested on Codex), an almagest tool called from a Codex session, the
Notification types in a live session, and, inside Obsidian, the Almagest navigator and the
repository panel.

## Constraints

- Dependencies: `gopkg.in/yaml.v3` and the MCP Go SDK pinned at v1.4.0 (later versions
  need Go 1.25). Nothing else in Go.
- The SDK validates tool output against the schema it infers: a field without
  `omitempty` is required, so an optional pointer or a union needs `omitempty`.
- Every write takes `.git/almagest.lock`. The lock is not re-entrant: a write takes it once
  (`vault.Begin`, `vault.BeginWrite`, or `v.Lock()`), and inner functions assume it held.
- A document is found by id or title, never by a path a tool was given.
- Tests never touch a real `~/.almagest`: `testvault.New` sets `ALMAGEST_HOME`. They skip
  without git.
- Prose in skills, docs, and messages follows the user's global writing guide.


## Build, test, and try

```bash
make build        # build/almagest
make install      # ~/.almagest/bin/<version>/almagest and the link, as the launcher installs
make test
make release      # build/release: each platform's binary and checksums.txt
make pin          # make release, then release/checksums.txt and the launcher
```

The binary, both plugin manifests, the marketplace entry and its `ref`, and the
launcher share one version; the `internal/plugin` tests fail when they drift. `make test`
runs them; `make build` and `make install` do not.

End to end in a scratch vault, without touching the real machine folder:

```bash
export ALMAGEST_HOME=/tmp/almagest-home ALMAGEST_BIN=$PWD/build/almagest ALMAGEST_HOOK_LOG=/tmp/hooks.log
almagest vault init --path /tmp/work --name Work
cd /tmp/work && claude -p "…" --plugin-dir /path/to/almagest \
  --allowedTools "mcp__plugin_almagest_almagest__*,Read,Grep,Glob,Skill,Edit,Agent"
claude -p --continue "yes" --plugin-dir …     # the user's answer at a gate
```

## Release

Work happens on `preview`. `main` takes changes only through a pull request from
`preview` whose checks pass (a ruleset on GitHub), so `main` is always a release or a
change that leaves the released binary as it is. The marketplace entry pins the plugin to
its release tag (`source: github`, `ref`); Codex's local entry takes `main`, which is safe
for that reason.

To release X.Y.Z:

1. On `preview`, `make version V=X.Y.Z`. It sets X.Y.Z in both plugin manifests and the
   marketplace entry (its `version` and `ref`), builds the binaries, and pins their
   checksums in `release/checksums.txt` and the launcher. Run `make test`, commit, push.
2. Open a pull request into `main`. CI runs the tests on Linux and macOS, and `checksums`
   builds the binaries again and requires the pinned bytes.
3. Merge it. The release workflow builds again, checks the bytes, attests the binaries,
   tags the commit X.Y.Z, and publishes the release. Nothing else to tag or push.

A pull request without a new version merges when its binary builds the bytes already
pinned (docs, skills, tests); the release workflow then publishes nothing. A change to the
binary without `make version` fails `checksums`. For the minute between the merge and the
release, the marketplace names a tag that does not exist yet, and an update then fails
and succeeds on the next try.

A session started inside this checkout may report that the project MCP server
`${CLAUDE_PLUGIN_ROOT}/bin/almagest` failed to start. Claude Code reads the
checkout's own `.mcp.json` as a project server, and that variable is set only for plugins.
It starts that server only when the checkout's untracked `.claude/settings.local.json`
enables it (`enabledMcpjsonServers` or `enableAllProjectMcpServers`). The plugin's own
server still runs; remove the project server from that file to silence the error.

## Not built yet

- Signing and notarizing the macOS binaries, and a Homebrew tap.
- Fetching a page from a URL. `source capture --text FILE --locator URL` already records
  `origin: url`; nothing fetches the page.
- The "Later" items of the Obsidian plugin page.
