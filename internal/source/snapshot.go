package source

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/gitx"
	"github.com/nathanaday/almagest/internal/vault"
)

// Bounds of a snapshot.
const (
	MaxTreeEntries  = 400
	MaxFileLines    = 300
	MaxMarkers      = 300
	MaxMarkerBytes  = 1 << 20
	snapshotDepth   = 3
	markerLineLimit = 200
)

// Manifests are the build files a snapshot quotes.
var Manifests = []string{"go.mod", "package.json", "Cargo.toml", "pyproject.toml", "requirements.txt", "setup.py", "Gemfile", "pom.xml", "build.gradle", "build.gradle.kts", "composer.json", "mix.exs", "CMakeLists.txt", "Makefile", "Dockerfile", "docker-compose.yml", "compose.yaml", "deno.json", "tsconfig.json"}

// Conventions are the files a repository's conventions come from.
var Conventions = []string{"AGENTS.md", "CLAUDE.md", ".claude/CLAUDE.md", "CONTRIBUTING.md", ".editorconfig", ".golangci.yml", ".golangci.yaml", ".eslintrc", ".eslintrc.json", ".eslintrc.js", "eslint.config.js", "eslint.config.mjs", ".prettierrc", ".prettierrc.json", "ruff.toml", ".rubocop.yml", "rustfmt.toml", ".clang-format"}

// Snap is a snapshot of a repository at its head.
type Snap struct {
	Commit  string
	Content []byte
}

var marker = regexp.MustCompile(`\b(TODO|FIXME)\b[:(]?`)

// Snapshot writes a repository's report at its head: the tree to three levels, its
// instruction and convention files, its manifests, its docs, and every TODO and FIXME with
// its location. The report holds no date, so a snapshot at one commit is the same file
// and capture finds it a duplicate.
func Snapshot(repo *doc.Doc) (*Snap, error) {
	root := vault.Expand(repo.Str("path"))
	head, err := repoHead(root)
	if err != nil {
		return nil, err
	}
	// Every file is read through the root, which refuses a link that leads out of the
	// repository.
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	g := gitx.Repo{Dir: root}
	files, err := g.LsFiles()
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	tracked := map[string]bool{}
	for _, f := range files {
		tracked[f] = true
	}
	var b strings.Builder
	title := vault.Title(repo)
	fmt.Fprintf(&b, "# %s @ %s\n\n", title, head[:7])
	fmt.Fprintf(&b, "A snapshot of the repository %s (%s) at commit %s on branch %s. It lists %d tracked files.\n", title, repo.ID(), head, g.Branch(), len(files))
	if remote := g.Remote(); remote != "" {
		fmt.Fprintf(&b, "Remote: %s\n", remote)
	}
	b.WriteString("\n## Tree\n\n")
	b.WriteString(tree(files))
	for _, group := range []struct {
		title string
		names []string
	}{{"Instructions and conventions", Conventions}, {"Manifests", Manifests}} {
		var present []string
		for _, n := range group.names {
			if tracked[n] {
				present = append(present, n)
			}
		}
		if len(present) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n", group.title)
		for _, n := range present {
			quote(&b, r, n)
		}
	}
	var docs []string
	for _, f := range files {
		lower := strings.ToLower(f)
		if (strings.HasPrefix(lower, "docs/") || strings.HasPrefix(lower, "doc/")) && (strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".rst") || strings.HasSuffix(lower, ".txt")) {
			docs = append(docs, f)
		}
	}
	readme := ""
	for _, f := range files {
		if !strings.Contains(f, "/") && strings.HasPrefix(strings.ToLower(f), "readme") {
			readme = f
			break
		}
	}
	if readme != "" || len(docs) > 0 {
		b.WriteString("\n## Docs\n")
		if readme != "" {
			quote(&b, r, readme)
		}
		if len(docs) > 0 {
			b.WriteString("\nThe docs folder holds:\n\n")
			for i, d := range docs {
				if i == MaxTreeEntries {
					fmt.Fprintf(&b, "- … and %d more\n", len(docs)-i)
					break
				}
				fmt.Fprintf(&b, "- `%s`\n", d)
			}
		}
	}
	b.WriteString("\n## TODO and FIXME\n\n")
	markers := todos(r, files)
	if len(markers) == 0 {
		b.WriteString("None.\n")
	}
	for _, m := range markers {
		b.WriteString("- " + m + "\n")
	}
	return &Snap{Commit: head, Content: []byte(b.String())}, nil
}

// tree lists the folders to three levels with their file counts, and the files near the
// root, bounded.
func tree(files []string) string {
	counts := map[string]int{}
	var shown []string
	for _, f := range files {
		parts := strings.Split(f, "/")
		for depth := 1; depth < len(parts) && depth <= snapshotDepth; depth++ {
			dir := strings.Join(parts[:depth], "/") + "/"
			if counts[dir] == 0 {
				shown = append(shown, dir)
			}
			counts[dir]++
		}
		if len(parts) <= 2 {
			shown = append(shown, f)
		}
	}
	sort.Strings(shown)
	var b strings.Builder
	b.WriteString("```text\n")
	for i, s := range shown {
		if i == MaxTreeEntries {
			fmt.Fprintf(&b, "… and %d more\n", len(shown)-i)
			break
		}
		depth := strings.Count(strings.TrimSuffix(s, "/"), "/")
		name := path.Base(strings.TrimSuffix(s, "/"))
		if strings.HasSuffix(s, "/") {
			fmt.Fprintf(&b, "%s%s/ (%d files)\n", strings.Repeat("  ", depth), name, counts[s])
		} else {
			fmt.Fprintf(&b, "%s%s\n", strings.Repeat("  ", depth), name)
		}
	}
	b.WriteString("```\n")
	return b.String()
}

// quote writes a file into the report, bounded, in a fence longer than any inside it.
func quote(b *strings.Builder, r *os.Root, rel string) {
	data, err := readIn(r, rel)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	cut := ""
	if len(lines) > MaxFileLines {
		cut = fmt.Sprintf("\n[… %d more lines]", len(lines)-MaxFileLines)
		lines = lines[:MaxFileLines]
	}
	text := strings.Join(lines, "\n")
	fence := "```"
	for strings.Contains(text, fence) {
		fence += "`"
	}
	fmt.Fprintf(b, "\n### %s\n\n%s\n%s\n%s%s\n", rel, fence, text, fence, cut)
}

// todos lists every TODO and FIXME line of the tracked text files, with its location.
func todos(r *os.Root, files []string) []string {
	var out []string
	for _, f := range files {
		st, err := r.Stat(f)
		if err != nil || st.IsDir() || st.Size() > MaxMarkerBytes {
			continue
		}
		data, err := readIn(r, f)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		n := 0
		for sc.Scan() {
			n++
			line := sc.Text()
			if !marker.MatchString(line) {
				continue
			}
			text := strings.TrimSpace(line)
			if len([]rune(text)) > markerLineLimit {
				text = string([]rune(text)[:markerLineLimit]) + "…"
			}
			out = append(out, fmt.Sprintf("`%s:%d` %s", f, n, strings.ReplaceAll(text, "`", "'")))
			if len(out) == MaxMarkers {
				return append(out, "… more markers are not listed")
			}
		}
	}
	return out
}
