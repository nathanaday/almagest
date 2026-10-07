package journal_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/journal"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

func TestNames(t *testing.T) {
	for folder, want := range map[string]string{"cs566-notes": "CS566 Notes", "deep_work": "Deep Work", "Garden": "Garden", "p3 field log": "P3 Field Log", "école": "École"} {
		if got := journal.Name(folder); got != want {
			t.Errorf("%s: %q, want %q", folder, got, want)
		}
	}
	day := time.Date(2026, 10, 6, 9, 0, 0, 0, time.Local)
	if got := journal.Title("cs566-notes", day); got != "User Journal CS566 Notes - 6 October 2026 Edition" {
		t.Fatalf("the title: %q", got)
	}
}

func TestTheHistoryBaseQuotesTheVolume(t *testing.T) {
	note := journal.HistoryNote(`Nathan's "notes"`)
	if !strings.Contains(note, `    - 'volume == "Nathan''s \"notes\""'`) {
		t.Fatalf("the Base:\n%s", note)
	}
}

func TestPublish(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("journals/cs566-notes/Week 1.md", "---\nmood: good\n---\nGradient descent finally clicked.\n")
	tv.Write("journals/cs566-notes/labs/Lab 1.md", "The lab used [[Backpropagation]].\n")
	tv.Write("journals/empty/.keep", "")
	tv.Commit()
	vols := journal.Volumes(tv.Index())
	if len(vols) != 2 || vols[0].Volume != "cs566-notes" || vols[0].Notes != 2 || !vols[0].Changed || vols[1].Notes != 0 || vols[1].Changed {
		t.Fatalf("the volumes: %+v", vols)
	}
	if _, err := journal.Publish(tv.V, "empty", tv.Clock); err == nil || !strings.Contains(err.Error(), "holds no note") {
		t.Fatalf("an empty volume: %v", err)
	}
	if _, err := journal.Publish(tv.V, ".", tv.Clock); err == nil {
		t.Fatal("the journals folder itself")
	}
	if _, err := journal.Publish(tv.V, "../scratchpad", tv.Clock); err == nil {
		t.Fatal("a path out of journals/")
	}

	day := tv.Tick(time.Hour)
	p, err := journal.Publish(tv.V, "cs566-notes", day)
	if err != nil {
		t.Fatal(err)
	}
	title := journal.Title("cs566-notes", day)
	if p.Source.Title != title || p.Commit == "" {
		t.Fatalf("the edition: %+v", p)
	}
	tv.Clean()
	src := doc.Parse("", []byte(tv.Read(vault.DocPath(title))))
	for k, want := range map[string]string{"origin": "journal", "authority": "primary", "volume": "cs566-notes", "edition": vault.Date(day), "locator": "journals/cs566-notes", "status": "pending"} {
		if got := src.Str(k); got != want {
			t.Errorf("%s: %q, want %q", k, got, want)
		}
	}
	original := tv.Read(vault.Originals + "/" + doc.LinkTarget(src.Str("file")))
	want := "# " + title + "\n\n## Week 1\n\nGradient descent finally clicked.\n\n## labs/Lab 1\n\nThe lab used [[Backpropagation]].\n"
	if original != want {
		t.Fatalf("the edition's text:\n%s\nwant:\n%s", original, want)
	}
	history := tv.Read("journals/cs566-notes/Journal · cs566-notes.md")
	if !strings.HasPrefix(history, journal.HistoryNotice) || !strings.Contains(history, `'volume == "cs566-notes"'`) {
		t.Fatalf("the history:\n%s", history)
	}
	if v := journal.Volumes(tv.Index())[0]; v.Changed || v.Edition != title {
		t.Fatalf("after the publish: %+v", v)
	}

	if _, err := journal.Publish(tv.V, "cs566-notes", tv.Tick(time.Hour)); err == nil || !strings.Contains(err.Error(), "no change since") {
		t.Fatalf("a publish with no change: %v", err)
	}
	tv.Write(vault.DocPath(title), doc.SetField(tv.Read(vault.DocPath(title)), "tags", []string{"school/cs566"}))
	tv.Write("journals/cs566-notes/Week 2.md", "Momentum.\n")
	tv.Commit()
	if v := journal.Volumes(tv.Index())[0]; !v.Changed {
		t.Fatal("a new note does not mark the volume changed")
	}
	second, err := journal.Publish(tv.V, "cs566-notes", tv.Clock)
	if err != nil {
		t.Fatal(err)
	}
	if second.Source.Title != title+" (2)" || strings.Join(second.Source.Tags, ",") != "school/cs566" {
		t.Fatalf("the second edition of the day: %+v", second.Source)
	}
	if tv.Read("journals/cs566-notes/Week 1.md") != "---\nmood: good\n---\nGradient descent finally clicked.\n" {
		t.Fatal("publish changed a journal note")
	}
}
