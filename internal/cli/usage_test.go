package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Every option a command reads, in the function that runs it or in a helper it hands
// the parsed args to, is in that command's usage. vault and json are common to most commands, and a
// removed option is refused, not offered.
func TestTheUsageNamesEveryOptionACommandReads(t *testing.T) {
	fset := token.NewFileSet()
	funcs := map[string]*ast.FuncDecl{}
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				funcs[fd.Name.Name] = fd
			}
		}
	}
	literal := func(e ast.Expr) (string, bool) {
		lit, ok := e.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(lit.Value)
		return s, err == nil
	}
	// reads returns the options read and the functions called by one function.
	reads := func(fd *ast.FuncDecl) (opts, removed, calls []string) {
		ast.Inspect(fd, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				switch fn.Sel.Name {
				case "get", "has", "list":
					if len(call.Args) == 1 {
						if s, ok := literal(call.Args[0]); ok {
							opts = append(opts, s)
						}
					}
				case "removed":
					if len(call.Args) > 0 {
						if s, ok := literal(call.Args[0]); ok {
							removed = append(removed, s)
						}
					}
				}
				calls = append(calls, fn.Sel.Name)
			case *ast.Ident:
				if fn.Name == "parse" {
					for _, a := range call.Args[1:] {
						if s, ok := literal(a); ok {
							opts = append(opts, s)
						}
					}
				}
				calls = append(calls, fn.Name)
			}
			return true
		})
		return
	}
	var readsJSON []string
	for _, c := range commands {
		fd := funcs[c.name+"Cmd"]
		if fd == nil {
			continue
		}
		seen, opts, removed := map[string]bool{}, map[string]bool{}, map[string]bool{"vault": true, "json": true}
		queue := []string{fd.Name.Name}
		for len(queue) > 0 {
			name := queue[0]
			queue = queue[1:]
			if seen[name] || funcs[name] == nil {
				continue
			}
			if name != fd.Name.Name && !takesArgs(funcs[name]) {
				continue
			}
			seen[name] = true
			o, r, calls := reads(funcs[name])
			for _, x := range o {
				opts[x] = true
			}
			for _, x := range r {
				removed[x] = true
			}
			queue = append(queue, calls...)
		}
		if opts["json"] {
			readsJSON = append(readsJSON, c.name)
		}
		var missing []string
		for o := range opts {
			if !removed[o] && !strings.Contains(c.usage, "--"+o) {
				missing = append(missing, "--"+o)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("%s reads %s, which its usage does not name", c.name, strings.Join(missing, ", "))
		}
	}
	// The usage names the commands that print JSON with --json, and the ones that print
	// text only, as the code reads --json.
	listed := func(prefix string) map[string]bool {
		for _, line := range strings.Split(usageTail, "\n") {
			if rest, ok := strings.CutPrefix(line, prefix); ok {
				set := map[string]bool{}
				for _, w := range strings.Split(strings.TrimSuffix(rest, "."), ",") {
					set[strings.TrimSpace(w)] = true
				}
				return set
			}
		}
		t.Fatalf("the usage has no line %q", prefix)
		return nil
	}
	jsonList, textList := listed("--json prints JSON from:"), listed("No --json:")
	for _, c := range commands {
		reads := slices.Contains(readsJSON, c.name)
		switch {
		case c.name == "match":
			if !reads || !strings.Contains(usageTail, "match always prints JSON") {
				t.Error("match prints JSON always, and the usage must say so")
			}
		case reads && !jsonList[c.name]:
			t.Errorf("%s reads --json, but the usage does not list it among the commands that print JSON", c.name)
		case !reads && !textList[c.name]:
			t.Errorf("%s reads no --json, but the usage does not list it among the commands with no --json", c.name)
		case reads && textList[c.name], !reads && jsonList[c.name]:
			t.Errorf("the usage lists %s in the wrong group", c.name)
		}
	}

}

// takesArgs reports whether a function takes the parsed args of a command.
func takesArgs(fd *ast.FuncDecl) bool {
	for _, f := range fd.Type.Params.List {
		if id, ok := f.Type.(*ast.Ident); ok && id.Name == "args" {
			return true
		}
	}
	return false
}
