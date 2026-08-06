---
score_cap:
  - criterion: "Resolve prefers profile.digest_file content over system_prompt_file when the digest file exists on disk"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1391_005_ResolvePrefersDigestFileWhenPresent ./acs/cycle1391/"
  - criterion: "Resolve falls back to system_prompt_file when digest_file is set but the file is absent"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1391_006_ResolveFallsBackWhenDigestFileMissing ./acs/cycle1391/"
  - criterion: "Resolve is unchanged for profiles that never set digest_file (zero regression on the existing 4-tier chain)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1391_007_ResolveUnchangedWhenDigestFileUnset ./acs/cycle1391/"
  - criterion: "the systemprompt package's own regression suite stays green"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/systemprompt/... 2>&1 | grep -q '^ok'"
---

# Eval: wire scout-phase system prompt to prefer a generated digest

> Pins `go/internal/systemprompt.Resolve`'s extension for cycle-1391 Task 2:
> a new `digest_file` profile field (resolved relative to `profileDir`,
> mirroring `system_prompt_file`) that, when present on disk, wins over
> `system_prompt_file` in the precedence chain
> (`env > profile.system_prompt > digest_file > system_prompt_file > ""`).
> This is the injection seam for `go/internal/digest`'s output
> ([[digest-projector-core]]): once a per-phase digest is generated, Resolve
> must prefer it automatically, with no new env flag
> (`no_feature_flags_use_design_patterns`) and zero regression for the
> existing four-tier chain when a profile never opts in. The
> digest-file-set-but-missing case is the load-bearing fallback predicate —
> without it, a profile pointed at a digest that hasn't been generated yet
> (or was deleted) would silently resolve to `""` instead of degrading
> gracefully to the hand-authored `system_prompt_file`. Source:
> scout-report.md cycle 1391, Task 2 (`systemprompt.go:1-7` precedence
> comment).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| digest-preferred | digest_file wins over system_prompt_file when present | 8/10 | `TestC1391_005_...` |
| graceful-fallback | digest_file set but missing → system_prompt_file | 7/10 | `TestC1391_006_...` |
| no-regression | digest_file unset → existing chain unaffected | 8/10 | `TestC1391_007_...` |
| package-green | systemprompt suite stays green | 6/10 | `go test ./internal/systemprompt/...` |
