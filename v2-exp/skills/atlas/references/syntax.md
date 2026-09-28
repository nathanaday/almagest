# Obsidian markdown

Every document is Obsidian Flavored Markdown.

## Properties

YAML between `---` lines at the top. Quote a link in a property: `scope: "[[p3]]"`,
`sources: ["[[DINOv2]]", "[[p3-edge]]"]`. Code owns some properties of each type;
never write those (see [pages.md](pages.md) and [threads.md](threads.md)).

## Links and embeds

- `[[Title]]` links a document by its title; `[[Title|shown text]]` shows other text;
  `[[Title#Heading]]` links a heading; `[[Title#^block]]` links a block.
- `![[file.pdf]]` embeds a file; `![[Title#Heading]]` embeds a section.
- A link inside a code fence or inline code is not a link.

## Callouts

```markdown
> [!note] Title
> Text.
```

Types: note, tip, important, warning, caution, example, quote, info. The first callout
of a thread, session, or change document is code's (stub, spec, task, receipt, killed,
session, change); never write or edit one of those.

## Other syntax

- Tags: `#tag` in text, or `tags: [a, b]` in properties.
- Task lines: `- [ ] open`, `- [x] done`. A line `- [ ] @atlas …` is a mention, a
  request to the agent.
- Math: `$inline$` and `$$ block $$`. Diagrams: a fenced `mermaid` block.
- Comments: `%% hidden %%`.
