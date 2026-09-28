
# Skills

A skill is a procedure and a policy: what to read, what counts as enough evidence, how to cite, when to stop, which skill comes next. A skill is advice. A rule that must hold lives in a tool's refusal or in a hook ([[Tools]], [[Hooks]]).

## Names

Skills are named `<noun>-<verb>` on four nouns: `atlas`, `repo`, `wiki`, `thread`. One skill is a home: `atlas`, which every request passes first. It orients, shows the board, and routes every request. A new capability finds its noun first.

No skill shares a name with a tool. V1 learned that a skill and a tool with one name read as a pair, and they are not: `change` serves every wiki skill, and `thread` serves every thread skill.

## The map

| Skill | Owns | Tools | Agents | Writes through |
|---|---|---|---|---|
| [[atlas]] | orienting in the vault; the board and the quick moves on a thread; routing any request | `vault`, `search`, `context`, `thread` | — | `thread` set, reopen |
| [[atlas-onboard]] | a new vault: its name, its area setting, its first repositories | `vault` init | — | `vault` init, then [[repo-link]] |
| [[repo-link]] | linking a repository and choosing its area | `context`, `search`, `change` | — | `change` |
| [[repo-unlink]] | unlinking a repository, and what depended on it | `context`, `search`, `thread`, `change` | — | `change`, `thread` |
| [[repo-ingest]] | describing a repository in the wiki, and bringing the description up to date | `context`, `source`, `match`, `change` | [[wiki-extract]], [[wiki-draft]] | `source` capture, `change` |
| [[wiki-ingest]] | triaging the inbox and capturing sources | `vault`, `source` capture, `thread` open | — | `source`, `thread`, then [[wiki-sync]] |
| [[wiki-sync]] | absorbing documents into the wiki: the pipeline | `vault`, `source`, `match`, `change` | [[wiki-extract]], [[wiki-draft]] | `change` |
| [[wiki-save]] | keeping something from the conversation | `source` capture | — | `source`, then [[wiki-sync]] |
| [[wiki-query]] | answering from the wiki | `search`, `context` | — | nothing |
| [[wiki-edit]] | changing pages that exist: rewrite, rename, merge, split, repair | `search`, `lint`, `change` | — | `change` |
| [[wiki-rollup]] | mapping a parent scope from its children: bridges and upgrades | `context`, `search`, `match`, `change` | [[wiki-draft]] | `change` |
| [[wiki-review]] | the health of the wiki, quick or deep | `lint`, `vault` | [[wiki-audit]] | nothing |
| [[thread-work]] | taking a request or a thread to its next stage and on | `search`, `context`, `thread` | — | the stage skills |
| [[thread-stub]] | opening a thread, in the user's words | `search`, `thread` open | — | `thread` |
| [[thread-spec]] | what done means | `thread`, `context`, `search` | — | `thread` file spec |
| [[thread-plan]] | splitting the spec into tasks | `thread`, `context` | — | `thread` tasks |
| [[thread-run]] | doing one task in its repository | `thread`, `context` | — | `thread` task; the repository's own tools |
| [[thread-receipt]] | verifying and closing a thread | `thread` | [[thread-review]] | `thread` file receipt, then [[wiki-sync]] |

No two skills own one verb. Every write to the wiki goes through `change`, and every write to a thread through `thread`.

## The workflow

```text
                 atlas  (orients, shows the board, routes every request)
   ┌──────────────┬──────────────┬───────────────┬──────────────────┐
   ▼ a question   ▼ work          ▼ a source      ▼ a repository     ▼ the wiki's health
 wiki-query     thread-work     wiki-ingest     repo-link          wiki-review ─▶ wiki-edit
   │ worth        │ stub          │ capture       repo-ingest        wiki-rollup
   ▼ keeping      │ spec ─ gate   ▼               repo-unlink
 wiki-save        │ tasks ─ gate wiki-sync ◀──────── pending: sources, specs, receipts
                  │ task … task      ▲
                  ▼ receipt ─────────┘
```

See [[AgentFlow/Agent Session.canvas|Agent Session]] for the same flow as a canvas, with every tool and hook.

## The form of a skill

Every skill follows one form, so a reader finds the same thing in the same place:

1. Frontmatter: `name`, and `description`: what it does in one sentence, then `Use for` and the words that trigger it. When a nearby skill owns a nearby verb, the description says which.
2. A heading and one paragraph: what the skill does and why.
3. A line `Tools:` with the tools and agents it uses, and the references it reads.
4. The procedure, as numbered steps.
5. The gate: the preview the user sees and the yes the skill waits for, where it writes.
6. `Hand off`: the skills that come next.

The design pages here follow the same order.

## References

References live with the skills and load only when a skill names them.

| Reference | Holds | Read by |
|---|---|---|
| `references/changes.md` | the change contract: the plan, the preview, the yes, conflicts | every skill that writes the wiki |
| `references/pages.md` | the page schemas and the citation rules, from [[Wiki]] | every skill that writes a page, and [[wiki-draft]] |
| `references/threads.md` | the thread documents and rules, from [[Thread Documents]] | every thread skill |
| `references/conventions.md` | the conventions step ([[Conventions Check.canvas|Conventions Check]]) | [[thread-spec]], [[thread-plan]] |
| `references/syntax.md` | Obsidian Flavored Markdown: properties, wikilinks, embeds, callouts | every skill that writes a document |

## Testing the skills

Two layers, as in V1:

- A plugin test: every skill folder has its name, a description with triggers, and a `Tools:` line; every link resolves; every skill, agent, and tool that a skill, hook, or doc names exists.
- End-to-end runs in a scratch vault with scratch repositories: each skill from a plain request to its end, and a set of plain requests, each routed to the intended skill by [[atlas]].
