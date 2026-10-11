# Comment history: `internal/llmcalls`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## dispatch postmortem follow-ups, fix round 1 (ADR-0130, 2026-10-11)

### `go/internal/llmcalls/record.go:108` — above `FreshSessionRetry bool 'json:"fresh_session_retry,omitempty"'`

```text
// FreshSessionRetry marks a dispatch whose pane died with a session-
// recoverable cause and was followed by ONE fresh session of the same CLI
// inside the same Launch (bridge F31). The two rows share Attempt; this
// flag — with the BRIDGE_FRESH_SESSION_RETRY signal's call_id — tells the
// dead dispatch from the one that followed.
```
