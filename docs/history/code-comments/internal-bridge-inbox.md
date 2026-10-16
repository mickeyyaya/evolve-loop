# Comment history: `internal/bridge/inbox`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d7

### `go/internal/bridge/inbox/inbox.go:14` — above `type Kind string`

```text
// Kind enumerates the injection semantics of an envelope.
//
//	command     — inject when the agent is idle (prompt marker visible).
//	interrupt   — send ESC first, then inject regardless of agent state.
//	nudge       — observer-originated command; idle-gated like command.
//	system_rule — late rule injection; idle-gated, prefixed "## Rules".
//	keystroke   — raw tmux send-keys, no idle-gate, no ESC prefix, no Enter
//	              suffix. Body is one tmux key-spec (literal text or a named
//	              key like "Enter" / "Escape" / "C-c" / "Up", or several
//	              space-separated tokens) sent verbatim. This is the operator's
//	              "full tmux control" hatch (cycle-124 F4 / ADR-0023): used by
//	              `evolve bridge send` and the phase-observer to dismiss
//	              modals, navigate menus, and confirm y/N prompts that no
//	              other Kind handles. The operator carries full responsibility
//	              for what reaches the running REPL.
```

### `go/internal/bridge/inbox/inbox.go:57` — above `CorrID string 'json:"corr_id,omitempty"'`

```text
// CorrID, when set, ties this injected ask to the agent's reply in the
// channel feed (ADR-0037). The driver echoes it in its inject_applied /
// idle_reached breadcrumbs; the producer brackets the answer span by it.
```

### `go/internal/bridge/inbox/inbox_test.go:251` — above `func TestKind_Valid_AllRecognizedKinds(t *testing.T) {`

```text
// TestKind_Valid_AllRecognizedKinds pins the exhaustive set of envelope kinds
// the bridge accepts. Adding a Kind constant requires extending Valid() in
// the same change; this test fails if either drifts. Cycle-124 F4 added
// KindKeystroke to the set (per ADR-0023 addendum) so the operator can send
// raw tmux key tokens through `evolve bridge send --kind=keystroke`.
```
