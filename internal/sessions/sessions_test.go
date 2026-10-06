package sessions_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// The gate lets a change with no writes apply at once, whatever it absorbs, and holds a
// change with writes until the user of its session answers after the proposal.
func TestUserAnsweredGatesOnlyAChangeThatWrites(t *testing.T) {
	tv := testvault.New(t)
	rel, err := sessions.Start(tv.V, sessions.Event{Harness: "claude", SessionID: "a1b2c3d4-5e6f-7a8b-9c0d-000000000001", Cwd: tv.V.Root}, tv.Clock)
	if err != nil {
		t.Fatal(err)
	}
	session := vault.NoteTitle(rel)
	tv.Write("threads/Plan · Spec.md", "---\nid: doc-pl0001\ntype: spec\n---\n## Goal\n\nx\n")
	proposed := tv.Tick(time.Minute)
	d := doc.Parse("changes/2026-09/Close the plan.md", []byte(doc.Render([]doc.Field{
		{Key: "id", Value: "chg-000001"},
		{Key: "type", Value: "change"},
		{Key: "status", Value: "proposed"},
		{Key: "proposed", Value: vault.Stamp(proposed)},
		{Key: "session", Value: doc.Link(session)},
		{Key: "absorbs", Value: []string{doc.Link("Plan · Spec")}},
	}, "")))
	gate := sessions.UserAnswered(tv.V)
	if err := gate(d, 0); err != nil {
		t.Fatalf("a change with no writes waits: %v", err)
	}
	if err := gate(d, 1); err == nil || !strings.Contains(err.Error(), "wait for the user's yes") {
		t.Fatalf("a change with writes before the user's turn: %v", err)
	}
	s := sessions.ByTitle(tv.V, session)
	if s == nil {
		t.Fatal("no session document")
	}
	tv.Write(rel, doc.SetField(s.Content, "status", sessions.Ended))
	if err := gate(d, 1); err == nil || !strings.Contains(err.Error(), "has ended") || !strings.Contains(err.Error(), "presses Approve") {
		t.Fatalf("a change whose session ended: %v", err)
	}
	tv.Write(rel, doc.SetField(s.Content, "last_prompt", vault.Stamp(tv.Tick(time.Minute))))
	if err := gate(d, 1); err != nil {
		t.Fatalf("a change with writes after the user's turn: %v", err)
	}
}
