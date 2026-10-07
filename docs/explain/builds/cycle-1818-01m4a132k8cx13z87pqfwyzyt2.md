# Build Explanation — Cycle 1818

## Build Binding
- Cycle: 1818
- Base SHA: feef882e23794a9bf02d575077d666c2f655d60f

## Summary
Cycle 1818 retries cycle 1815, whose audit never ran because the auditor's CLI exited with rc=1. The worktree carries 1815's salvaged fix for four resolver-hygiene defects in `internal/policy` and `internal/setup`. `WorkflowConfig` now returns copies of `PhaseEnables` and `RemediablePhases` instead of the loaded policy's own map and slice, and it assigns the three remediation knobs once instead of twice. `ChronicleConfig` and `FailureDispositionConfig` ignore negative overrides and keep the compiled defaults. `internal/setup` drops its private one-pass `baseCLI` and calls `policy.BaseCLI`, which strips driver suffixes repeatedly. The acceptance contract is re-authored under `go/acs/cycle1818`, and 1815's predicate package and unshipped explanation are archived.

## Rationale
Because the result aliased the loaded policy, a caller that wrote to `WorkflowConfig().PhaseEnables` or `.RemediablePhases` changed that policy and every later resolve. The other slice and map fields were already copied, so cloning these two makes the resolver consistent. Deleting the duplicated tail block also removes a second assignment that re-aliased `RemediablePhases`. The `!= 0` guards let a negative digest budget, or a negative escalation threshold, step or cap, reach the escalation boundary, and a negative cap drives a recurrence weight below zero. Guarding on `> 0` matches the package's existing rule that numeric overrides apply only when positive. Setup's one-pass strip turned `codex-tmux-p` into a phantom `codex-tmux` family in `Detect` and `Recommend`. The shared normalizer already handled chained suffixes. The retry keeps the salvaged code unchanged, because the 1815 failure was an infrastructure failure and not a defect in the change.

## Changed Areas
- `go/internal/policy/workflow.go` — clones `PhaseEnables` with `maps.Clone` and `RemediablePhases` with `slices.Clone`, and deletes the duplicated remediation block at the end of `WorkflowConfig`, so no result aliases the loaded policy.
- `go/internal/policy/policy_chronicle.go` — `digest_tokens` and `digest_cycles` override only when positive, so a negative value resolves to the default.
- `go/internal/policy/policy_disposition.go` — `threshold`, `step` and `cap` override only when positive, so a negative value cannot reach the escalation boundary.
- `go/internal/setup/setup.go` — removes the private `baseCLI` and its `strings` import; `Detect` groups doctor rows by `policy.BaseCLI`.
- `go/internal/setup/recommend.go` — the default-family preference, the cross-family pair and the allowed-family set all normalize through `policy.BaseCLI`.
- `go/internal/policy/remediation_policy_test.go` — adds `TestWorkflowConfig_MutatingResultDoesNotChangeNextResolve`, which mutates one result and checks the next resolve and the loaded policy. It also pins that an explicit empty list stays empty and non-nil.
- `go/internal/policy/policy_chronicle_test.go` — adds `TestChronicleConfig_NegativeOverridesResolveToDefaults`, which covers negative, zero and small positive overrides.
- `go/internal/policy/policy_disposition_test.go` — adds `TestFailureDispositionConfig_NegativeOverridesResolveToDefaults`, which covers negative and small positive overrides.
- `go/acs/cycle1818/predicates_test.go` — this cycle's eleven TDD-authored acceptance predicates for the task. They replace the archived cycle-1815 package, so the gates run only a contract this cycle owns.
- `.evolve/evals/policy-resolver-hygiene.md` — the task's eval with five score caps, with its evidence commands repointed from `TestC1815_*` to `TestC1818_*`.
- `docs/architecture/packages/internal-policy.md` — records that resolvers hand out copies and that the chronicle and disposition numeric overrides apply only when positive.
- `docs/architecture/packages/internal-setup.md` — records that setup normalizes CLI names through `policy.BaseCLI` and keeps no normalizer of its own.
- `docs/private/research/archived-2026-10-07/superseded-predicate-packages/cycle1815/predicates_test.go` — cycle 1815's predicate package, moved out of `go/acs`. Its contract is superseded by cycle 1818's, so it no longer runs.
- `docs/private/research/archived-2026-10-07/unshipped-build-explanations/cycle-1815-01m4930vcx9cfwvj5728e9ktfa.md` — cycle 1815's explanation document. It is archived because that cycle never shipped and this document replaces it.

## Design Decisions
`slices.Clone` and `maps.Clone` were chosen over `append([]string(nil), …)` because they keep nil as nil and an explicit empty list as empty. The resolved values therefore match the old ones exactly, except that they no longer alias the policy. The package already uses `slices.Clone`. Setup calls `policy.BaseCLI` rather than `profiles.BaseCLI`, because `policy.BaseCLI` is the exported normalizer the rest of the runtime uses. Zero still means "unset" for the chronicle and disposition knobs, as before; only negative values change behavior. The retry re-authors predicates instead of reusing cycle 1815's, so that the audit grades a contract bound to this cycle.

## Verification
On main, ten of the eleven cycle-1818 predicates fail. The one that passes is ADR uniqueness, which was already fixed. On this worktree, all eleven pass. `gofmt -l` prints nothing, and `go vet` and `go test -count=1` pass for `internal/policy` and `internal/setup`. The native ACS suite for cycle 1818 reports no red predicates.

## Compatibility
No exported symbol, JSON key or policy schema changes. A policy that sets a negative chronicle or failure-disposition number now gets the default instead of the negative value. For chained driver names, setup now reports one row per CLI family instead of a phantom `-tmux` family.

## Limitations
Zero and negative overrides are still accepted silently at load time instead of being rejected with an error, unlike in the strict `checkpoint` block. The scout's file regrouping and its bridge-manifest facade stay out of scope.
