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
	concept := Get("concept")
	good := Values{"id": "con-abcdef", "type": "concept", "created": "2026-09-27", "updated": "2026-09-27", "description": "x", "status": "draft", "scope": "[[p3]]", "sources": []string{"[[DINOv2]]"}}
	r := fake{"[[p3]]": "area", "[[DINOv2]]": "source", "[[T]]": "stub"}
	if p := concept.Check(good, r); len(p) != 0 {
		t.Fatalf("a good page: %v", p)
	}
	bad := Values{"id": "ent-abcdef", "type": "concept", "created": "2026-09-27", "updated": "x", "status": "great", "scope": "[[T]]", "sources": []string{"[[gone]]"}}
	var got []string
	for _, p := range concept.Check(bad, r) {
		got = append(got, p.String())
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{"id: ent-abcdef does not start with con-", "description: is required", `status: is "great"`, "scope: [[T]] is a stub; it must be a area or repository", "sources: [[gone]] names no document"} {
		if !strings.Contains(joined, want) {
			t.Errorf("lacks %q in\n%s", want, joined)
		}
	}
	if ByID("tsk-abcdef").Name != "task" || ByID("nope") != nil || !Get("stub").Owned("stage") || Get("stub").Owned("priority") {
		t.Fatal("lookups and owners")
	}
	if len(Types) != 13 {
		t.Fatalf("%d types", len(Types))
	}
}
