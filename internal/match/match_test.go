package match_test

import (
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/match"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestJoinAndMatch(t *testing.T) {
	tv := testvault.New(t)
	tv.Page("concept", "Self-supervised learning", map[string]any{"aliases": []string{"SSL"}, "description": "Training a model on data with no labels."}, "")
	tv.Page("concept", "Vision transformer", map[string]any{"description": "A transformer model for images, split into patches."}, "")
	tv.Page("entity", "Radar", map[string]any{"description": "A sensor that measures range."}, "")
	idx := tv.Index()
	maps := []match.ItemMap{
		{Doc: "src-aaaaaa", Chunk: 1, Items: []match.Item{
			{Type: "concept", Name: "self supervised learning", Claims: []match.Claim{{Text: "a", Locator: "p. 1"}}},
			{Type: "concept", Name: "Vision Transformers", Aliases: []string{"ViT"}, Description: "Transformer models that read images as patches."},
			{Type: "concept", Name: "Knowledge distillation", Description: "Training a small student model from a large teacher."},
		}},
		{Doc: "src-aaaaaa", Chunk: 2, Items: []match.Item{
			{Type: "concept", Name: "SSL", Claims: []match.Claim{{Text: "b", Locator: "p. 21"}}},
			{Type: "concept", Name: "ViT", Description: "Image transformer."},
			{Type: "entity", Name: "Radar"},
		}},
	}
	m, err := match.Run(idx, match.Input{Items: maps})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]match.Subject{}
	for _, s := range m.Subjects {
		got[s.Key] = s
	}
	ssl := got["self supervised learning"]
	if ssl.Match != "hit" || ssl.Page == nil || ssl.Page.Title != "Self-supervised learning" || len(ssl.Items) != 2 {
		t.Fatalf("a name and an alias join, and hit: %+v", ssl)
	}
	vit := got["vision transformer"]
	if vit.Match != "hit" || len(vit.Items) != 2 {
		t.Fatalf("a plural folds when the singular occurs: %+v", got)
	}
	kd := got["knowledge distillation"]
	if kd.Match != "new" {
		t.Fatalf("new: %+v", kd)
	}
	if len(m.Subjects) != 4 {
		t.Fatalf("subjects %d", len(m.Subjects))
	}
	if _, err := match.Run(idx, match.Input{Items: []match.ItemMap{{Items: []match.Item{{Name: "x"}}}}}); err == nil {
		t.Fatal("an item with no type is refused")
	}
}

func TestSiblingPages(t *testing.T) {
	tv := testvault.New(t)
	tv.Page("area", "p3", nil, "")
	tv.Page("repository", "p3-cloud", map[string]any{"parent": "[[p3]]", "path": "/a"}, "")
	tv.Page("repository", "p3-edge", map[string]any{"parent": "[[p3]]", "path": "/b"}, "")
	tv.Page("repository", "other", map[string]any{"path": "/c"}, "")
	a := tv.Page("concept", "OTA updates", map[string]any{"scope": "[[p3-cloud]]", "description": "Over the air firmware updates for devices."}, "")
	tv.Page("concept", "Firmware updates over the air", map[string]any{"scope": "[[p3-edge]]", "description": "How devices get firmware updates over the air."}, "")
	tv.Page("concept", "OTA updates elsewhere", map[string]any{"scope": "[[other]]", "description": "Over the air firmware updates for devices."}, "")
	m, err := match.Run(tv.Index(), match.Input{Pages: []string{a}, Siblings: true})
	if err != nil {
		t.Fatal(err)
	}
	s := m.Subjects[0]
	if s.Match != "near" || len(s.Neighbors) != 1 || s.Neighbors[0].Ref.Title != "Firmware updates over the air" {
		t.Fatalf("siblings only: %+v", s)
	}
}
