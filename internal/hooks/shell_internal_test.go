package hooks

import (
	"strings"
	"testing"
	"time"
)

// Brace expansion stays fast whatever ranges a word holds.
func TestBraceExpansionIsBounded(t *testing.T) {
	began := time.Now()
	words := expandBraces(strings.Repeat("{1..1000}", 4))
	if len(words) != 1 || time.Since(began) > time.Second {
		t.Fatalf("four ranges: %d words in %s", len(words), time.Since(began))
	}
	if got := expandBraces("a{b,c}d"); strings.Join(got, " ") != "abd acd" {
		t.Fatalf("a{b,c}d: %v", got)
	}
	if got := expandBraces("{h..h}ook"); strings.Join(got, " ") != "hook" {
		t.Fatalf("{h..h}ook: %v", got)
	}
}
