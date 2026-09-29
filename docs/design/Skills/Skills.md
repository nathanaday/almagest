# Skills

A skill is a procedure and a policy: what to read, what counts as enough evidence, how to cite, when to stop, which skill comes next. A skill is advice. A rule that must hold lives in a tool's refusal or in a hook ([[Tools]], [[Hooks]]).

## Names

Skills are named `<noun>-<verb>` on four nouns: `atlas`, `repo`, `wiki`, `spec`. One skill is a home: `atlas`, which every request passes first. It orients, shows the board, and routes every request. A new capability finds its noun first.

No skill shares a name with a tool. A skill and a tool with one name read as a pair, and they are not: `change` serves every wiki skill, and `work` serves every spec skill and [[wiki-stub]].

## The map

| Skill | Owns | Tools | Agents | Writes through |
|---|---|---|---|---|
| [[atlas]] | orienting in the vault; the board and the quick moves on work; routing any request | `vault`, `search`, `context`, `work` | — | `work` set, block, unblock, drop, reopen |
| [[atlas-onboard]] | a new vault: its name, its tag setting, its first repositories | `vault` init | — | `vault` init, then [[repo-link]] |
| [[repo-link]] | linking a repository and choosing its tags | `context`, `search`, `change` | — | `change` |
| [[repo-unlink]] | unlinking a repository, and what depended on it | `context`, `search`, `work`, `change` | — | `change`, `work` |
| [[repo-ingest]] | describing a repository in the wiki, and bringing the description up to date | `context`, `source`, `match`, `change` | [[wiki-extract]], [[wiki-draft]] | `source` capture, `change` |
| [[wiki-ingest]] | triaging the inbox and capturing sources | `vault`, `source` capture, `work` stub | — | `source`, `work`, then [[wiki-sync]] |
| [[wiki-sync]] | absorbing documents into the wiki: the pipeline | `vault`, `source`, `match`, `change` | [[wiki-extract]], [[wiki-draft]] | `change` |
| [[wiki-save]] | keeping something from the conversation | `source` capture | — | `source`, then [[wiki-sync]] |
| [[wiki-query]] | answering from the wiki | `search`, `context` | — | nothing |
| [[wiki-edit]] | changing knowledge that exists: rewrite, rename, merge, split, retag, promote a stub to a topic, confirm, repair | `search`, `lint`, `change` | — | `change` |
| [[wiki-map]] | mapping a tag: its overview, bridges across its child tags, topics tagged too narrowly | `context`, `search`, `match`, `change` | [[wiki-draft]] | `change` |
| [[wiki-review]] | the health of the wiki, quick or deep | `lint`, `vault` | [[wiki-audit]] | nothing |
| [[wiki-stub]] | planting a stub, in the user's words | `search`, `work` stub | — | `work` |
| [[spec-work]] | taking a request or a plan to its next step and on | `search`, `context`, `work` | — | the spec skills |
| [[spec-write]] | what done means: a stub or a request becomes a plan or a design | `work`, `context`, `search` | — | `work` promote, spec |
| [[spec-split]] | splitting a plan into parts | `work`, `context` | — | `work` spec |
| [[spec-run]] | doing one leaf plan in its repository | `work`, `context` | — | `work` start, block; the repository's own tools |
| [[spec-close]] | verifying and closing a plan | `work` | [[spec-review]] | `work` done or drop, then [[wiki-sync]] |

No two skills own one verb. Every write to knowledge goes through `change`, and every write to a stub, a spec, or an event through `work`.

## The workflow

```text
                 atlas  (orients, shows the board, routes every request)
   ┌──────────────┬──────────────┬───────────────┬──────────────────┐
   ▼ a question   ▼ work          ▼ a source      ▼ a repository     ▼ the wiki's health
 wiki-query     spec-work       wiki-ingest     repo-link          wiki-review ─▶ wiki-edit
   │ worth        │ stub          │ capture       repo-ingest        wiki-map
   ▼ keeping      │ write ─ gate  ▼               repo-unlink
 wiki-save        │ split ─ gate wiki-sync ◀──────── pending: sources, specs, completed events
                  │ run … run        ▲
                  ▼ close ───────────┘
```

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
| `references/changes.md` | the change contract: the plan, the operations, the preview, the yes, conflicts | every skill that writes knowledge |
| `references/pages.md` | the six document types, their fields and sections, the tags and tag pages, and the citation rules, from [[Documents]] and the type pages | every skill that writes a document, and [[wiki-draft]] |
| `references/work.md` | stubs, plans, designs, events, their lifecycle and rules, from [[Stub]], [[Spec]], and [[Event]] | [[wiki-stub]] and every spec skill |
| `references/conventions.md` | the conventions step: the policies [[context]] returns, and how to decide which bind a piece of work | [[spec-write]], [[spec-split]] |
| `references/syntax.md` | Obsidian Flavored Markdown: properties, tags, wikilinks, embeds, callouts | every skill that writes a document |

## Testing the skills

Two layers:

- A plugin test: every skill folder has its name, a description with triggers, and a `Tools:` line; every link resolves; every skill, agent, and tool that a skill, hook, or doc names exists.
- End-to-end runs in a scratch vault with scratch repositories: each skill from a plain request to its end, and a set of plain requests, each routed to the intended skill by [[atlas]].
