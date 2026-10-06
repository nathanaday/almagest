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
	before := len(tv.Log())
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"setup", "--yes", "--no-plugin"}, "--yes is gone: setup asks nothing"},
		{[]string{"setup", "--yes=1", "--no-plugin"}, "--yes is gone"},
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

	// The same word as the value of another option is that option's value.
	for _, args := range [][]string{
		{"setup", "--name", "--yes", "--no-plugin"},
	} {
		if code, _, errOut := r.atlas("", args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, errOut)
		}
	}
}
