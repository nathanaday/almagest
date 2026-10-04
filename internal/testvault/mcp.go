package testvault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MCPServer writes a shell script that answers an MCP client: initialize, and a tool
// list of the given names. A name may hold a shell expansion, such as ${HOME}, so a test
// can see what the server's environment held. It returns the script's path.
func MCPServer(t *testing.T, tools ...string) string {
	t.Helper()
	var list []string
	for _, name := range tools {
		list = append(list, `{"name":"`+name+`","inputSchema":{"type":"object"}}`)
	}
	script := `#!/bin/sh
tools="[` + strings.ReplaceAll(strings.Join(list, ","), `"`, `\"`) + `]"
while IFS= read -r line; do
  id=$(printf '%s' "$line" | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p')
  [ -n "$id" ] || continue
  case "$line" in
    *'"tools/list"'*) printf '{"jsonrpc":"2.0","id":%s,"result":{"tools":%s}}\n' "$id" "$tools" ;;
    *'"initialize"'*) printf '{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"fake","version":"0"}}}\n' "$id" ;;
    *) printf '{"jsonrpc":"2.0","id":%s,"result":{}}\n' "$id" ;;
  esac
done
`
	path := filepath.Join(t.TempDir(), "server")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
