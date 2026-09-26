---
type: concept
title: "V2 Obsidian Plugin Brainstorm"
status: seed
created: 2026-09-26
updated: 2026-09-26
tags:
  - concept
  - v2
  - obsidian-plugin
  - brainstorm
assessment: provisional
provenance: synthetic
---

# V2 Obsidian Plugin Brainstorm

> [!note] Provenance
> A design conversation on 2026-09-26 produced this page. The statements about Obsidian and Claude Code behavior are assessments from that conversation. Nobody tested them yet. Treat each one as provisional until a test confirms it.

## Definition

A list of candidate features for an Obsidian plugin in the next generation of atlas-obsidian, with an estimate of the effort and the risk of each. The goal is to lean more into the Obsidian side and keep the plugin reasonable and simple.

Effort scale, for one developer who knows the Obsidian plugin API: **S** is 1 to 2 days, **M** is 3 to 5 days, **L** is 1 to 2 weeks. The first plugin also costs about 1 to 2 days of setup: the TypeScript scaffold, the build, a settings tab, and the bridge to the binary.

## The approach: a thin plugin

The plugin is a fourth layer over the [[atlas-obsidian CLI]], like the terminal view. It calls the binary through `child_process`, watches files, and draws UI. It holds no logic, so Go stays the one place that decides things.

Costs of this approach:

- **Desktop only.** `child_process` and reads of `~/.atlas-obsidian` need Node. The manifest sets `isDesktopOnly: true`.
- **Finding the binary.** Obsidian started from the macOS Dock does not get the shell's PATH. The plugin needs the same search as `scripts/atlas` (`~/go/bin`, the Homebrew prefixes) and a setting to override it.
- **A third install to keep in version step.** `project.Init` already writes `.obsidian/`, so it could install the plugin into each vault. The user must still turn off restricted mode once.
- **Distribution.** Start with BRAT or a local install. The community store reviews plugins that spawn processes more closely, and review takes weeks.

## The features

| Feature | Possible? | Effort | Needs the plugin? |
|---|---|---|---|
| Refresh button | Yes | S | Yes, but it is small |
| @mention highlighting | Yes | S | Yes |
| @mention pickup by the agent | Yes | M | No, it is binary work |
| Agent status pane | Yes | M backend + S to M pane | The pane does; the data does not |
| Graph presets | Partly | S to M, with fragile internals | Not if presets are generated files |
| Live canvas editing with the agent | Mostly already works | M to L to do well | Only for the race |

### Live canvas edits

The request: an agent updates a canvas, the user sees the change at once, and the user can change a card color or add text that the agent then reads.

- Obsidian watches the vault. An open canvas probably reloads when an agent writes the `.canvas` file on disk, and the user's edits go to the same file. Both directions may already work without a plugin. Test this first with the canvas skill.
- The weak point is a race. Obsidian waits a short time before it saves a canvas, so an agent write can land while a user edit is still unsaved, and one of the two is lost.
- A fix needs a plugin that applies the agent's changes to the live canvas node by node, through `view.canvas`. That API is undocumented, its only types come from community projects, and any Obsidian update can break it. This is the most fragile feature on the list.
- Recommendation: use the file alone first. Build the plugin part only if the race matters in practice.

### Graph presets

The request: one-click graphs such as "the threads graph", "the wiki graph", and "the repositories graph", through plugin buttons or as files.

- The core graph view has no saved presets. Its settings live in one global `.obsidian/graph.json`.
- A plugin can open a graph leaf and set its search filter and color groups, but only through undocumented view state. Small work that breaks without warning.
- Alternative without a plugin: the binary generates each preset as a file, such as `threads.canvas`, `wiki.canvas`, and `repositories.canvas`. This fits the rule that code owns what code can derive. Plugin buttons can open those files later.
- Open question: what nodes and edges make up the repositories graph.
- Related thread: colored graph groups by default for hubs with members (thr-20260924-f87e).

### Agent status pane

The request: a pane that shows the Claude Code agents at work on a thread or a feature. This answers the V1 feedback "what are my agents doing right now".

- **Rejected: agents that register and announce themselves.** That is advice to a model, and a model forgets advice. A fact belongs in code.
- **Rejected: a direct connection to the harness.** Claude Code has no public API for live sessions. Its transcript JSONL files have an internal format that can change.
- **Recommended: hooks.** The plugin already ships them. `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `Notification`, `Stop`, and `SessionEnd` each receive the session id and the working directory. The binary writes `~/.atlas-obsidian/state/sessions/<id>.json` with the project, a state (working, waiting for input, idle, ended), and the time of the last update.
  - The thread comes from `thread` tool calls and the `touched` hook, which know which thread they changed.
  - A crashed session leaves a stale file. Check that the process is alive, or expire the file after a time limit.
- The pane watches that folder. Under the three-layers rule, the terminal view shows the same data.
- Codex hooks differ from Claude Code hooks. Plan for both from the start.

### @mentions

The request: the user writes `@atlas add these to the wiki` in a note. Obsidian highlights every mention. The next atlas session lists the open mentions, and the user picks which ones to do. Preset agents come from the atlas config, with `@atlas` as the default.

- **Highlighting:** a CodeMirror 6 decoration for the editor and a markdown post-processor for reading view. Small. Obsidian styles and indexes tags such as `#atlas` with no plugin, if `#` is acceptable in place of `@`.
- **Pickup:** the binary scans the vault, and the session-start hook lists what it finds. No plugin needed.
- **Closing a mention** needs a design decision:
  - Rewriting the user's note conflicts with the rule that the folder is the user's.
  - A mention inside `wiki/` can change only through `plan` and `apply`.
  - One option: a mention is a task line, `- [ ] @atlas add these to the wiki`. It is open while unchecked, and the agent checks the box when done. Obsidian already renders and queries task lines.
- A mention is close to a note waiting in the inbox, which the thread-stub skill handles. Decide whether a mention becomes a thread or stays its own concept.

### Refresh button

The request: a button or event in Obsidian that brings every status up to date.

- A ribbon icon and a command run `atlas-obsidian refresh` and `sync`, then show the result in a notice.
- No agent is needed. Refresh is code.
- Once the plugin watches files, it could run the thread sync on change without a button. Guard against a loop where the sync's own writes start another sync.

## Suggested order

1. The plugin scaffold, the refresh button, and mention highlighting. These are small and test the bridge between the plugin and the binary.
2. Agent status: the hook data in Go, then the pane and the terminal view.
3. Graph presets as generated files.
4. Live canvas merging, only if the test with files alone shows that the race matters.

## Open questions

- Does each project stay its own vault in V2? The V2 idea of one consolidated knowledge base suggests one vault. The answer changes where the plugin is installed and what a mention scan covers.
- Are mentions task lines, or free text anywhere?

## Related

- [[atlas-obsidian CLI]]: the binary the plugin calls.

## Sources

- A design conversation in a Claude Code session on 2026-09-26 (synthetic; not independent evidence).
