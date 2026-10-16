# Comment history: `internal/bridge/keyspec`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero k2

### `go/internal/bridge/keyspec/keyspec.go:1` — above `package keyspec`

```text
// Package keyspec validates and classifies tmux key-spec tokens — the body of
// a `keystroke` envelope (the operator's "send any keyevent like a human"
// hatch). It is intentionally non-blocking: Validate only *flags* tokens that
// look like a mistyped key name, so a genuine modal-dismissal keystroke is
// never refused. tmux send-keys accepts arbitrary literal text, so literals
// are always valid; the value-add is catching typos like "Excape" / "Etner"
// before they are sent verbatim and silently do nothing.
//
// Vocabulary mirrors `tmux send-keys`: named keys (Enter, Escape, Tab, arrows,
// PgUp/PgDn, Home/End, F1–F12, …) and modifier combinations (C-c, M-x, S-Tab).
```
