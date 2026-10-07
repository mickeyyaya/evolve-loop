---
score_cap:
  - criterion: "A malformed policy.json yields a coded WARN on stderr from phase-observer config resolution instead of a silent zero-value fallback"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1819_001_' ./acs/cycle1819"
  - criterion: "--enforce stall resolution over a policy.json that decodes with a type error also emits the coded WARN"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1819_002_' ./acs/cycle1819"
  - criterion: "evolve phases --persona-override without ':' is a usage error (exit 10) naming the flag, while <path>:<name> keeps working"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1819_00[34]_' ./acs/cycle1819"
  - criterion: "The phases usage and unknown-subcommand texts list every dispatched subcommand, including check-coherence, check-artifact-coherence and check-provenance"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1819_005_' ./acs/cycle1819"
  - criterion: "phase-observer rejects a zero or non-numeric pgid at the flag boundary with ExitInvalidArgs and a message naming pgid"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1819_006_' ./acs/cycle1819"
  - criterion: "The phasecmd package stays green, formatted and vetted"
    max_if_missing: 8
    evidence: "cd go && go vet ./internal/cli/phasecmd/ && test -z \"$(gofmt -l internal/cli/phasecmd)\" && go test -count=1 ./internal/cli/phasecmd/"
---

# Eval: phasecmd policy and profile load errors no longer fall back silently

> Pins the repair of five silent fallbacks in `go/internal/cli/phasecmd` found by
> comment-reduction batch 15 (inbox `phasecmd-silent-policy-fallbacks`, 2026-09-26)
> and shipped in cycle 1819: `loadObserverPolicy` and `resolveStallPolicy` dropped
> a malformed `.evolve/policy.json` to a zero Policy without a word, a
> `--persona-override` value without `:` was ignored, a bogus pgid parsed to 0 and
> relied on a downstream guard, and the `evolve phases` usage text omitted two of
> its own subcommands. Every predicate drives the production entry points
> (`phasecmd.RunPhaseObserver`, `phasecmd.RunPhases`, wired in
> `go/cmd/evolve/registry.go`), never an internal helper.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| silent-policy-observer | malformed policy.json -> coded WARN from observer resolution | 7/10 | `go test -tags acs -run TestC1819_001_ ./acs/cycle1819` |
| silent-policy-stall | mistyped policy.json under --enforce -> coded WARN | 6/10 | `go test -tags acs -run TestC1819_002_ ./acs/cycle1819` |
| persona-override-flag | no-colon value is usage error 10; well-formed value still accepted | 7/10 | `go test -tags acs -run 'TestC1819_00[34]_' ./acs/cycle1819` |
| usage-drift | usage and unknown-subcommand texts list every dispatched name | 7/10 | `go test -tags acs -run TestC1819_005_ ./acs/cycle1819` |
| zero-pgid | pgid 0 / non-numeric rejected at the flag boundary | 6/10 | `go test -tags acs -run TestC1819_006_ ./acs/cycle1819` |
| package-health | phasecmd vet, gofmt and unit tests green | 8/10 | `go test -count=1 ./internal/cli/phasecmd/` |
