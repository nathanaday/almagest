
# session

> The agent's own prose for its session document. Takes a [[Session Note]] and returns it.

| `do` | Writes |
|---|---|
| `describe` | the `description` field: what this session works on, in one line |
| `progress` | one dated line under `## Progress` |
| `summary` | `## Summary`: what the session did |

The MCP server does not know which session called it, and it does not need to. The tool checks the note (one line for describe, at most 2000 characters for a summary) and returns it. The `PostToolUse` hook, which receives the session id, the subagent's `agent_id`, and the tool's input, writes the note into the right document.

The rest of the session document is the hooks' ([[Sessions]]). To list sessions, use [[search]] with `types: [session]`.
