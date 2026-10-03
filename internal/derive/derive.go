// Package derive keeps the parts of the knowledge documents that code owns: the lead
// callout of a source, a repository, and a topic; the sections code writes (a source's
// original, a repository's live status block and its Threads and Knowledge Bases, an
// overview's Map, a topic's Threads when a spec cites it); a source's status; and a
// repository's git facts.
package derive

import (
	"cmp"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/tags"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// WriteFunc writes a file when its content differs, and reports whether it wrote.
type WriteFunc func(rel string, content []byte) (bool, error)

// Sync makes every source, repository, and topic agree with the vault: the derived
// fields, the lead callouts, and the sections code writes. It writes a file only when its
// content differs, and never changes updated. It returns the paths it wrote.
func Sync(idx *vault.Index, write WriteFunc) ([]string, error) {
	var out []string
	cited := Cited(idx)
	for _, d := range idx.Of("source", "repository", "topic") {
		content := derived(idx, d, cited)
		if content == d.Content {
			continue
		}
		wrote, err := write(d.Path, []byte(content))
		if err != nil {
			return out, err
		}
		if wrote {
			out = append(out, d.Path)
		}
	}
	return out, nil
}

// Cited are the paths of the documents that some thread's spec links.
func Cited(idx *vault.Index) map[string]bool {
	out := map[string]bool{}
	for _, s := range idx.Of("spec") {
		for _, l := range links.Find(s.Body) {
			for _, p := range idx.LinkPaths(l.Target) {
				out[p] = true
			}
		}
	}
	return out
}

func derived(idx *vault.Index, d *doc.Doc, cited map[string]bool) string {
	content := d.Content
	switch d.Type() {
	case "source":
		status := "absorbed"
		if idx.Pending(d) {
			status = "pending"
		}
		if !d.Front.Equal("status", status) {
			content = doc.SetField(content, "status", status)
		}
		content = doc.ReplaceLead(content, SourceLead(idx, doc.Parse(d.Path, []byte(content))))
		content = putOriginal(content, d)
	case "repository":
		content = doc.ReplaceLead(content, RepositoryLead(d))
		content = putBlock(content, "atlas-repo", RepoBlock(d))
		front, body, _ := doc.Split(content)
		order := schema.Get("repository").Sections
		// 7.x named the section Work.
		body = doc.RemoveSection(body, "Work")
		body = doc.PutSection(body, "Threads", RepoThreadsBase, order)
		if def := d.Str("defines"); def != "" && tags.Valid(def) {
			body = doc.PutSection(body, "Knowledge", TagBase(idx, def, "Knowledge", false), order)
		} else {
			body = doc.RemoveSection(body, "Knowledge")
		}
		content = doc.Join(front, body)
	case "topic":
		content = doc.ReplaceLead(content, TopicLead(idx, d))
		front, body, _ := doc.Split(content)
		order := schema.Get("topic").SectionsOf(d.Str("kind"))
		if def := d.Str("defines"); d.Str("kind") == "overview" && def != "" && tags.Valid(def) {
			body = doc.PutSection(body, "Map", TagBase(idx, def, "Map", true), order)
		}
		if cited[d.Path] {
			body = doc.PutSection(body, "Threads", TopicThreadsBase, order)
		} else {
			body = doc.RemoveSection(body, "Threads")
		}
		content = doc.Join(front, body)
	}
	return content
}

// mediaName is how a callout names a source's media.
var mediaName = map[string]string{"pdf": "PDF", "image": "Image", "markdown": "Markdown", "text": "Text", "office": "Office file", "audio": "Audio", "video": "Video", "other": "File"}

// SourceLead is a source's card: its media, its size, its authority; its authors and
// date; where it came from; and the change that absorbed it.
func SourceLead(idx *vault.Index, d *doc.Doc) string {
	title := mediaName[d.Str("media")]
	if title == "" {
		title = "File"
	}
	if m := d.Str("measure"); m != "" {
		title += " · " + m
	}
	title += " · " + cmp.Or(d.Str("authority"), "unknown")
	var lines []string
	var by []string
	if a := d.List("authors"); len(a) > 0 {
		names := a
		if len(names) > 3 {
			names = append(append([]string{}, names[:2]...), "and others")
		}
		by = append(by, joinNames(names))
	}
	if p := d.Str("published"); p != "" {
		by = append(by, "published "+p)
	}
	if len(by) > 0 {
		lines = append(lines, strings.Join(by, " · "))
	}
	where := "Captured " + day(d.Str("captured"))
	switch origin, loc := d.Str("origin"), d.Str("locator"); {
	case origin == "url" && loc != "":
		where += " from " + loc
	case loc != "":
		where += " from `" + loc + "` (" + origin + ")"
	case origin != "":
		where += " (" + origin + ")"
	}
	if c := idx.AbsorbedBy(d); c != nil && !idx.Pending(d) {
		where += " · absorbed by " + doc.Link(vault.Title(c))
	} else {
		where += " · pending: not yet ingested"
	}
	lines = append(lines, where)
	return doc.Callout("source", title, lines...)
}

func joinNames(n []string) string {
	switch len(n) {
	case 1:
		return n[0]
	case 2:
		return n[0] + " and " + n[1]
	}
	return strings.Join(n[:len(n)-1], ", ") + ", " + n[len(n)-1]
}

// embedMedia are the media Obsidian shows inline.
var embedMedia = map[string]bool{"pdf": true, "image": true, "audio": true, "video": true}

// Original is the line that shows a source's original: an embed for what Obsidian shows
// inline, a link for the rest.
func Original(d *doc.Doc) string {
	file := doc.LinkTarget(d.Str("file"))
	if file == "" {
		return ""
	}
	media := d.Str("media")
	if embedMedia[media] || (media == "markdown" && d.Str("origin") != "repository" && short(d.Str("measure"))) {
		return "![[" + file + "]]"
	}
	return "[[" + file + "|Open the original (" + strings.TrimPrefix(path.Ext(file), ".") + ")]]"
}

// short reports whether a measure is at most 200 lines.
func short(measure string) bool {
	var n int
	if _, err := fmt.Sscanf(measure, "%d lines", &n); err == nil {
		return n <= 200
	}
	return false
}

var originalLine = regexp.MustCompile(`^!?\[\[[^\]]+\]\]\s*$`)

// putOriginal puts a source's original line right after the lead callout, in place of
// one there.
func putOriginal(content string, d *doc.Doc) string {
	line := Original(doc.Parse(d.Path, []byte(content)))
	if line == "" {
		return content
	}
	front, body, ok := doc.Split(content)
	lead := doc.Lead(body)
	rest := strings.TrimLeft(doc.StripLead(body), "\n")
	first, after, _ := strings.Cut(rest, "\n")
	if originalLine.MatchString(first) && strings.Contains(first, doc.LinkTarget(d.Str("file"))) {
		rest = strings.TrimLeft(after, "\n")
	}
	out := "\n"
	if lead != "" {
		out += lead + "\n\n"
	}
	out += line + "\n"
	if rest != "" {
		out += "\n" + rest
	}
	if !ok {
		return out
	}
	return doc.Join(front, out)
}

// putBlock puts a fenced block of a language right after the lead callout, in place of
// one there.
func putBlock(content, lang, block string) string {
	front, body, ok := doc.Split(content)
	lead := doc.Lead(body)
	rest := strings.TrimLeft(doc.StripLead(body), "\n")
	if strings.HasPrefix(rest, "```"+lang) {
		if end := strings.Index(rest[3:], "\n```"); end >= 0 {
			rest = strings.TrimLeft(rest[3+end+4:], "\n")
		}
	}
	out := "\n"
	if lead != "" {
		out += lead + "\n\n"
	}
	out += block + "\n"
	if rest != "" {
		out += "\n" + rest
	}
	if !ok {
		return out
	}
	return doc.Join(front, out)
}

// RepositoryLead is a repository's card: its path, its branch and head, its remote, and
// how far its description is behind.
func RepositoryLead(d *doc.Doc) string {
	p := d.Str("path")
	if d.Front.Bool("unlinked") || p == "" {
		return doc.Callout("repository-missing", "Unlinked", "Kept for its links. Agents no longer work in it; repo-link links it again.")
	}
	if !gitx.IsRoot(vault.Expand(p)) {
		return doc.Callout("repository-missing", "`"+p+"` is gone", "The path is no git work tree now. Link the new path (repo-link), or unlink it (repo-unlink).")
	}
	var lines []string
	head := "`" + cmp.Or(d.Str("branch"), "?") + "`"
	if h := d.Str("head"); h != "" {
		head += " at `" + h + "`"
		if t, ok := schema.ParseTime(d.Str("head_time")); ok {
			head += ", " + t.Format("2006-01-02 15:04")
		}
	}
	if r := d.Str("remote"); r != "" {
		head += " · `" + shortRemote(r) + "`"
	}
	lines = append(lines, head)
	desc := "Not described yet (repo-ingest)"
	if c := d.Str("described"); c != "" {
		desc = "Described at `" + c + "`"
		if n := d.Front.Int("behind"); n > 0 {
			desc += fmt.Sprintf(", %d %s behind", n, doc.Plural(n, "commit", "commits"))
		} else {
			desc += ", current"
		}
	}
	if def := d.Str("defines"); def != "" {
		desc += " · tag #" + def
	}
	lines = append(lines, desc)
	return doc.Callout("repository", "`"+p+"`", lines...)
}

// shortRemote is a remote without its scheme and user: github.com/acme/p3-edge.
func shortRemote(r string) string {
	r = strings.TrimSuffix(r, ".git")
	if i := strings.Index(r, "://"); i >= 0 {
		r = r[i+3:]
	}
	if at := strings.Index(r, "@"); at >= 0 {
		r = r[at+1:]
	}
	return strings.Replace(r, ":", "/", 1)
}

// RepoBlock is a repository's live status block, which the Obsidian plugin renders.
func RepoBlock(d *doc.Doc) string {
	return "```atlas-repo\n" + d.ID() + " · " + d.Str("path") + " · live status needs the Atlas plugin\n```"
}

// TopicLead is a topic's card: its kind, its status, its sources, and its tags. A policy
// says its strength and reach; an overview names its tag.
func TopicLead(idx *vault.Index, d *doc.Doc) string {
	kind := cmp.Or(d.Str("kind"), "concept")
	status := cmp.Or(d.Str("status"), "stable")
	var lines []string
	title := ""
	switch kind {
	case "policy":
		title = doc.Capital(cmp.Or(d.Str("strength"), "should"))
		reach := d.List("tags")
		if len(reach) == 0 {
			title += " · holds for every repository"
		} else {
			parts := make([]string, len(reach))
			for i, t := range reach {
				parts[i] = "#" + t
			}
			title += " · holds for repositories tagged " + strings.Join(parts, " and ")
		}
	case "overview":
		def := d.Str("defines")
		title = "The page of #" + def
		if p := tags.Parent(def); p != "" {
			if page := idx.TagPage(p); page != nil {
				title += " · under " + doc.Link(page.Title())
			} else {
				title += " · under #" + p
			}
		}
	default:
		title = doc.Capital(kind)
	}
	switch status {
	case "deprecated", "contested", "draft":
		title = doc.Capital(status) + " · " + title
	}
	n := len(d.List("sources"))
	meta := fmt.Sprintf("%d %s", n, doc.Plural(n, "source", "sources"))
	if r := d.Str("refreshed"); r != "" {
		meta += " · refreshed " + day(r)
	}
	if kind != "policy" {
		if t := tagLine(d.List("tags")); t != "" {
			meta += " · " + t
		}
	}
	lines = append(lines, meta)
	return doc.Callout(kind, title, lines...)
}

// RepoThreadsBase is a repository's Threads section: an inline Base of the threads with a
// task list for it.
const RepoThreadsBase = "```base\n" + `filters:
  and:
    - file.inFolder("wiki/documents")
    - 'type == "stub"'
    - 'list(repositories).contains(this.file.asLink())'
views:
  - type: table
    name: Threads
    order:
      - file.name
      - status
      - tasks
      - priority
      - refreshed
    sort:
      - property: refreshed
        direction: DESC
` + "```"

// TopicThreadsBase is a topic's Threads section: an inline Base of the specs that cite
// the topic, each with its thread and its status.
const TopicThreadsBase = "```base\n" + `filters:
  and:
    - file.inFolder("wiki/documents")
    - 'type == "spec"'
    - 'file.hasLink(this.file)'
views:
  - type: table
    name: Threads
    order:
      - thread
      - status
      - file.name
      - refreshed
    sort:
      - property: refreshed
        direction: DESC
` + "```"

// TagBase is an inline Base of the documents that hold a tag or a tag below it, grouped
// by type. It names each tag, since code knows the tree.
func TagBase(idx *vault.Index, tag, name string, withEvents bool) string {
	list := []string{tag}
	var walk func(t string)
	walk = func(t string) {
		for _, c := range idx.TagChildren(t) {
			list = append(list, c)
			walk(c)
		}
	}
	walk(strings.ToLower(tag))
	sort.Strings(list[1:])
	quoted := make([]string, len(list))
	for i, t := range list {
		quoted[i] = `"` + t + `"`
	}
	events := "\n    - 'type != \"event\"'"
	if withEvents {
		events = ""
	}
	return "```base\n" + `filters:
  and:
    - file.inFolder("wiki/documents")
    - 'file.hasTag(` + strings.Join(quoted, ", ") + `)'` + events + `
views:
  - type: table
    name: ` + name + `
    groupBy:
      property: type
      direction: ASC
    order:
      - file.name
      - description
      - status
      - refreshed
` + "```"
}

// GitFacts reads git for each linked repository and writes remote, branch, head,
// head_time, and behind when one of them changed, with refreshed set to now. A quiet
// repository gets no write. It returns the paths it wrote.
func GitFacts(idx *vault.Index, write WriteFunc, now time.Time) ([]string, error) {
	var out []string
	for _, d := range idx.Of("repository") {
		content, changed := Facts(d, now)
		if !changed {
			continue
		}
		wrote, err := write(d.Path, []byte(content))
		if err != nil {
			return out, err
		}
		if wrote {
			out = append(out, d.Path)
		}
	}
	return out, nil
}

// Facts is a repository's content with its git facts read now, and whether they changed.
func Facts(d *doc.Doc, now time.Time) (string, bool) {
	p := d.Str("path")
	if p == "" || d.Front.Bool("unlinked") {
		return d.Content, false
	}
	g := gitx.Repo{Dir: vault.Expand(p)}
	if !gitx.IsRoot(g.Dir) {
		return d.Content, false
	}
	head, _ := g.Head()
	fields := []doc.Field{
		{Key: "remote", Value: g.Remote()},
		{Key: "branch", Value: g.Branch()},
		{Key: "head", Value: shortSHA(head)},
	}
	if t := g.HeadTime(); !t.IsZero() {
		fields = append(fields, doc.Field{Key: "head_time", Value: vault.Stamp(t)})
	}
	if c := d.Str("described"); c != "" {
		if n, err := g.Behind(c); err == nil {
			fields = append(fields, doc.Field{Key: "behind", Value: n})
		}
	}
	changed := false
	content := d.Content
	for _, f := range fields {
		if !d.Front.Equal(f.Key, f.Value) {
			content = doc.SetField(content, f.Key, f.Value)
			changed = true
		}
	}
	if changed {
		content = doc.SetField(content, "refreshed", vault.Stamp(now))
	}
	return content, changed
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

func tagLine(list []string) string {
	out := make([]string, 0, len(list))
	for _, t := range list {
		if tags.Valid(t) {
			out = append(out, "#"+t)
		}
	}
	sort.Strings(out)
	return strings.Join(out, " · ")
}

func day(stamp string) string {
	if t, ok := schema.ParseTime(stamp); ok {
		return vault.Date(t)
	}
	return stamp
}
