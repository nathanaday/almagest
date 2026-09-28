
# Session Note

> The input of [[session]]: the prose the agent adds to its own session document.

```yaml
do: describe | progress | summary
text: "Score detection boxes by motion to cut vehicle false alarms"
```

The tool checks the note and returns it. The `PostToolUse` hook writes it into the session document of the session that made the call, which the hook knows by its session id. See [[Sessions#Links]].
