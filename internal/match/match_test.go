package match_test

import (
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/match"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestJoinAndMatch(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "Self-supervised learning", map[string]any{"kind": "concept", "aliases": []string{"SSL"}, "description": "Training a model on data with no labels."}, "")
	tv.Doc("topic", "Vision transformer", map[string]any{"kind": "concept", "description": "A transformer model for images, split into patches."}, "")
	tv.Doc("topic", "Radar", map[string]any{"kind": "entity", "description": "A sensor that measures range."}, "")
	idx := tv.Index()
	maps := []match.ItemMap{
		{Doc: "src-aaaaaa", Chunk: 1, Items: []match.Item{
			{Kind: "concept", Name: "self supervised learning", Claims: []match.Claim{{Text: "a", Locator: "p. 1"}}},
			{Kind: "concept", Name: "Vision Transformers", Aliases: []string{"ViT"}, Description: "Transformer models that read images as patches."},
			{Kind: "concept", Name: "Knowledge distillation", Description: "Training a small student model from a large teacher."},
		}},
		{Doc: "src-aaaaaa", Chunk: 2, Items: []match.Item{
			{Kind: "concept", Name: "SSL", Claims: []match.Claim{{Text: "b", Locator: "p. 21"}}},
			{Kind: "concept", Name: "ViT", Description: "Image transformer."},
			{Kind: "entity", Name: "Radar"},
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
		t.Fatal("an item with no kind is refused")
	}
}

func TestAcrossTheChildTagsOfATag(t *testing.T) {
	tv := testvault.New(t)
	a := tv.Doc("topic", "OTA updates", map[string]any{"kind": "concept", "tags": []string{"work/p3/p3-cloud"}, "description": "Over the air firmware updates for devices."}, "")
	tv.Doc("topic", "Firmware updates over the air", map[string]any{"kind": "concept", "tags": []string{"work/p3/p3-edge/ota"}, "description": "How devices get firmware updates over the air."}, "")
	tv.Doc("topic", "OTA in the cloud", map[string]any{"kind": "concept", "tags": []string{"work/p3/p3-cloud"}, "description": "Over the air firmware updates for devices."}, "")
	tv.Doc("topic", "OTA updates elsewhere", map[string]any{"kind": "concept", "tags": []string{"home"}, "description": "Over the air firmware updates for devices."}, "")
	m, err := match.Run(tv.Index(), match.Input{Docs: []string{a}, Tags: []string{"work/p3"}, Across: true})
	if err != nil {
		t.Fatal(err)
	}
	s := m.Subjects[0]
	if s.Match != "near" || len(s.Neighbors) != 1 || s.Neighbors[0].Ref.Title != "Firmware updates over the air" {
		t.Fatalf("another child tag only: %+v", s)
	}
	all, err := match.Run(tv.Index(), match.Input{Tags: []string{"work/p3"}, Across: true})
	if err != nil || len(all.Subjects) != 3 {
		t.Fatalf("across with no documents takes every topic under the tag: %+v %v", all, err)
	}
	if _, err := match.Run(tv.Index(), match.Input{Tags: []string{"a", "b"}, Across: true}); err == nil {
		t.Fatal("across takes one tag")
	}
}
