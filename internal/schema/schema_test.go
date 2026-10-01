package schema

import (
	"errors"
	"strings"
	"testing"
)

type fake map[string]string

func (f fake) TypeOfLink(target string) (string, error) {
	if target == "[[two]]" {
		return "", errors.New("two files")
	}
	return f[target], nil
}

func TestCheck(t *testing.T) {
	topic := Get("topic")
	good := Values{"id": "doc-abcdef", "type": "topic", "kind": "concept", "created": "2026-09-27T10:00:00", "updated": "2026-09-27", "description": "x", "status": "draft", "tags": []string{"ml/ssl", "vision"}, "sources": []string{"[[DINOv2]]"}}
	r := fake{"[[DINOv2]]": "source", "[[T]]": "stub", "[[p3]]": "repository"}
	if p := topic.Check(good, r); len(p) != 0 {
		t.Fatalf("a good topic: %v", p)
	}
	// A 6.x id keeps its prefix.
	good["id"] = "con-abcdef"
	if p := topic.Check(good, r); len(p) != 0 {
		t.Fatalf("an old id: %v", p)
	}
	bad := Values{"id": "x", "type": "topic", "kind": "idea", "created": "2026-09-27", "updated": "soon", "status": "great", "tags": []string{"Bad Tag"}, "sources": []string{"[[gone]]"}}
	var got []string
	for _, p := range topic.Check(bad, r) {
		got = append(got, p.String())
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{"id: x is not an id", "description: is required", `kind: is "idea"`, `status: is "great"`, `updated: is "soon"`, `tags: "Bad Tag" is no valid tag`, "sources: [[gone]] names no document"} {
		if !strings.Contains(joined, want) {
			t.Errorf("lacks %q in\n%s", want, joined)
		}
	}
	tasks := Get("tasks")
	if p := tasks.Check(Values{"id": "doc-abcdef", "type": "tasks", "description": "x", "created": "2026-09-27", "updated": "2026-09-27", "thread": "[[T]]", "repository": "[[T]]"}, r); len(p) != 1 || !strings.Contains(p[0].Message, "must be a repository") {
		t.Fatalf("a task list that names a stub as its repository: %v", p)
	}
	if !Get("stub").Owned("status") || Get("stub").Owned("after") || !Get("spec").Owned("thread") || !Get("event").Owned("subject") {
		t.Fatal("owners")
	}
	if len(DocumentTypes) != 9 || !IsThread("chord") || IsThread("event") || !IsDocument("event") || IsDocument("session") || Get("topic").SectionsOf("policy")[0] != "Rule" {
		t.Fatal("types")
	}
}
