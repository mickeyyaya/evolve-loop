# Eval: cli-version-freeze-claude

> Pins the closure of the HIGH inbox defect `claude-cli-version-freeze`. The
> `defaultSelfUpdateEvidence` function in `go/internal/looppreflight/freeze.go` recognized only
> `codex` as self-updating. The `claude` binary self-updated to 2.1.173 (removed the
> `esc to interrupt` affordance), breaking PaneBusy detection and causing `exit=81` in soak
> cycles 286/288/289/291. A host where claude is not brew-pinned therefore passed the
> version-freeze readiness check silently. Cycle 297 adds a `claude` clause checking
> `~/.claude/settings.json` (analogous to codex's `~/.codex/version.json`). Source incident:
> claude 2.1.173 self-update, 2026-06-11; soak batch #6 (R8.4), cycle 297.

## Task
In `defaultSelfUpdateEvidence`, add a `bin == "claude"` case that reports self-update evidence
when `~/.claude/settings.json` exists (returns `(true, path+" present ...", nil)`), absence as
`(false, "", nil)`, and an unresolvable home dir as an error (ambiguity → WARN, same as codex).

## Acceptance Criteria

### [code] Real defaultSelfUpdateEvidence recognizes claude's settings.json

```bash
cd go && go test ./internal/looppreflight/... -run TestDefaultSelfUpdateEvidence_ClaudePresent -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected output contains `--- PASS: TestDefaultSelfUpdateEvidence_ClaudePresent`.

### [code] Unpinned self-updating claude-tmux HALTs the batch end-to-end

```bash
cd go && go test ./internal/looppreflight/... -run TestRun_VersionFreeze_ClaudeUnpinnedRealEvidence_Halts -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected output contains `--- PASS: TestRun_VersionFreeze_ClaudeUnpinnedRealEvidence_Halts`.

### [code] Absent claude state does not over-fire (negative axis)

```bash
cd go && go test ./internal/looppreflight/... -run "TestDefaultSelfUpdateEvidence_ClaudeAbsent|TestRun_VersionFreeze_ClaudeNoSettings_Passes" -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected output contains `--- PASS:` for both tests and no `--- FAIL:`.

### [code] Full looppreflight package passes (no codex regression)

```bash
cd go && go test ./internal/looppreflight/... -count=1 2>&1 | tail -1
```

Expected output begins with `ok` for `github.com/mickeyyaya/evolveloop/go/internal/looppreflight`.

## Anti-gaming check

A build that leaves `defaultSelfUpdateEvidence` returning `(false, "", nil)` for every binary
except `codex` fails `TestDefaultSelfUpdateEvidence_ClaudePresent` (which redirects `HOME` to a
temp dir holding `~/.claude/settings.json` and calls the real function) and
`TestRun_VersionFreeze_ClaudeUnpinnedRealEvidence_Halts` (which wires the real evidence function
behind a claude-tmux profile and requires a HALT). Neither can be satisfied by a source string —
the registry must actually stat the claude updater-state file.
