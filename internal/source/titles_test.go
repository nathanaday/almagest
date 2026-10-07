package source_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nathanaday/almagest/internal/derive"
	"github.com/nathanaday/almagest/internal/source"
	"github.com/nathanaday/almagest/internal/testvault"
)

const (
	cafeNFC = "Café"
	cafeNFD = "Café"
)

func foldsUnicode(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "probe-"+cafeNFC)
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	_, err := os.Lstat(filepath.Join(dir, "probe-"+cafeNFD))
	return err == nil
}

func TestACaptureBesideATitleInAnotherUnicodeFormTakesTheNextTitle(t *testing.T) {
	tv := testvault.New(t)
	if !foldsUnicode(t, tv.V.Abs("tool/source-core/documents")) {
		t.Skip("this file system keeps NFC and NFD names apart")
	}
	tv.Doc("topic", cafeNFC, map[string]any{"kind": "entity"}, "## What it is\n\nA café.\n")
	if _, err := derive.Sync(tv.Index(), tv.V.WriteIfChanged); err != nil {
		t.Fatal(err)
	}
	tv.Commit()
	topic := tv.Read("tool/source-core/documents/" + cafeNFC + ".md")
	tv.Write("ingest/"+cafeNFD+".md", "# Notes\n\nFrom the café.\n")
	res, err := source.Capture(tv.V, source.Request{Ingest: []string{cafeNFD + ".md"}}, at)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Captured[0].Ref.Title; got != cafeNFD+" (2)" {
		t.Fatalf("the source took the title %q", got)
	}
	if tv.Read("tool/source-core/documents/"+cafeNFC+".md") != topic {
		t.Fatal("the topic changed")
	}
	tv.Clean()
}

func TestALongWideFileNameIsCutAtACharacterBoundary(t *testing.T) {
	tv := testvault.New(t)
	name := strings.Repeat("日", 80) + ".md"
	tv.Write("ingest/"+name, "# Notes\n")
	res, err := source.Capture(tv.V, source.Request{Ingest: []string{name}}, at)
	if err != nil {
		t.Fatal(err)
	}
	title := res.Captured[0].Ref.Title
	if len(title) > 150 || !utf8.ValidString(title) || !strings.HasPrefix(name, title) {
		t.Fatalf("the title has %d bytes: %q", len(title), title)
	}
	tv.Clean()
}
