---
score_cap:
  - criterion: "A failed lane's released inbox item (FAIL outcome drain and cycle processing release) contains no absolute home path: continuation.worktree and continuation.findings_path are in ~ form, branch/snapshot_sha/base_sha/cycle unchanged"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1781_004_' ./acs/cycle1781/"
  - criterion: "The next builder still reads the finding: the production continuation resolver (inboxmover.ResolveContinuationForScope, wired at cmd_cycle.go) hands core's findings reader a findings_path it can open as given, from a claimed item that carries no home path"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1781_007_' ./acs/cycle1781/"
  - criterion: "Stamp paths outside the home directory, including a sibling directory that shares the home prefix, pass through verbatim"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1781_005_' ./acs/cycle1781/"
  - criterion: "The continuation registry, host runtime state, stores and returns absolute paths"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1781_006_' ./acs/cycle1781/"
  - criterion: "Every export of the touched apicover-enrolled packages (inboxmover, inboxmover/lifecycle, continuation) is named by a _test.go in its package"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1781_008_' ./acs/cycle1781/"
---

# Eval: a pending item's continuation stamp carries no host paths

> `continuation.RedactHostPaths` turns the home directory into `~` for
> released bindings, because tracked inbox items ride commits to the public
> remote (audit cycle-1507 M1). A pending item's own `continuation` stamp
> did not get the same treatment. `lifecycle.Mover.releaseOne` marshalled
> the manifest it reads through `continuation.ReadManifest` straight into the
> item. So every failed lane that preserved work wrote the operator's
> `/Users/<account>/…` worktree and findings paths into a tracked file; the
> stamps from cycles 1761 and 1762 landed that way on 2026-09-29.
>
> The stamp must be redacted. Every reader of `findings_path` must then
> expand `~` before opening it, or the next builder silently loses the prior
> findings. The only reader, `core.readContinuationFindings`, opens whatever
> the continuation resolver returns. The registry is host runtime state and
> keeps absolute paths.
>
> The stamp writer (`inboxmover/lifecycle/release.go`) and the findings reader
> (`core/continuation_stamp.go`) are protected surfaces. A lane therefore
> redacts in the `continuation` package and expands in the resolver
> (`inboxmover/continuation_resolve.go`). The cycle-1781 TDD phase used a
> `-overlay` mutation probe of exactly that shape: all predicates passed, and
> removing the expansion turned `TestC1781_007_` RED.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| redaction | released item holds no home path; ~ form; identity intact | 9/10 | `TestC1781_004_` |
| reader expands | resolver hands the builder an openable findings_path | 9/10 | `TestC1781_007_` |
| edge | non-home and home-prefixed sibling paths verbatim | 6/10 | `TestC1781_005_` |
| negative | the registry keeps absolute paths | 6/10 | `TestC1781_006_` |
| apicover | touched enrolled packages name every export | 5/10 | `TestC1781_008_` |
