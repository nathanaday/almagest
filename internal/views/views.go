// Package views writes the navigation notes of views/: a home note, the timeline, the
// library, the repositories, and a note per tag in a folder per tag. A view lists, counts,
// and links; it never copies a document's content, and code can write every view from
// the documents alone. Git ignores views/, and a sync writes over any edit.
package views

import (
	"cmp"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/derive"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/tags"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Titles and places of the views.
const (
	Home        = "View · Home"
	Timeline    = "View · Timeline"
	Library     = "View · Library"
	Repos       = "View · Repositories"
	TagFolder   = "tags"
	TimeFolder  = "timeline"
	TimelineAge = 30 * 24 * time.Hour
	MaxRecent   = 10
	MaxNarrow   = 20
)

// Notice opens every view.
const Notice = "> [!view] Written by Atlas from the documents. Edits here are lost at the next sync."

// TagTitle is the title of a tag's view: Tag · school › cs513.
func TagTitle(t string) string {
	return "Tag · " + strings.ReplaceAll(t, "/", " › ")
}

// TagPath is where a tag's view lives: in a folder per tag.
func TagPath(t string) string {
	return vault.Views + "/" + TagFolder + "/" + t + "/" + TagTitle(t) + ".md"
}

// Write writes every view from an index of the vault, a file only when its content
// differs, and removes the views that stand for nothing now. It returns the paths it wrote
// or removed, and the strays: notes in views/ that code did not write, which it moves to
// inbox/ instead of deleting, since git does not hold views/. The caller holds the lock.
func Write(idx *vault.Index, now time.Time) (written []string, strays []vault.Moved, err error) {
	files := Render(idx, now)
	v := idx.V
	for rel, content := range files {
		wrote, err := v.WriteIfChanged(rel, []byte(content))
		if err != nil {
			return written, strays, err
		}
		if wrote {
			written = append(written, rel)
		}
	}
	var stale []string
	filepath.WalkDir(v.Abs(vault.Views), func(abs string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		rel := v.Rel(abs)
		if _, ok := files[rel]; !ok && strings.HasSuffix(rel, ".md") {
			stale = append(stale, rel)
		}
		return nil
	})
	for _, rel := range stale {
		data, err := os.ReadFile(v.Abs(rel))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), Notice) {
			if err := v.Remove(rel); err == nil {
				written = append(written, rel)
			}
			continue
		}
		to, err := moveToInbox(v, rel)
		if err != nil {
			return written, strays, err
		}
		strays = append(strays, vault.Moved{From: rel, To: to})
	}
	pruneEmpty(v.Abs(vault.Views))
	sort.Strings(written)
	sort.Slice(strays, func(i, j int) bool { return strays[i].From < strays[j].From })
	return written, strays, nil
}

// moveToInbox moves a note to inbox/ under a free name, and returns where it went.
func moveToInbox(v *vault.Vault, rel string) (string, error) {
	base := strings.TrimSuffix(path.Base(rel), ".md")
	to := vault.Inbox + "/" + base + ".md"
	for n := 2; v.Exists(to); n++ {
		to = fmt.Sprintf("%s/%s (%d).md", vault.Inbox, base, n)
	}
	if err := v.Contain(rel); err != nil {
		return "", err
	}
	if err := v.Contain(to); err != nil {
		return "", err
	}
	if err := os.MkdirAll(v.Abs(vault.Inbox), 0o755); err != nil {
		return "", err
	}
	return to, os.Rename(v.Abs(rel), v.Abs(to))
}

// pruneEmpty removes the empty folders under root, and keeps root.
func pruneEmpty(root string) {
	var dirs []string
	filepath.WalkDir(root, func(abs string, e fs.DirEntry, err error) error {
		if err == nil && e.IsDir() && abs != root {
			dirs = append(dirs, abs)
		}
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i])
	}
}

// Render builds every view in memory, by path.
func Render(idx *vault.Index, now time.Time) map[string]string {
	r := &renderer{idx: idx, now: now, name: filepath.Base(idx.V.Root)}
	out := map[string]string{
		vault.Views + "/" + Home + ".md":    r.home(),
		vault.Views + "/" + Library + ".md": r.library(),
		vault.Views + "/" + Repos + ".md":   r.repositories(),
	}
	main, months := r.timeline()
	out[vault.Views+"/"+Timeline+".md"] = main
	for month, content := range months {
		out[vault.Views+"/"+TimeFolder+"/"+Timeline+" "+month+".md"] = content
	}
	for t := range idx.TagCounts() {
		out[TagPath(t)] = r.tagView(t)
	}
	return out
}

type renderer struct {
	idx  *vault.Index
	now  time.Time
	name string
}

func (r *renderer) note(title string, sections ...string) string {
	var parts []string
	for _, s := range sections {
		if strings.TrimSpace(s) != "" {
			parts = append(parts, strings.TrimSpace(s))
		}
	}
	return Notice + "\n\n" + strings.Join(parts, "\n\n") + "\n"
}

func section(title, body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	return "## " + title + "\n\n" + strings.TrimSpace(body)
}

// home is the entry to the vault.
func (r *renderer) home() string {
	idx := r.idx
	counts := map[string]int{}
	for _, d := range idx.Documents() {
		counts[d.Type()]++
	}
	var proposed, waiting, live []*doc.Doc
	for _, c := range idx.Of("change") {
		if c.Str("status") == "proposed" {
			proposed = append(proposed, c)
		}
	}
	for _, s := range idx.Of("session") {
		switch s.Str("status") {
		case "waiting":
			waiting = append(waiting, s)
			live = append(live, s)
		case "running", "idle":
			live = append(live, s)
		}
	}
	mentions := idx.OpenTasks(func(t string) bool { return strings.Contains(t, "@atlas") }, vault.Documents, vault.Changes, vault.Sessions, vault.Views, vault.Scratchpad)
	todos := idx.OpenTasks(func(t string) bool { return slices.Contains(tags.Inline(t), "todo") }, vault.Views, vault.Changes, vault.Scratchpad)
	pending := len(idx.PendingDocs())
	var typeCounts []string
	plurals := map[string]string{"topic": "topics", "source": "sources", "repository": "repositories"}
	for _, t := range []string{"topic", "source", "repository"} {
		typeCounts = append(typeCounts, fmt.Sprintf("%d %s", counts[t], doc.Plural(counts[t], t, plurals[t])))
	}
	head := doc.Callout("atlas", r.idx.V.Name(),
		strings.Join(typeCounts, " · "),
		fmt.Sprintf("%d pending · %d proposed %s · %d live %s", pending, len(proposed), doc.Plural(len(proposed), "change", "changes"), len(live), doc.Plural(len(live), "session", "sessions")))
	var wait []string
	for _, c := range proposed {
		wait = append(wait, "- "+doc.Link(vault.Title(c))+" · proposed change · "+c.Str("counts"))
	}
	for _, s := range waiting {
		wait = append(wait, "- "+doc.Link(s.Title())+" · waits for your answer · "+s.Str("description"))
	}
	for _, m := range mentions {
		wait = append(wait, fmt.Sprintf("- %s: %s", doc.Link(vault.Title(m.Doc)), doc.OneLine(m.Text, 120)))
	}
	var todoLines []string
	for _, t := range todos {
		todoLines = append(todoLines, fmt.Sprintf("- %s: %s", doc.Link(vault.Title(t.Doc)), doc.OneLine(t.Text, 160)))
	}
	var tagLines []string
	for _, t := range idx.TopTags() {
		line := fmt.Sprintf("- [[%s|#%s]] · %d", TagTitle(t), t, idx.TagCounts()[t])
		if p := idx.TagPage(t); p != nil {
			line += " · " + doc.Link(p.Title())
		}
		tagLines = append(tagLines, line)
	}
	viewsList := strings.Join([]string{
		"- " + doc.Link(Timeline) + ": what happened, newest first",
		"- " + doc.Link(Library) + ": topics, sources, repositories",
		"- " + doc.Link(Repos) + ": every linked repository, its tags, and its git status",
	}, "\n")
	return r.note(Home, head,
		section("Waiting for you", strings.Join(wait, "\n")),
		section("To-do lines", strings.Join(todoLines, "\n")),
		section("Tags", strings.Join(tagLines, "\n")),
		section("Views", viewsList),
		section("Recent", strings.Join(r.recentLines(MaxRecent), "\n")))
}

// base is an inline Base over wiki/documents with the filters given and one table view.
func base(name string, filters []string, order []string, groupBy string, sortBy ...string) string {
	var b strings.Builder
	b.WriteString("```base\nfilters:\n  and:\n    - file.inFolder(\"wiki/documents\")\n")
	for _, f := range filters {
		b.WriteString("    - '" + strings.ReplaceAll(f, "'", "''") + "'\n")
	}
	b.WriteString("views:\n  - type: table\n    name: " + name + "\n")
	if groupBy != "" {
		b.WriteString("    groupBy:\n      property: " + groupBy + "\n      direction: ASC\n")
	}
	b.WriteString("    order:\n")
	for _, o := range order {
		b.WriteString("      - " + o + "\n")
	}
	if len(sortBy) > 0 {
		b.WriteString("    sort:\n")
		for _, s := range sortBy {
			prop, dir, _ := strings.Cut(s, " ")
			b.WriteString("      - property: " + prop + "\n        direction: " + cmp.Or(dir, "ASC") + "\n")
		}
	}
	b.WriteString("```")
	return b.String()
}

// hasTag is the Base filter of the documents that hold a tag or a tag below it: code
// names each, since it knows the tree.
func (r *renderer) hasTag(t string) string {
	list := []string{t}
	var walk func(string)
	walk = func(x string) {
		for _, c := range r.idx.TagChildren(x) {
			list = append(list, c)
			walk(c)
		}
	}
	walk(t)
	quoted := make([]string, len(list))
	for i, x := range list {
		quoted[i] = `"` + x + `"`
	}
	return "file.hasTag(" + strings.Join(quoted, ", ") + ")"
}

// library is the knowledge, for browsing.
func (r *renderer) library() string {
	topics := base("Topics", []string{`type == "topic"`}, []string{"file.name", "kind", "description", "tags", "status", "refreshed"}, "kind", "file.name ASC")
	sources := base("Sources", []string{`type == "source"`}, []string{"file.name", "media", "authority", "status", "captured"}, "status", "captured DESC")
	repos := base("Repositories", []string{`type == "repository"`}, []string{"file.name", "path", "branch", "head", "behind", "refreshed"}, "", "file.name ASC")
	care := base("Needs care", []string{`type == "topic"`, `status == "draft" || status == "contested" || !sources || sources.length == 0`}, []string{"file.name", "kind", "status", "sources", "refreshed"}, "status")
	return r.note(Library, section("Topics", topics), section("Sources", sources), section("Repositories", repos), section("Needs care", care))
}

// repositories lists each linked repository with its tags and its live git status block; the unlinked ones follow as links.
func (r *renderer) repositories() string {
	repos := r.idx.Of("repository")
	sort.Slice(repos, func(i, j int) bool {
		return strings.ToLower(vault.Title(repos[i])) < strings.ToLower(vault.Title(repos[j]))
	})
	var linked, unlinked []string
	for _, d := range repos {
		if d.Str("unlinked") == "true" || d.Str("path") == "" {
			unlinked = append(unlinked, "- "+doc.Link(vault.Title(d))+" · "+d.Str("description"))
			continue
		}
		link := func(t string) string { return fmt.Sprintf("[[%s|#%s]]", TagTitle(t), t) }
		own := d.Str("defines")
		var tagParts, also []string
		if own != "" {
			tagParts = append(tagParts, "Tag: "+link(own))
			if parent := tags.Parent(own); parent != "" {
				tagParts = append(tagParts, "Under: "+link(parent))
			}
		}
		for _, t := range d.List("tags") {
			if t != "" && t != own && (own == "" || !strings.HasPrefix(own, t+"/")) {
				also = append(also, link(t))
			}
		}
		if len(also) > 0 {
			label := "Also: "
			if own == "" {
				label = "Tags: "
			}
			tagParts = append(tagParts, label+strings.Join(also, " "))
		}
		lines := []string{"## " + doc.Link(vault.Title(d))}
		if desc := d.Str("description"); desc != "" {
			lines = append(lines, desc)
		}
		lines = append(lines, "`"+d.Str("path")+"`")
		if len(tagParts) > 0 {
			lines = append(lines, strings.Join(tagParts, " · "))
		}
		lines = append(lines, derive.RepoBlock(d))
		linked = append(linked, strings.Join(lines, "\n\n"))
	}
	if len(linked) == 0 {
		linked = append(linked, "No repository is linked. The repo-link skill links one.")
	}
	return r.note(Repos, strings.Join(linked, "\n\n"), section("Unlinked", strings.Join(unlinked, "\n")))
}

// timelineEntry is one line of the timeline, at a time.
type timelineEntry struct {
	at   time.Time
	line string
}

func (r *renderer) entries() []timelineEntry {
	var out []timelineEntry
	for _, c := range r.idx.Of("change") {
		if c.Str("status") != "applied" {
			continue
		}
		t, ok := schema.ParseTime(c.Str("applied"))
		if !ok {
			continue
		}
		out = append(out, timelineEntry{t, fmt.Sprintf("%s · change applied · %s · %s", t.Format(vault.ClockFormat), doc.Link(vault.Title(c)), shortCounts(c.Str("counts")))})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at.After(out[j].at) })
	return out
}

// shortCounts drops the zero counts of a change: "3 create, 1 modify".
func shortCounts(s string) string {
	var parts []string
	for _, p := range strings.Split(s, ", ") {
		if !strings.HasPrefix(p, "0 ") {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

// timeline is the last 30 days, a heading per day, and one note per earlier month.
func (r *renderer) timeline() (string, map[string]string) {
	var recent []timelineEntry
	months := map[string][]timelineEntry{}
	for _, e := range r.entries() {
		if r.now.Sub(e.at) <= TimelineAge {
			recent = append(recent, e)
		} else {
			m := e.at.Format(vault.MonthFormat)
			months[m] = append(months[m], e)
		}
	}
	var monthLinks []string
	keys := make([]string, 0, len(months))
	for m := range months {
		keys = append(keys, m)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	out := map[string]string{}
	for _, m := range keys {
		out[m] = r.note(Timeline+" "+m, byDay(months[m]))
		monthLinks = append(monthLinks, "- "+doc.Link(Timeline+" "+m))
	}
	main := r.note(Timeline, byDay(recent), section("Earlier", strings.Join(monthLinks, "\n")))
	return main, out
}

func byDay(list []timelineEntry) string {
	var b strings.Builder
	day := ""
	for _, e := range list {
		if d := vault.Date(e.at); d != day {
			if day != "" {
				b.WriteString("\n")
			}
			day = d
			b.WriteString("### " + d + "\n\n")
		}
		b.WriteString("- " + e.line + "\n")
	}
	return strings.TrimSpace(b.String())
}

func (r *renderer) recentLines(n int) []string {
	var out []string
	for i, e := range r.entries() {
		if i == n {
			break
		}
		out = append(out, "- "+vault.Date(e.at)+" "+e.line)
	}
	return out
}

// tagView is one tag's note.
func (r *renderer) tagView(t string) string {
	idx := r.idx
	count := idx.TagCounts()[t]
	var lines []string
	if p := idx.TagPage(t); p != nil {
		lines = append(lines, "Page: "+doc.Link(p.Title()))
	}
	if parent := tags.Parent(t); parent != "" {
		lines = append(lines, "Under: "+fmt.Sprintf("[[%s|#%s]]", TagTitle(parent), parent))
	}
	if kids := idx.TagChildren(t); len(kids) > 0 {
		var parts []string
		for _, k := range kids {
			parts = append(parts, fmt.Sprintf("[[%s|%s]] (%d)", TagTitle(k), tags.Leaf(k), idx.TagCounts()[k]))
		}
		lines = append(lines, "Below: "+strings.Join(parts, " · "))
	}
	head := doc.Callout("tag", fmt.Sprintf("#%s · %d %s", t, count, doc.Plural(count, "document", "documents")), lines...)
	// Narrow: the tags that occur with this one, most first.
	with := map[string]int{}
	var holders []*doc.Doc
	for _, d := range idx.Documents() {
		if !vault.Holds(d, t) {
			continue
		}
		holders = append(holders, d)
		for _, x := range tags.Expand(vault.DocTags(d)) {
			if !tags.Under(x, t) && !tags.Under(t, x) {
				with[x]++
			}
		}
	}
	var narrow []string
	keys := make([]string, 0, len(with))
	for k := range with {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if with[keys[i]] != with[keys[j]] {
			return with[keys[i]] > with[keys[j]]
		}
		return keys[i] < keys[j]
	})
	for i, k := range keys {
		if i == MaxNarrow {
			break
		}
		// Obsidian decodes with decodeURIComponent, which reads + as a plus, not a space.
		q := url.PathEscape("tag:#" + t + " tag:#" + k)
		narrow = append(narrow, fmt.Sprintf("[%s (%d)](obsidian://search?vault=%s&query=%s)", k, with[k], url.PathEscape(r.name), q))
	}
	has := map[string]bool{}
	for _, d := range holders {
		has[d.Type()] = true
	}
	filter := r.hasTag(t)
	var sections []string
	sections = append(sections, head, section("Narrow", strings.Join(narrow, " · ")))
	if has["topic"] {
		sections = append(sections, section("Topics", base("Topics", []string{filter, `type == "topic"`}, []string{"file.name", "kind", "description", "status"}, "kind", "file.name ASC")))
	}
	if has["source"] {
		sections = append(sections, section("Sources", base("Sources", []string{filter, `type == "source"`}, []string{"file.name", "media", "authority", "status"}, "", "status DESC")))
	}
	if has["repository"] {
		sections = append(sections, section("Repositories", base("Repositories", []string{filter, `type == "repository"`}, []string{"file.name", "path", "branch", "behind"}, "", "file.name ASC")))
	}
	return r.note(TagTitle(t), sections...)
}
