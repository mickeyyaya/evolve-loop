# Comment history: `internal/phases/swarmrunner`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## groupKiller injected-kill fix

### `go/internal/phases/swarmrunner/kill_unix_test.go:41`

```text
// tmux teardown tests live in internal/swarm (ExecTmuxKill, seam-based — never
// a real tmux exec). The predecessor here live-fired `tmux kill-session -t ''`
// against the shared default socket, which tmux resolves to the CALLER'S OWN
// session — running this suite from inside any tmux pane killed that session
// (2026-06-11 killer-B). Do not reintroduce real-tmux calls in unit tests.
```
