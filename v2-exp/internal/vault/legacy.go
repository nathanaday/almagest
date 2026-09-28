package vault

import (
	"reflect"

	"gopkg.in/yaml.v3"
)

// sameYAML reports whether two YAML texts hold the same data. Obsidian rewrites a Base
// it opens, quoting and spacing it its own way, and that is no edit.
func sameYAML(a, b string) bool {
	var x, y any
	if yaml.Unmarshal([]byte(a), &x) != nil || yaml.Unmarshal([]byte(b), &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// oldScopeBase is the Base file 0.1.0 wrote for the scope callouts, and its content as
// written; sync removes the file when it is unchanged.
const oldScopeBase = "wiki/Scope.base"

const oldScopeBaseContent = `filters:
  and:
    - file.inFolder("wiki")
    - 'file.ext == "md"'
    - 'chain.contains(this.file.asLink())'
properties:
  file.name:
    displayName: Page
views:
  - type: table
    name: In this scope
    groupBy:
      property: type
      direction: ASC
    order:
      - file.name
      - description
      - scope
      - status
    sort:
      - property: file.name
        direction: ASC
`

// OldScopeBaseContent is the old file's content, for tests.
func OldScopeBaseContent() string { return oldScopeBaseContent }
