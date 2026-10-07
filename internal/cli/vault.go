package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/nathanaday/almagest/internal/core"
	"github.com/nathanaday/almagest/internal/migrate"
	"github.com/nathanaday/almagest/internal/vault"
)

// vaultCmd is almagest vault: the status, init, sync, snapshot, safe delete, and the
// migration of a vault of the layout before tool/.
func (c *CLI) vaultCmd(argv []string) error {
	a := parse(argv, "views", "dry-run")
	now := c.Now()
	switch a.arg(0) {
	case "", "status":
		idx, err := c.index(a)
		if err != nil {
			return err
		}
		st := core.StatusOf(idx, now)
		st.Versions = map[string]string{"binary": Version, "obsidian_plugin": idx.V.InstalledPluginVersion()}
		return c.emit(a, map[string]any{"status": st}, func(w io.Writer) { printStatus(w, st) })
	case "init":
		p := a.get("path")
		if p == "" {
			p = a.arg(1)
		}
		if p == "" {
			p = c.Dir
		}
		st, err := core.Init(vault.InitOptions{Path: p, Name: a.get("name"), Description: a.get("description"), Tagging: a.get("tagging")}, c.home(), now)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"status": st}, func(w io.Writer) {
			fmt.Fprintf(w, "Vault %s is ready at %s.\nOpen it in Obsidian (almagest open --register --vault %s). For the palette and the widgets, install Almagest from Obsidian's community plugins: %s\n", st.Vault.Name, st.Vault.Path, shellArg(st.Vault.Path), vault.PluginLink)
		})
	case "sync":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		s, err := core.Sync(v, now, core.SyncOptions{Views: a.has("views")})
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"synced": s}, func(w io.Writer) {
			fmt.Fprintf(w, "Synced: %s, %d moved back, %s, %s, %s", count(len(s.Knowledge), "knowledge document", "knowledge documents"), len(s.Moved), count(len(s.Lost), "lost session", "lost sessions"), count(len(s.Sessions), "session callout", "session callouts"), count(s.Views, "view", "views"))
			if s.Settings {
				fmt.Fprint(w, ", the harness settings")
			}
			fmt.Fprintln(w, ".")
			for _, m := range s.Strays {
				fmt.Fprintln(w, core.StrayLine(m))
			}
			for _, p := range s.Skipped {
				fmt.Fprintf(w, "Left %s as saved during the sync; the next sync derives it.\n", p)
			}
		})
	case "snapshot":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		sha, n, err := core.Snapshot(v)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"snapshot": map[string]any{"commit": sha, "files": n}}, func(w io.Writer) {
			if sha == "" {
				fmt.Fprintln(w, "Nothing to commit: the vault holds no hand edits.")
				return
			}
			fmt.Fprintf(w, "Committed %s as a snapshot, %s.\n", count(n, "hand edit", "hand edits"), short(sha))
		})
	case "trash":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		res, err := core.Trash(v, a.arg(1), now)
		if err != nil {
			return err
		}
		if res.Moved != "" {
			c.views(v, now)
		}
		if err := c.emit(a, map[string]any{"trash": res}, func(w io.Writer) { printTrash(w, res) }); err != nil {
			return err
		}
		if len(res.Backlinks) > 0 {
			return exitError{code: 2, err: fmt.Errorf("%s stays: %s; point them elsewhere first", res.Path, count(len(res.Backlinks), "document links it", "documents link it"))}
		}
		return nil
	case "migrate":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		if a.has("dry-run") {
			r, err := migrate.Plan(v)
			if err != nil {
				return err
			}
			return c.emit(a, map[string]any{"migration": r}, func(w io.Writer) { printMigration(w, r) })
		}
		r, err := migrate.Run(v, now)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"migration": r}, func(w io.Writer) { printMigration(w, r) })
	}
	return fmt.Errorf("vault takes status, init, sync, snapshot, trash, or migrate, not %q", a.arg(0))
}

func printMigration(w io.Writer, r *migrate.Report) {
	if r.Commit == "" {
		fmt.Fprintf(w, "The migration of %s would move %s into %s/ and edit %s. Nothing was written.\n", r.Vault, count(len(r.Moved), "file", "files"), vault.Tool, count(len(r.Edited), "file", "files"))
	} else {
		fmt.Fprintf(w, "Migrated %s to layout %d in one commit, %s: moved %s into %s/ and edited %s.\n", r.Vault, r.To, short(r.Commit), count(len(r.Moved), "file", "files"), vault.Tool, count(len(r.Edited), "file", "files"))
	}
	for _, e := range r.Edited {
		fmt.Fprintf(w, "  edits %s\n", e)
	}
	for _, x := range r.Warnings {
		fmt.Fprintf(w, "Note: %s.\n", x)
	}
	if r.Commit != "" {
		if r.Problems > 0 {
			fmt.Fprintf(w, "almagest lint finds %s; they were there before the migration, or name a path it could not follow.\n", count(r.Problems, "error", "errors"))
		}
		fmt.Fprintln(w, "Update the Almagest plugin in Obsidian, then start a new agent session.")
	}
}

func printTrash(w io.Writer, r *core.Trashed) {
	if len(r.Backlinks) > 0 {
		fmt.Fprintf(w, "%s stays: these link it.\n", r.Path)
		for _, b := range r.Backlinks {
			fmt.Fprintf(w, "  %s\n", b.Path)
		}
		return
	}
	fmt.Fprintf(w, "Moved %s to %s.\n", r.Path, r.Moved)
	if r.Change != nil {
		fmt.Fprintf(w, "  through the change %s (%s)\n", r.Change.Title, r.Change.ID)
	}
}

func printStatus(w io.Writer, st *core.Status) {
	fmt.Fprintf(w, "%s · %s · %s · tagging %s\n", st.Vault.Name, st.Vault.Path, st.Vault.ID, st.Vault.Tagging)
	var docs []string
	for _, t := range []string{"topic", "source", "repository"} {
		docs = append(docs, fmt.Sprintf("%d %s", st.Documents[t], t))
	}
	fmt.Fprintf(w, "Documents: %s · %d draft · %d contested\n", strings.Join(docs, ", "), st.Topics.Draft, st.Topics.Contested)
	var tagList []string
	for i, t := range st.Tags {
		if i == 12 {
			tagList = append(tagList, "…")
			break
		}
		tagList = append(tagList, fmt.Sprintf("%s %d", t.Tag, t.Count))
	}
	if len(tagList) > 0 {
		fmt.Fprintf(w, "Tags: %s\n", strings.Join(tagList, " · "))
	}
	fmt.Fprintf(w, "Sessions: %d running · %d waiting · %d idle\n", len(st.Sessions.Running), len(st.Sessions.Waiting), len(st.Sessions.Idle))
	for _, s := range st.Sessions.Waiting {
		fmt.Fprintf(w, "  waits for you: %s %s\n", s.Title, s.Description)
	}
	fmt.Fprintf(w, "Ingest: %d · Pending: %d · Proposed changes: %d · Problems: %d\n", len(st.Ingest), len(st.Pending), len(st.Changes.Proposed), st.Problems)
	for _, c := range st.Changes.Proposed {
		fmt.Fprintf(w, "  proposed: %s (%s)\n", c.Title, c.ID)
	}
}
