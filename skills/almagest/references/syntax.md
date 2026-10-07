# Obsidian markdown

Every document is Obsidian Flavored Markdown.

## Properties

YAML between `---` lines at the top. Quote a link in a property:
`subject: "[[Score boxes by motion]]"`, `sources: ["[[DINOv2]]", "[[p3-edge]]"]`. Code
owns some properties of each type; never write those (see [pages.md](pages.md)).

## Tags

- `tags: [work/p3, ml]` in properties: the document's categories. No `#` in the
  property. `/` nests a tag.
- `#todo` in text: a marker, not a category. Code reads only the property.

## Links and embeds

- `[[Title]]` links a document by its title; `[[Title|shown text]]` shows other text;
  `[[Title#Heading]]` links a heading; `[[Title#^block]]` links a block.
- `![[file.pdf]]` embeds a file; `![[Title#Heading]]` embeds a section.
- A link inside a code fence or inline code is not a link.
- A title never holds `/ \ : * ? " < > | [ ] # ^`; code removes them.

## Callouts

```markdown
> [!note] Title
> Text.
```

Types: note, tip, important, warning, caution, example, quote, info.

The first callout of a typed document is code's, when its type is one of these: source,
repository, repository-missing, concept, entity, policy, overview. The first callout of a
session or change document is code's too (session, change). Never write or edit one of
those. Every other callout is the user's, or yours to write in a section you own.

## Other syntax

- Task lines: `- [ ] open`, `- [x] done`.
- Math: `$inline$` and `$$ block $$`. Diagrams: a fenced `mermaid` block.
- Comments: `%% hidden %%`.
