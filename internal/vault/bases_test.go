package vault_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// A Base equal to any copy Atlas shipped moves to the current one; an edited Base stays.
func TestABaseUpgradesFromEveryShippedCopy(t *testing.T) {
	current := read(t, "template/Sessions.base")
	for name, before := range map[string]string{
		"the 8.x copy":   read(t, "template/old/8.1/Sessions.base"),
		"the 7.x copy":   read(t, "template/old/7.0/Sessions.base"),
		"the 6.5 copy":   read(t, "template/old/Sessions.base"),
		"an edited copy": strings.Replace(read(t, "template/old/7.0/Sessions.base"), "name: Lost", "name: Gone", 1),
	} {
		t.Run(name, func(t *testing.T) {
			tv := testvault.New(t)
			tv.Write("sessions/Sessions.base", before)
			tv.Commit()
			git := func(args ...string) string {
				out, err := exec.Command("git", append([]string{"-C", tv.V.Root}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
				return strings.TrimSpace(string(out))
			}
			setup := git("rev-parse", "HEAD")
			if err := write(tv, "Next"); err != nil {
				t.Fatal(err)
			}
			got := tv.Read("sessions/Sessions.base")
			// A commit of its own holds the new copy: no snapshot calls it a hand edit, and the
			// write's commit leaves it out.
			touched := git("log", "--format=%s", setup+"..HEAD", "--", "sessions/Sessions.base")
			if want := map[bool]string{true: "", false: "layout: upgrade sessions/Sessions.base"}[name == "an edited copy"]; touched != want {
				t.Fatalf("the commits that touch the Base: %q, want %q", touched, want)
			}
			if name == "an edited copy" {
				if got != before {
					t.Fatal("an edited Base changed")
				}
				return
			}
			if got != current {
				t.Fatalf("the Base did not upgrade:\n%s", got)
			}
		})
	}
	for _, name := range []string{"Sessions.base", "Changes.base"} {
		got := read(t, "template/"+name)
		for _, gone := range []string{"threads", "specs", "work"} {
			if strings.Contains(got, gone) {
				t.Errorf("%s still reads %s", name, gone)
			}
		}
	}
	if !strings.Contains(current, "name: By repository") {
		t.Fatal("Sessions.base lacks its repository view")
	}
}

// write is a write of one note, with a commit of its own.
func write(tv *testvault.T, title string) (err error) {
	tx, err := vault.Begin(tv.V, nil)
	if err != nil {
		return err
	}
	defer tx.End(&err)
	if err := tx.Write(vault.DocPath(title), []byte("a write\n")); err != nil {
		return err
	}
	_, err = tx.Commit("write: " + title)
	return err
}

func read(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// When git refuses the upgrade commit, the old copy goes back and the write goes on.
func TestARefusedUpgradeCommitPutsTheOldBaseBack(t *testing.T) {
	tv := testvault.New(t)
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", tv.V.Root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	// A Base git ignores: CommitOnly refuses it.
	git("rm", "-q", "--cached", "sessions/Sessions.base")
	f, err := os.OpenFile(filepath.Join(tv.V.Root, ".git", "info", "exclude"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\nsessions/Sessions.base\n")
	f.Close()
	git("commit", "-q", "-m", "ignore the Base")
	old := read(t, "template/old/7.0/Sessions.base")
	tv.Write("sessions/Sessions.base", old)
	if err := write(tv, "Goes on"); err != nil {
		t.Fatalf("the write after a refused upgrade: %v", err)
	}
	if tv.Read("sessions/Sessions.base") != old {
		t.Fatal("the old copy did not come back")
	}
	if staged := git("diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("staged: %s", staged)
	}
	if !strings.HasPrefix(git("log", "-1", "--format=%s"), "write: Goes on") {
		t.Fatalf("the write did not commit: %s", git("log", "-3", "--format=%s"))
	}
}
