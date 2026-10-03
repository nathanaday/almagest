package vault_test

import (
	"os"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
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
			if err := tv.V.EnsureFolders(); err != nil {
				t.Fatal(err)
			}
			got := tv.Read("sessions/Sessions.base")
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
