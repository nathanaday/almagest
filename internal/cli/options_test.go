package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestARemovedOptionIsRefused(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	tv.Write("chord.json", `{"title": "Ship it", "threads": []}`)
	tv.Commit()
	before := len(tv.Log())
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"chord", "create", "--new-tags", "chord.json"}, `--new-tags is gone: chord create reads "new_tags": true from its JSON`},
		{[]string{"chord", "create", "chord.json", "--new-tags=true"}, "--new-tags is gone"},
		{[]string{"setup", "--yes", "--no-plugin"}, "--yes is gone: setup asks nothing"},
	} {
		code, _, errOut := r.atlas("", c.args...)
		if code == 0 || !strings.Contains(errOut, c.want) {
			t.Fatalf("%v: exit %d: %s", c.args, code, errOut)
		}
	}
	if len(tv.Log()) != before {
		t.Fatalf("a refused option wrote a commit: %v", tv.Log())
	}
	if _, err := os.Stat(filepath.Join(tv.Home.Root, "bin")); !os.IsNotExist(err) {
		t.Fatalf("setup --yes installed the binary: %v", err)
	}
}
