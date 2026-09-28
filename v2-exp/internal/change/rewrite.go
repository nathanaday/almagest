package change

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/doc"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/links"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/schema"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/vault"
)

// Retitle is one title that goes away: a rename, or a remove with a redirect.
type Retitle struct {
	Old string
	New string
	// Redirect is set when the old page is removed and New is another page; its type
	// decides which typed fields may follow the link.
	Redirect string
}

// Rewrite is one document whose links a rename changes.
type Rewrite struct {
	Path    string
	Title   string
	Before  string
	Content string
	// Links are the old titles whose links the document held.
	Links []string
}

// Rewrites computes the link rewrite pass over every markdown document of the vault but
// the scratchpad: each wikilink to an old title becomes a link to the new one, with its
// fragment and alias kept. A change document keeps its Writes, which record what was
// true then. A typed field follows a redirect only when the redirect's type fits the
// field; a link left behind is a warning. contents, when it holds a path, is the text to
// rewrite in place of the file on disk.
func Rewrites(idx *vault.Index, retitles []Retitle, contents map[string]string, skip map[string]bool) ([]Rewrite, []string) {
	if len(retitles) == 0 {
		return nil, nil
	}
	rename := links.Rename{}
	redirects := map[string]string{}
	for _, r := range retitles {
		rename[r.Old] = r.New
		if r.Redirect != "" {
			redirects[links.Key(r.Old)] = r.Redirect
		}
	}
	var out []Rewrite
	var warnings []string
	all := append(append([]*doc.Doc{}, idx.Docs...), idx.Notes...)
	sort.Slice(all, func(i, j int) bool { return all[i].Path < all[j].Path })
	for _, d := range all {
		if skip[d.Path] {
			continue
		}
		before := d.Content
		if c, ok := contents[d.Path]; ok {
			before = c
		}
		after, held, warns := rewriteContent(d, before, rename, redirects)
		warnings = append(warnings, warns...)
		if after != before {
			out = append(out, Rewrite{Path: d.Path, Title: vault.Title(d), Before: before, Content: after, Links: held})
		}
	}
	return out, warnings
}

// rewriteContent rewrites one document's text.
func rewriteContent(d *doc.Doc, content string, rename links.Rename, redirects map[string]string) (string, []string, []string) {
	front, body, hasFront := doc.Split(content)
	var held, warnings []string
	note := func(target string) {
		for _, h := range held {
			if h == target {
				return
			}
		}
		held = append(held, target)
	}
	// The body: every link, but a change document's Writes.
	bodyPart, rest := body, ""
	if d.Type() == "change" {
		for _, h := range doc.Headings(body) {
			if h.Level == 2 && strings.EqualFold(h.Title, "Writes") {
				lines := strings.Split(body, "\n")
				bodyPart = strings.Join(lines[:h.Line], "\n") + "\n"
				rest = strings.Join(lines[h.Line:], "\n")
				break
			}
		}
	}
	for _, l := range links.Find(bodyPart) {
		if _, ok := rename[oldKey(rename, l.Target)]; ok {
			note(l.Target)
		}
	}
	newBody, _ := links.Rewrite(bodyPart, rename)
	newBody += rest
	if !hasFront {
		return newBody, held, warnings
	}
	out := doc.Join(front, newBody)
	f, err := doc.ParseFront(front)
	if err != nil {
		return out, held, warnings
	}
	t := schema.Get(f.Str("type"))
	for _, key := range f.Keys() {
		values := f.List(key)
		changed := false
		next := make([]string, len(values))
		for i, v := range values {
			next[i] = v
			if !doc.IsLink(v) {
				continue
			}
			k := oldKey(rename, doc.LinkTarget(v))
			if k == "" {
				continue
			}
			note(doc.LinkTarget(v))
			if redirectType, isRedirect := redirects[links.Key(k)]; isRedirect && t != nil {
				if field := t.Field(key); field != nil && len(field.Targets) > 0 && !contains(field.Targets, redirectType) {
					warnings = append(warnings, fmt.Sprintf("%s: %s still links %s; a %s cannot take its place", vault.Title(d), key, v, redirectType))
					continue
				}
			}
			if nv, ok := links.RewriteValue(v, rename); ok {
				next[i] = nv
				changed = true
			}
		}
		if !changed {
			continue
		}
		if isList(f, key) && !(t != nil && t.Field(key) != nil && t.Field(key).Kind == schema.Link) {
			out = doc.SetField(out, key, next)
		} else {
			out = doc.SetField(out, key, next[0])
		}
	}
	return out, held, warnings
}

// oldKey is the key of rename that target names, or "".
func oldKey(rename links.Rename, target string) string {
	k := links.BaseKey(target)
	for old := range rename {
		if links.Key(old) == k {
			return old
		}
	}
	return ""
}

func isList(f *doc.Front, key string) bool {
	return f.IsList(key)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
