// Package views writes the navigation notes of wiki-view/: a home note, the timeline, the
// library, the repositories, and a note per tag in a folder per tag. A view lists, counts,
// and links; it never copies a document's content, and code can write every view from
// the documents alone. Git ignores wiki-view/, and a sync writes over any edit.
package views

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/almagest/internal/derive"
	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/journal"
	"github.com/nathanaday/almagest/internal/schema"
	"github.com/nathanaday/almagest/internal/tags"
	"github.com/nathanaday/almagest/internal/vault"
)

// viewPrefix opens the title of every view; no document takes it (vault.ReservedPrefixes).
const viewPrefix = "View · "

// Titles and places of the views.
const (
	Home        = viewPrefix + "Home"
	Timeline    = viewPrefix + "Timeline"
	Library     = viewPrefix + "Library"
	Repos       = viewPrefix + "Repositories"
	TimeFolder  = "timeline"
	TimelineAge = 30 * 24 * time.Hour
	MaxRecent   = 10
	MaxRoots    = 20
)

// Notice opens every view.
const Notice = "> [!view] Written by Almagest from the documents. Edits here are lost at the next sync."

// BaseNotice opens every view that is a Base, as a YAML comment.
const BaseNotice = "# Written by Almagest from the documents. Edits here are lost at the next sync."

// tagRef names a tag in a view: a link to its page, or the plain tag when it has none.
func (r *renderer) tagRef(t string) string {
	if p := r.idx.TagPage(t); p != nil {
		return "[[" + vault.Title(p) + "|#" + t + "]]"
	}
	return "#" + t
}

// Write writes every view from an index of the vault, a file only when its content
// differs, and removes the views that stand for nothing now. It returns the paths it wrote
// or removed, and the strays: notes in wiki-view/ that code did not write, which it moves
// to ingest/ instead of deleting, since git does not hold wiki-view/. The caller holds the lock.
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
	filepath.WalkDir(v.Abs(vault.WikiView), func(abs string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		rel := v.Rel(abs)
		if _, ok := files[rel]; !ok && (strings.HasSuffix(rel, ".md") || strings.HasSuffix(rel, ".base")) {
			stale = append(stale, rel)
		}
		return nil
	})
	for _, rel := range stale {
		data, err := os.ReadFile(v.Abs(rel))
		if err != nil {
			continue
		}
		if strings.HasSuffix(rel, ".base") {
			// Obsidian drops the notice when it saves a Base; a reserved title still marks it as code's.
			if strings.HasPrefix(string(data), BaseNotice) || strings.HasPrefix(path.Base(rel), viewPrefix) {
				if err := v.Remove(rel); err == nil {
					written = append(written, rel)
				}
			}
			continue
		}
		if strings.Contains(string(data), Notice) {
			if err := v.Remove(rel); err == nil {
				written = append(written, rel)
			}
			continue
		}
		to, err := moveToIngest(v, rel)
		if err != nil {
			return written, strays, err
		}
		strays = append(strays, vault.Moved{From: rel, To: to})
	}
	pruneEmpty(v.Abs(vault.WikiView))
	sort.Strings(written)
	sort.Slice(strays, func(i, j int) bool { return strays[i].From < strays[j].From })
	return written, strays, nil
}

// moveToIngest moves a note to ingest/ under a free name, and returns where it went.
func moveToIngest(v *vault.Vault, rel string) (string, error) {
	base := strings.TrimSuffix(path.Base(rel), ".md")
	to := vault.Ingest + "/" + base + ".md"
	for n := 2; v.Exists(to); n++ {
		to = fmt.Sprintf("%s/%s (%d).md", vault.Ingest, base, n)
	}
	if err := v.Contain(rel); err != nil {
		return "", err
	}
	if err := v.Contain(to); err != nil {
		return "", err
	}
	if err := os.MkdirAll(v.Abs(vault.Ingest), 0o755); err != nil {
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
	main, months := r.timeline()
	out := map[string]string{
		vault.WikiView + "/" + Home + ".md":     r.home(),
		vault.WikiView + "/" + Library + ".md":  r.library(),
		vault.WikiView + "/" + Repos + ".md":    r.repositories(),
		vault.WikiView + "/" + Timeline + ".md": main,
	}
	for month, content := range months {
		out[vault.WikiView+"/"+TimeFolder+"/"+Timeline+" "+month+".md"] = content
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

// home is the entry to the vault. Its sections are inline Bases. A Base reads only files,
// so what only code knows (how many documents lie under a root, the open task lines, the
// journals with changes) goes in Home's own frontmatter, which a Base reads as this.
func (r *renderer) home() string {
	idx := r.idx
	counts := map[string]int{}
	for _, d := range idx.Documents() {
		counts[d.Type()]++
	}
	waits := len(idx.PendingDocs())
	for _, c := range idx.Of("change") {
		if s := c.Str("status"); s == "proposed" || s == "running" {
			waits++
		}
	}
	for _, s := range idx.Of("session") {
		if s.Str("status") == "waiting" {
			waits++
		}
	}
	var journals []string
	journalLines := &flowMap{}
	for _, vol := range journal.Volumes(idx) {
		if entry := journal.Entry(idx.V, vol.Volume); vol.Changed && entry != "" {
			journals = append(journals, pathLink(entry))
			journalLines.set(entry, vol.Name+" ("+vault.Journals+"/"+vol.Volume+") has changes to publish")
		}
	}
	waits += len(journals)
	var todo []string
	todoLines := &flowMap{}
	for _, t := range idx.OpenTasks(func(t string) bool { return slices.Contains(tags.Inline(t), "todo") }, vault.WikiView, vault.Changes, vault.Scratchpad) {
		if !todoLines.has(t.Doc.Path) {
			todo = append(todo, pathLink(t.Doc.Path))
		}
		todoLines.append(t.Doc.Path, doc.OneLine(t.Text, 160))
	}
	roots, rest := r.roots()
	var start []string
	under := &flowMap{}
	for _, x := range roots {
		start = append(start, pathLink(x.d.Path))
		under.number(x.d.Path, x.under)
	}
	var typeCounts []string
	plurals := map[string]string{"topic": "topics", "source": "sources", "repository": "repositories"}
	for _, t := range []string{"topic", "source", "repository"} {
		typeCounts = append(typeCounts, fmt.Sprintf("%d %s", counts[t], doc.Plural(counts[t], t, plurals[t])))
	}
	// A Base cannot list the views: Obsidian excludes wiki-view/, and a Base skips what it excludes.
	lines := []string{strings.Join(typeCounts, " · "), "Views: " + doc.Link(Library) + " · " + doc.Link(Timeline) + " · " + doc.Link(Repos)}
	if desc := idx.V.Description(); desc != "" {
		lines = append([]string{desc}, lines...)
	}
	head := doc.Callout("almagest", idx.V.Name(), lines...)

	var waitSection, todoSection, recentSection string
	if waits > 0 {
		waitSection = section("Waiting for you", inlineBase(baseSpec{
			name: "Waiting for you",
			or: []string{
				`file.inFolder("` + vault.Changes + `") && (status == "proposed" || status == "running")`,
				`file.inFolder("` + vault.Sessions + `") && status == "waiting"`,
				`file.inFolder("` + vault.Documents + `") && type == "source" && status == "pending"`,
				`list(this.journals).contains(file.asLink())`,
			},
			formulas: [][2]string{
				{"what", `if(type == "change", if(status == "proposed", "Proposed change", "Work running"), if(type == "session", "Session waits for you", if(type == "source", "Source to absorb", "Journal to publish")))`},
				{"detail", `if(type == "change", if(status == "proposed", ` + writesFormula + `, kind), if(type == "session", description, if(type == "source", "Not in the wiki yet: ask an agent to sync the wiki", this.journal_lines[file.path])))`},
			},
			order:   []string{"file.name", "formula.detail"},
			groupBy: "formula.what",
			sort:    []string{"file.name ASC"},
		}))
	}
	if len(todo) > 0 {
		todoSection = section("To-do lines", inlineBase(baseSpec{
			name:     "To-do lines",
			or:       []string{`list(this.todo).contains(file.asLink())`},
			formulas: [][2]string{{"lines", `this.todo_lines[file.path]`}},
			order:    []string{"file.name", "formula.lines"},
			sort:     []string{"file.name ASC"},
		}))
	}
	var startParts []string
	if len(roots) > 0 {
		startParts = append(startParts, "Open one, then Navigator in the Almagest palette: it shows the documents above, beside, and below the note you read.", inlineBase(baseSpec{
			name:     "Start here",
			and:      []string{`file.inFolder("` + vault.Documents + `")`, `list(this.start_here).contains(file.asLink())`},
			formulas: [][2]string{{"under", `this.under[file.path]`}},
			order:    []string{"file.name", "formula.under", "description"},
			sort:     []string{"formula.under DESC", "file.name ASC"},
		}))
	}
	if rest != "" {
		startParts = append(startParts, rest)
	}
	if len(r.entries()) > 0 {
		recentSection = section("Recent", inlineBase(baseSpec{
			name:     "Recent",
			and:      []string{`file.inFolder("` + vault.Changes + `")`, `type == "change"`, `status == "applied"`},
			formulas: [][2]string{{"applied", `date(applied).format("YYYY-MM-DD HH:mm")`}, {"writes", writesFormula}},
			order:    []string{"formula.applied", "file.name", "formula.writes"},
			sort:     []string{"applied DESC"},
			limit:    MaxRecent,
		}))
	}
	front := []string{
		"cssclasses: " + doc.FormatValue([]string{HomeClass}),
		"start_here: " + doc.FormatValue(doc.NonNil(start)),
		"under: " + under.String(),
		"todo: " + doc.FormatValue(doc.NonNil(todo)),
		"todo_lines: " + todoLines.String(),
		"journals: " + doc.FormatValue(doc.NonNil(journals)),
		"journal_lines: " + journalLines.String(),
	}
	body := r.note(Home, head, waitSection, todoSection, section("Start here", strings.Join(startParts, "\n\n")), recentSection)
	return doc.Join(strings.Join(front, "\n"), "\n"+body)
}

// HomeClass styles Home in Obsidian: the plugin hides its frontmatter, which only its Bases read.
const HomeClass = "almagest-home"

// writesFormula is a change's counts without the zero ones: "3 create, 1 modify".
const writesFormula = `counts.split(", ").filter(!value.startsWith("0 ")).join(", ")`

// pathLink is a wikilink by path, which names one file wherever its name repeats.
func pathLink(rel string) string { return "[[" + strings.TrimSuffix(rel, ".md") + "]]" }

// flowMap is a YAML flow mapping by path, in the order its keys came, which a Base reads as
// an object: this.under[file.path].
type flowMap struct {
	keys []string
	vals map[string]string
}

func (m *flowMap) has(k string) bool {
	_, ok := m.vals[k]
	return ok
}

func (m *flowMap) set(k, v string) {
	if m.vals == nil {
		m.vals = map[string]string{}
	}
	if !m.has(k) {
		m.keys = append(m.keys, k)
	}
	m.vals[k] = strconv.Quote(v)
}

// number sets a key to a number, which a Base sorts as one.
func (m *flowMap) number(k string, n int) {
	m.set(k, "")
	m.vals[k] = strconv.Itoa(n)
}

// append adds a line to a key's text, after " · ".
func (m *flowMap) append(k, v string) {
	if old, ok := m.vals[k]; ok {
		prev, _ := strconv.Unquote(old)
		v = prev + " · " + v
	}
	m.set(k, v)
}

func (m *flowMap) String() string {
	parts := make([]string, len(m.keys))
	for i, k := range m.keys {
		parts[i] = strconv.Quote(k) + ": " + m.vals[k]
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// baseSpec is an inline Base with one table view: its filters (and, with or as one of
// them, or or alone), formulas, columns, grouping, sort ("property DIR"), and row limit.
type baseSpec struct {
	name     string
	and, or  []string
	formulas [][2]string
	order    []string
	groupBy  string
	sort     []string
	limit    int
}

func inlineBase(s baseSpec) string {
	var b strings.Builder
	b.WriteString("```base\nfilters:\n")
	indent := "    "
	if len(s.and) > 0 {
		b.WriteString("  and:\n")
		for _, f := range s.and {
			b.WriteString("    - " + yamlQuote(f) + "\n")
		}
		if len(s.or) > 0 {
			b.WriteString("    - or:\n")
			indent = "        "
		}
	} else {
		b.WriteString("  or:\n")
	}
	for _, f := range s.or {
		b.WriteString(indent + "- " + yamlQuote(f) + "\n")
	}
	if len(s.formulas) > 0 {
		b.WriteString("formulas:\n")
		for _, f := range s.formulas {
			b.WriteString("  " + f[0] + ": " + yamlQuote(f[1]) + "\n")
		}
	}
	b.WriteString("views:\n  - type: table\n    name: " + s.name + "\n")
	if s.groupBy != "" {
		b.WriteString("    groupBy:\n      property: " + s.groupBy + "\n      direction: ASC\n")
	}
	if s.limit > 0 {
		b.WriteString("    limit: " + strconv.Itoa(s.limit) + "\n")
	}
	b.WriteString("    order:\n")
	for _, o := range s.order {
		b.WriteString("      - " + o + "\n")
	}
	if len(s.sort) > 0 {
		b.WriteString("    sort:\n")
		for _, x := range s.sort {
			prop, dir, _ := strings.Cut(x, " ")
			b.WriteString("      - property: " + prop + "\n        direction: " + cmp.Or(dir, "ASC") + "\n")
		}
	}
	b.WriteString("```")
	return b.String()
}

func yamlQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

type root struct {
	d     *doc.Doc
	under int
}

// roots are the documents with nothing above them that have documents under them, the
// most under first, at most MaxRoots; rest is the line for the others.
func (r *renderer) roots() (roots []root, rest string) {
	alone := 0
	for _, d := range r.idx.Documents() {
		if len(r.idx.Up(d)) > 0 {
			continue
		}
		if n := r.idx.Under(d); n > 0 {
			roots = append(roots, root{d, n})
		} else {
			alone++
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		if roots[i].under != roots[j].under {
			return roots[i].under > roots[j].under
		}
		return strings.ToLower(vault.Title(roots[i].d)) < strings.ToLower(vault.Title(roots[j].d))
	})
	var parts []string
	if more := len(roots) - MaxRoots; more > 0 {
		parts = append(parts, fmt.Sprintf("%d more with documents under them", more))
		roots = roots[:MaxRoots]
	}
	if alone > 0 {
		parts = append(parts, fmt.Sprintf("%d %s with nothing above or below", alone, doc.Plural(alone, "document", "documents")))
	}
	if len(parts) > 0 {
		rest = doc.Capital(strings.Join(parts, ", and ")) + "."
	}
	return roots, rest
}

// base is an inline Base over the documents with the filters given and one table view.
func base(name string, filters []string, order []string, groupBy string, sortBy ...string) string {
	var b strings.Builder
	b.WriteString("```base\nfilters:\n  and:\n    - file.inFolder(\"" + vault.Documents + "\")\n")
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
		link := r.tagRef
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
	at     time.Time
	line   string
	change *doc.Doc
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
		out = append(out, timelineEntry{t, fmt.Sprintf("%s · change applied · %s · %s", t.Format(vault.ClockFormat), doc.Link(vault.Title(c)), shortCounts(c.Str("counts"))), c})
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
