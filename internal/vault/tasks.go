package vault

import (
	"regexp"
	"strings"

	"github.com/nathanaday/almagest/internal/doc"
)

// TaskLine is one open task line of a markdown file: `- [ ] …`.
type TaskLine struct {
	Doc  *doc.Doc
	Line int // 1-based
	Text string
}

var (
	openTask  = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)]) \[ \] (.*)$`)
	fenceMark = regexp.MustCompile("^\\s*(```|~~~)")
)

// OpenTaskText is the text of an open task line, or "" when the line is none.
func OpenTaskText(line string) (string, bool) {
	m := openTask.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// OpenTasks lists the open task lines that keep says to, in every markdown file of the
// vault but those under the skipped folders, outside code fences.
func (idx *Index) OpenTasks(keep func(text string) bool, skip ...string) []TaskLine {
	var out []TaskLine
	all := append(append(append([]*doc.Doc{}, idx.Notes...), idx.Docs...), idx.Misplaced...)
	for _, d := range all {
		skipped := false
		for _, s := range skip {
			if strings.HasPrefix(d.Path, s+"/") {
				skipped = true
			}
		}
		if skipped || !strings.Contains(d.Content, "[ ]") {
			continue
		}
		inFence := false
		for i, line := range strings.Split(d.Content, "\n") {
			if fenceMark.MatchString(line) {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			if text, ok := OpenTaskText(line); ok && keep(text) {
				out = append(out, TaskLine{Doc: d, Line: i + 1, Text: strings.TrimSpace(text)})
			}
		}
	}
	return out
}
