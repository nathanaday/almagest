package vault_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
)

// A Base equal to any copy Atlas shipped moves to the current one; an edited Base stays.
func TestABaseUpgradesFromEveryShippedCopy(t *testing.T) {
	current := read(t, "template/Sessions.base")
	for name, before := range map[string]string{
		"the 7.0 to 8.1 copy": read(t, "template/old/7.0/Sessions.base"),
		"the 6.5 copy":        read(t, "template/old/Sessions.base"),
		"an edited copy":      strings.Replace(read(t, "template/old/7.0/Sessions.base"), "name: Lost", "name: Gone", 1),
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
			if _, err := thread.Stub(tv.V, thread.StubIn{Title: "Next", Text: "A write."}, thread.Opts{Now: tv.Clock}); err != nil {
				t.Fatal(err)
			}
			got := tv.Read("sessions/Sessions.base")
			// The write's own commit holds the new copy; no snapshot calls it a hand edit.
			touched := git("log", "--format=%s", setup+"..HEAD", "--", "sessions/Sessions.base")
			if want := map[bool]string{true: "", false: "thread: stub Next"}[name == "an edited copy"]; touched != want {
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
	if strings.Contains(current, "specs") || !strings.Contains(current, "name: By thread") {
		t.Fatal("Sessions.base still reads specs")
	}
}

func read(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
