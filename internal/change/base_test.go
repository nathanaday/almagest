package change_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/change"
	"github.com/nathanaday/almagest/internal/testvault"
)

// A Base upgrade before an apply lands in a commit of its own, so undo of the change
// neither names nor restores the Base, even after Obsidian rewrote it.
func TestUndoLeavesABaseUpgradeAlone(t *testing.T) {
	tv := testvault.New(t)
	shipped, err := os.ReadFile("../vault/template/old/7.0/Sessions.base")
	if err != nil {
		t.Fatal(err)
	}
	tv.Write("sessions/Sessions.base", string(shipped))
	id := tv.Doc("topic", "Motion scoring", map[string]any{"kind": "concept"}, "## Definition\n\nOld.\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Sharpen", Writes: []change.Write{{Op: "modify", ID: id, Body: str("## Definition\n\nNew.\n")}}})
	apply(t, tv, pv.Ref.ID)
	files, err := exec.Command("git", "-C", tv.V.Root, "show", "--name-only", "--format=", "HEAD").CombinedOutput()
	if err != nil || strings.Contains(string(files), "Sessions.base") {
		t.Fatalf("the change's commit holds the Base: %s %v", files, err)
	}
	upgraded := tv.Read("sessions/Sessions.base")
	if upgraded == string(shipped) {
		t.Fatal("the Base did not upgrade, so the test proves nothing")
	}
	// Obsidian rewrites a Base it opens.
	tv.Write("sessions/Sessions.base", upgraded+"\n")
	if _, err := change.Undo(tv.V, pv.Ref.ID, tv.Tick(time.Minute)); err != nil {
		t.Fatalf("undo after a Base save: %v", err)
	}
	if tv.Read("sessions/Sessions.base") != upgraded+"\n" {
		t.Fatal("undo changed the Base")
	}
}
