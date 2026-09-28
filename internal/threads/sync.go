package threads

import (
	"fmt"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Writer writes a file only when its content differs and reports whether it wrote. A
// write inside a commit passes the transaction's; sync outside one passes the vault's.
type Writer func(rel string, content []byte) (bool, error)

// Sync makes every derived part of every thread agree with its documents: the stub's
// stage, outcome, active flag, and task count; each task's active flag; and the lead
// callout of each document. It writes a file only when its content differs and never
// changes updated. It returns the paths it wrote.
func (b *Board) Sync(write Writer) ([]string, error) {
	var out []string
	for _, t := range b.Threads {
		docs := append([]*doc.Doc{t.Stub}, t.Docs()...)
		for _, d := range docs {
			content := b.derive(t, d)
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
	}
	return out, nil
}

// Docs are the thread's documents but the stub: spec, tasks, receipt, then old ones.
func (t *Thread) Docs() []*doc.Doc {
	var out []*doc.Doc
	if t.Spec != nil {
		out = append(out, t.Spec)
	}
	out = append(out, t.Tasks...)
	if t.Receipt != nil {
		out = append(out, t.Receipt)
	}
	return append(out, t.Old...)
}

// derive is a document's content with its derived fields and lead callout current.
func (b *Board) derive(t *Thread, d *doc.Doc) string {
	content := d.Content
	switch d.Type() {
	case "stub":
		done, total := t.Counts()
		outcome := ""
		if t.Receipt != nil {
			outcome = t.Receipt.Str("outcome")
		}
		content = doc.SetFields(content, []doc.Field{
			{Key: "stage", Value: t.Stage()},
			{Key: "outcome", Value: outcome},
			{Key: "active", Value: len(b.Holders(t.Title())) > 0},
			{Key: "tasks", Value: fmt.Sprintf("%d/%d", done, total)},
		})
	case "task":
		active := d.Str("status") == TaskOpen && len(b.Holders(d.Title())) > 0
		content = doc.SetField(content, "active", active)
	}
	return doc.ReplaceLead(content, b.Lead(t, d))
}

// Lead is the code-owned callout at the top of a thread document. It gives the page its
// stage's color and icon, and it leads to every other document of the thread.
func (b *Board) Lead(t *Thread, d *doc.Doc) string {
	kind := d.Type()
	title := t.Title()
	var info []string
	switch d.Type() {
	case "task":
		title = fmt.Sprintf("%s %s · %s", Label(d), TaskTitle(d), d.Str("status"))
		info = append(info, "`"+d.ID()+"`")
		if r := d.Str("repository"); r != "" {
			info = append(info, r)
		}
		var deps []string
		for _, dep := range d.List("depends") {
			if dd := b.Idx.Linked(dep); dd != nil {
				deps = append(deps, fmt.Sprintf("[[%s|%s]]", dd.Title(), Label(dd)))
			}
		}
		if len(deps) > 0 {
			info = append(info, "after "+strings.Join(deps, ", "))
		}
		if bl := d.Str("blocked"); bl != "" {
			info = append(info, "blocked: "+bl)
		}
		if d.Str("status") == TaskOpen {
			info = append(info, b.activeIn(d.Title())...)
		}
	case "receipt":
		if d.Str("outcome") == "killed" {
			kind = "killed"
		}
		if d.Front.Bool("superseded") {
			title += " · superseded"
		} else if o := d.Str("outcome"); o != "" {
			title += " · " + o
		}
		info = append(info, "`"+t.ID()+"`")
	default:
		info = append(info, "`"+t.ID()+"`")
		info = append(info, t.Stub.List("scope")...)
		if p := t.Stub.Str("priority"); p != "" && p != "normal" {
			info = append(info, p)
		}
		if bl := t.Stub.Str("blocked"); bl != "" {
			info = append(info, "blocked: "+bl)
		}
		if t.Receipt != nil {
			info = append(info, t.Receipt.Str("outcome"))
		}
		info = append(info, b.activeIn(t.Title())...)
	}
	lines := []string{b.chain(t, d), strings.Join(info, " · ")}
	if d.Type() == "receipt" && d.Front.Bool("superseded") {
		lines = append(lines, "The thread was reopened; this receipt no longer closes it.")
	}
	return doc.Callout(kind, title, lines...)
}

// activeIn names the live session that works on a thread or a task.
func (b *Board) activeIn(title string) []string {
	holders := b.Holders(title)
	if len(holders) == 0 {
		return nil
	}
	return []string{"active in " + doc.Link(holders[0].Title())}
}

// chain is the path through the thread's stages, with the current one in bold and the
// others linked when their document exists.
func (b *Board) chain(t *Thread, cur *doc.Doc) string {
	part := func(d *doc.Doc, name string) string {
		switch {
		case d == nil:
			return name
		case d.Path == cur.Path:
			return "**" + name + "**"
		}
		return fmt.Sprintf("[[%s|%s]]", d.Title(), name)
	}
	stub := part(t.Stub, "Stub")
	spec := part(t.Spec, "Spec")
	var tasks string
	done, total := t.Counts()
	switch {
	case cur.Type() == "task":
		tasks = fmt.Sprintf("**%s of %d**", Label(cur), len(t.Tasks))
	case len(t.Tasks) == 0:
		tasks = "Tasks"
	default:
		target := t.Tasks[len(t.Tasks)-1]
		if open := t.OpenTasks(); len(open) > 0 {
			target = open[0]
		}
		tasks = fmt.Sprintf("[[%s|Tasks %d/%d]]", target.Title(), done, total)
	}
	receipt := part(t.Receipt, "Receipt")
	if cur.Type() == "receipt" && cur.Front.Bool("superseded") {
		receipt = "**Receipt (superseded)**"
		if t.Receipt != nil {
			receipt = fmt.Sprintf("[[%s|Receipt]]", t.Receipt.Title())
		}
	}
	return strings.Join([]string{stub, spec, tasks, receipt}, " → ")
}

// SyncVault loads the vault and syncs every thread, writing through the vault. The caller
// holds the lock.
func SyncVault(v *vault.Vault) ([]string, error) {
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	return Load(idx).Sync(v.WriteIfChanged)
}
