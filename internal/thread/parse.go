package thread

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
)

// Requirement is one line of a spec's Requirements: `- R1: text`.
type Requirement struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Task states.
const (
	TaskOpen    = "open"
	TaskDone    = "done"
	TaskDropped = "dropped"
)

// Task is one check box of a task list's Tasks section:
// `- [ ] T2: text (R1, R2)`, `- [x] T1: text (R1) · a3f9c21 · note`, or
// `- [-] T3: text (R1) · dropped: reason`. A line a user typed with no id is a task too.
type Task struct {
	ID           string   `json:"id"`
	State        string   `json:"state"`
	Text         string   `json:"text"`
	Requirements []string `json:"requirements"`
	// Trail is what follows the text: the commits and the line of a done task, or the
	// reason of a dropped one.
	Trail string `json:"trail,omitempty"`
	// Line is the index of the task's line in the list's content.
	Line int `json:"-"`
	// List is the task list that holds it.
	List *doc.Doc `json:"-"`
}

// Finding is one check box of a verification's Findings: `- [ ] F1: text`, closed as
// `- [x] F1: text → outcome`.
type Finding struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Open    bool   `json:"open"`
	Outcome string `json:"outcome,omitempty"`
	Line    int    `json:"-"`
}

// Row is one row of a verification's Requirements table.
type Row struct {
	Requirement string `json:"requirement"`
	Text        string `json:"text,omitempty"`
	Result      string `json:"result"`
	Evidence    string `json:"evidence"`
}

// trailSep parts a task's text from its trail, and the items of the trail.
const trailSep = " · "

// outcomeSep parts a finding's text from its outcome.
const outcomeSep = " → "

var (
	reqLine     = regexp.MustCompile(`^\s*[-*]\s+\**(R\d+)\**\s*[:.]\s*(.+)$`)
	boxLine     = regexp.MustCompile(`^\s*[-*] \[(.)\] (.*)$`)
	taskID      = regexp.MustCompile(`^(T\d+): (.*)$`)
	findingID   = regexp.MustCompile(`^(F\d+): (.*)$`)
	reqRefs     = regexp.MustCompile(`\((R\d+(?:\s*,\s*R\d+)*)\)`)
	reqID       = regexp.MustCompile(`^R\d+$`)
	rowID       = regexp.MustCompile(`^(R\d+)(?:\s*[:.]\s*(.*))?$`)
	fenceToggle = regexp.MustCompile("^\\s*(```|~~~)")
)

// Requirements reads the requirement lines of a spec's body, in order. A line that
// continues a requirement is part of its prose, not of its text here.
func Requirements(body string) []Requirement {
	text, _ := doc.Section(body, "Requirements")
	var out []Requirement
	for _, l := range strings.Split(text, "\n") {
		if m := reqLine.FindStringSubmatch(l); m != nil {
			out = append(out, Requirement{ID: m[1], Text: strings.TrimSpace(m[2])})
		}
	}
	return out
}

// sectionLines are the lines of a level-two section with their indexes in the content,
// outside code fences.
func sectionLines(content, title string) (lines []string, index []int) {
	inSection, fence := false, false
	for i, l := range strings.Split(content, "\n") {
		if fenceToggle.MatchString(l) {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		if strings.HasPrefix(l, "## ") {
			inSection = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(l, "## ")), title)
			continue
		}
		if inSection {
			lines = append(lines, l)
			index = append(index, i)
		}
	}
	return lines, index
}

// Tasks reads the check boxes of a task list's Tasks section.
func Tasks(list *doc.Doc) []Task {
	lines, index := sectionLines(list.Content, "Tasks")
	var out []Task
	for i, l := range lines {
		m := boxLine.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		t := Task{State: TaskDone, Line: index[i], List: list, Requirements: []string{}}
		switch m[1] {
		case " ":
			t.State = TaskOpen
		case "-":
			t.State = TaskDropped
		}
		rest := m[2]
		if id := taskID.FindStringSubmatch(rest); id != nil {
			t.ID, rest = id[1], id[2]
		}
		t.Text, t.Trail, _ = strings.Cut(rest, trailSep)
		t.Text = strings.TrimSpace(t.Text)
		if refs := reqRefs.FindStringSubmatch(t.Text); refs != nil {
			for _, r := range strings.Split(refs[1], ",") {
				t.Requirements = append(t.Requirements, strings.TrimSpace(r))
			}
		}
		out = append(out, t)
	}
	return out
}

// TaskLine renders a task as its line.
func TaskLine(t Task) string {
	box := " "
	switch t.State {
	case TaskDone:
		box = "x"
	case TaskDropped:
		box = "-"
	}
	line := "- [" + box + "] "
	if t.ID != "" {
		line += t.ID + ": "
	}
	line += t.Text
	if t.Trail != "" {
		line += trailSep + t.Trail
	}
	return line
}

// TaskText is a task's text for a new line: one line, its requirements in brackets at
// the end, and no separator of its own.
func TaskText(text string, reqs []string) string {
	text = strings.ReplaceAll(oneLine(text, 300), trailSep, ", ")
	text = strings.TrimSpace(reqRefs.ReplaceAllString(text, ""))
	if len(reqs) > 0 {
		text += " (" + strings.Join(reqs, ", ") + ")"
	}
	return text
}

// Details is the text under a task's heading in a list's Details section, or "".
func Details(list *doc.Doc, id string) string {
	text, _ := doc.Section(list.Body, "Details")
	lines := strings.Split(text, "\n")
	start := -1
	for i, l := range lines {
		if !strings.HasPrefix(l, "### ") {
			continue
		}
		if start >= 0 {
			return strings.TrimSpace(strings.Join(lines[start:i], "\n"))
		}
		head := strings.TrimSpace(strings.TrimPrefix(l, "### "))
		if head == id || strings.HasPrefix(head, id+":") || strings.HasPrefix(head, id+" ") {
			start = i + 1
		}
	}
	if start >= 0 {
		return strings.TrimSpace(strings.Join(lines[start:], "\n"))
	}
	return ""
}

// Findings reads the check boxes of a verification's Findings section.
func Findings(v *doc.Doc) []Finding {
	lines, index := sectionLines(v.Content, "Findings")
	var out []Finding
	for i, l := range lines {
		m := boxLine.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		f := Finding{Open: m[1] == " ", Line: index[i]}
		rest := m[2]
		if id := findingID.FindStringSubmatch(rest); id != nil {
			f.ID, rest = id[1], id[2]
		}
		f.Text, f.Outcome, _ = strings.Cut(rest, outcomeSep)
		f.Text = strings.TrimSpace(f.Text)
		out = append(out, f)
	}
	return out
}

// Rows reads the rows of a verification's Requirements table.
func Rows(v *doc.Doc) []Row {
	text, _ := doc.Section(v.Body, "Requirements")
	var out []Row
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "|") {
			continue
		}
		cells := splitRow(l)
		if len(cells) < 3 {
			continue
		}
		m := rowID.FindStringSubmatch(cells[0])
		if m == nil {
			continue
		}
		out = append(out, Row{Requirement: m[1], Text: m[2], Result: strings.ToLower(cells[1]), Evidence: cells[2]})
	}
	return out
}

// splitRow cuts a table row into its cells, at the pipes a backslash does not escape.
func splitRow(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			cur.WriteByte('|')
			i++
		case line[i] == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(line[i])
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

// cell makes text safe inside a table cell: one line, with its pipes escaped.
func cell(text string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(text), " "), "|", `\|`)
}

func shortHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])[:12]
}

// SpecHash is the hash of what a verification checks a thread against: the spec's
// requirements and its rules. A reworded requirement or rule makes a verification stale.
func SpecHash(spec *doc.Doc) string {
	if spec == nil {
		return ""
	}
	var b strings.Builder
	for _, r := range Requirements(spec.Body) {
		b.WriteString(r.ID + ": " + strings.Join(strings.Fields(r.Text), " ") + "\n")
	}
	rules, _ := doc.Section(spec.Body, "Rules")
	b.WriteString(strings.Join(strings.Fields(rules), " "))
	return shortHash(b.String())
}

// TasksHash is the hash of the tasks a verification covered: the tasks that are not
// dropped, by id or text. A task added later makes the verification stale.
func TasksHash(lists []*doc.Doc) string {
	var keys []string
	for _, l := range lists {
		for _, t := range Tasks(l) {
			if t.State == TaskDropped {
				continue
			}
			if t.ID != "" {
				keys = append(keys, t.ID)
			} else {
				keys = append(keys, t.Text)
			}
		}
	}
	sort.Strings(keys)
	return shortHash(strings.Join(keys, "\n"))
}

// number is the number of an id like T12, R3, or F1, or 0.
func number(id string) int {
	if len(id) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(id[1:])
	return n
}
