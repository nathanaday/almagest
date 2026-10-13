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
	"strings"
	"time"

	"github.com/nathanaday/almagest/internal/derive"
	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/journal"
	"github.com/nathanaday/almagest/internal/schema"
	"github.com/nathanaday/almagest/internal/tags"
	"github.com/nathanaday/almagest/internal/vault"
)

// Titles and places of the views.
const (
	Home        = "View · Home"
	Timeline    = "View · Timeline"
	Library     = "View · Library"
	Repos       = "View · Repositories"
	Navigator   = "View · Navigator"
	TimeFolder  = "timeline"
	TimelineAge = 30 * 24 * time.Hour
	MaxRecent   = 10
	MaxRoots    = 20
)

// Notice opens every view.
const Notice = "> [!view] Written by Almagest from the documents. Edits here are lost at the next sync."

// BaseNotice opens every view that is a Base, as a YAML comment.
const BaseNotice = "# Written by Almagest from the documents. Edits here are lost at the next sync."

// NavigatorPath is where the navigator lives.
var NavigatorPath = vault.WikiView + "/" + Navigator + ".base"

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
			if strings.HasPrefix(string(data), BaseNotice) {
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
	out := map[string]string{
		vault.WikiView + "/" + Home + ".md":    r.home(),
		vault.WikiView + "/" + Library + ".md": r.library(),
		vault.WikiView + "/" + Repos + ".md":   r.repositories(),
	}
	main, months := r.timeline()
	out[vault.WikiView+"/"+Timeline+".md"] = main
	for month, content := range months {
		out[vault.WikiView+"/"+TimeFolder+"/"+Timeline+" "+month+".md"] = content
	}
	out[NavigatorPath] = navigatorBase()
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
	var proposed, running, waiting, live []*doc.Doc
	for _, c := range idx.Of("change") {
		switch c.Str("status") {
		case "proposed":
			proposed = append(proposed, c)
		case "running":
			running = append(running, c)
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
	todos := idx.OpenTasks(func(t string) bool { return slices.Contains(tags.Inline(t), "todo") }, vault.WikiView, vault.Changes, vault.Scratchpad)
	pending := len(idx.PendingDocs())
	var typeCounts []string
	plurals := map[string]string{"topic": "topics", "source": "sources", "repository": "repositories"}
	for _, t := range []string{"topic", "source", "repository"} {
		typeCounts = append(typeCounts, fmt.Sprintf("%d %s", counts[t], doc.Plural(counts[t], t, plurals[t])))
	}
	head := doc.Callout("almagest", r.idx.V.Name(),
		strings.Join(typeCounts, " · "),
		fmt.Sprintf("%d pending · %d proposed %s · %d live %s", pending, len(proposed), doc.Plural(len(proposed), "change", "changes"), len(live), doc.Plural(len(live), "session", "sessions")))
	var wait []string
	for _, c := range proposed {
		wait = append(wait, "- "+doc.Link(vault.Title(c))+" · proposed change · "+c.Str("counts"))
	}
	for _, c := range running {
		wait = append(wait, "- "+doc.Link(vault.Title(c))+" · "+cmp.Or(c.Str("kind"), "work")+" running")
	}
	for _, vol := range journal.Volumes(idx) {
		if vol.Changed {
			wait = append(wait, "- the journal "+vol.Name+" ("+vault.Journals+"/"+vol.Volume+") has changes to publish")
		}
	}
	for _, s := range waiting {
		wait = append(wait, "- "+doc.Link(s.Title())+" · waits for your answer · "+s.Str("description"))
	}
	var todoLines []string
	for _, t := range todos {
		todoLines = append(todoLines, fmt.Sprintf("- %s: %s", doc.Link(vault.Title(t.Doc)), doc.OneLine(t.Text, 160)))
	}
	viewsList := strings.Join([]string{
		"- The navigator: the documents above, beside, and below the note you read. Run \"Almagest: Open the navigator\" to show it in the right sidebar",
		"- " + doc.Link(Timeline) + ": what happened, newest first",
		"- " + doc.Link(Library) + ": topics, sources, repositories",
		"- " + doc.Link(Repos) + ": every linked repository, its tags, and its git status",
	}, "\n")
	return r.note(Home, head,
		section("Waiting for you", strings.Join(wait, "\n")),
		section("To-do lines", strings.Join(todoLines, "\n")),
		section("Start here", r.startHere()),
		section("Views", viewsList),
		section("Recent", strings.Join(r.recentLines(MaxRecent), "\n")))
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

// startHere lists the roots, the documents with nothing above them, that have documents
// under them: the most under first.
func (r *renderer) startHere() string {
	type root struct {
		d     *doc.Doc
		under int
	}
	var roots []root
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
	var lines []string
	for i, x := range roots {
		if i == MaxRoots {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s · %d under it", doc.Link(vault.Title(x.d)), x.under))
	}
	var rest []string
	if more := len(roots) - MaxRoots; more > 0 {
		rest = append(rest, fmt.Sprintf("%d more with documents under them", more))
	}
	if alone > 0 {
		rest = append(rest, fmt.Sprintf("%d %s with nothing above or below", alone, doc.Plural(alone, "document", "documents")))
	}
	if len(rest) > 0 {
		lines = append(lines, "- "+doc.Capital(strings.Join(rest, ", and ")))
	}
	return strings.Join(lines, "\n")
}

// The navigator's relations. A document falls back to the page of each tag it holds
// exactly when its up is empty, as vault.UpEdges says. A tag joins with its # in a Base,
// and the commas keep a tag from matching a tag below it.
const (
	navUp        = `list(this.up).contains(file.asLink())`
	navUpPage    = `!defines.isEmpty() && this.up.isEmpty() && ("," + list(this.tags).join(",") + ",").contains(",#" + defines + ",")`
	navBelow     = `list(up).contains(this.file.asLink())`
	navBelowPage = `!this.defines.isEmpty() && up.isEmpty() && ("," + list(tags).join(",") + ",").contains(",#" + this.defines + ",")`
	navBeside    = `!this.up.isEmpty() && list(up).containsAny(list(this.up))`
	navSources   = `list(this.sources).contains(file.asLink())`
	navCitedBy   = `list(sources).contains(this.file.asLink())`
)

// navigatorBase is a Base for the right sidebar, where this is the note the reader has
// open: one table of its neighbors, grouped by how each relates to it.
func navigatorBase() string {
	relation := `if(` + navUp + ` || (` + navUpPage + `), "1 · Up", if(` + navBelow + ` || (` + navBelowPage + `), if(type == "source", "4 · Sources below", "3 · Below"), if(` + navSources + `, "5 · Sources", if(` + navCitedBy + `, "6 · Cited by", "2 · Beside"))))`
	var b strings.Builder
	b.WriteString(BaseNotice + "\n# Open it in the right sidebar: it shows the neighbors of the note you read.\n")
	b.WriteString("formulas:\n  relation: " + yamlQuote(relation) + "\n")
	b.WriteString("filters:\n  and:\n    - file.inFolder(\"" + vault.Documents + "\")\n    - file.path != this.file.path\n    - or:\n")
	for _, f := range []string{navUp, navUpPage, navBelow, navBelowPage, navBeside, navSources, navCitedBy} {
		b.WriteString("        - " + yamlQuote(f) + "\n")
	}
	b.WriteString("properties:\n  formula.relation:\n    displayName: \" \"\n")
	b.WriteString("views:\n  - type: table\n    name: Navigator\n    groupBy:\n      property: formula.relation\n      direction: ASC\n    order:\n      - file.name\n    sort:\n      - property: file.name\n        direction: ASC\n")
	return b.String()
}

func yamlQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
