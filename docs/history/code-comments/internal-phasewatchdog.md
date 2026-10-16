# Comment history: `internal/phasewatchdog`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d8

### `go/internal/phasewatchdog/phasewatchdog.go:1` — above `package phasewatchdog`

```text
// Package phasewatchdog ports legacy/scripts/dispatch/phase-watchdog.sh.
//
// Activity-based phase watchdog. Background loop that polls file mtimes
// within a workspace and kills a stalled process group when no file
// activity has been detected for longer than the threshold.
//
// v9.4.0 phase-aware baseline: the effective idle baseline is the LATER of
// (a) the freshest watched-file mtime and (b) the current phase's start
// time. A new phase advance resets the clock — preventing false positives
// when a phase produces zero writes but is actively progressing.
```
