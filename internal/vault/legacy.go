package vault

import (
	"path"
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

// oldBases are Bases an earlier release wrote, by path, with their content as written.
// Sync replaces each with the one this binary ships when nobody edited it.
var oldBases = map[string][]string{
	"threads/Threads.base": {oldThreadsBase},
}

// upgradeBases replaces each unedited Base of an earlier release with the current one.
func upgradeBases(v *Vault) error {
	for rel, olds := range oldBases {
		data, err := v.Read(rel)
		if err != nil {
			continue
		}
		for _, old := range olds {
			if !sameYAML(string(data), old) {
				continue
			}
			current, err := templates.ReadFile("template/" + path.Base(rel))
			if err != nil {
				return err
			}
			if _, err := v.WriteIfChanged(rel, current); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// oldThreadsBase is Threads.base before the By area view.
const oldThreadsBase = `filters:
  and:
    - file.inFolder("threads")
    - 'file.ext == "md"'
formulas:
  stage_order: 'if(stage == "tasks", "1 · tasks", if(stage == "spec", "2 · spec", if(stage == "stub", "3 · stub", "4 · closed")))'
  priority_order: 'if(priority == "high", 1, if(priority == "low", 3, if(priority == "someday", 4, 2)))'
properties:
  file.name:
    displayName: Thread
  formula.stage_order:
    displayName: Stage
views:
  - type: table
    name: Open
    filters:
      and:
        - 'type == "stub"'
        - 'stage != "closed"'
    groupBy:
      property: formula.stage_order
      direction: ASC
    order:
      - file.name
      - priority
      - tasks
      - active
      - blocked
      - scope
      - updated
    sort:
      - property: formula.priority_order
        direction: ASC
      - property: updated
        direction: DESC
  - type: table
    name: Active now
    filters:
      and:
        - 'type == "stub" || type == "task"'
        - 'active == true'
    order:
      - file.name
      - type
      - stage
      - status
      - tasks
      - scope
  - type: table
    name: Blocked
    filters:
      and:
        - 'type == "stub" || type == "task"'
        - 'blocked && blocked != ""'
    order:
      - file.name
      - type
      - blocked
      - thread
  - type: table
    name: Tasks
    filters:
      and:
        - 'type == "task"'
    groupBy:
      property: status
      direction: DESC
    order:
      - file.name
      - thread
      - repository
      - active
      - blocked
      - depends
    sort:
      - property: order
        direction: ASC
  - type: table
    name: Closed
    filters:
      and:
        - 'type == "stub"'
        - 'stage == "closed"'
    order:
      - file.name
      - outcome
      - scope
      - updated
    sort:
      - property: updated
        direction: DESC
`

// OldThreadsBase is the old file's content, for tests.
func OldThreadsBase() string { return oldThreadsBase }
