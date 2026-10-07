# Build Explanation — Cycle 1815

## Build Binding
- Cycle: 1815
- Base SHA: 1058e13fb2e09b1716fa90f7d78dc618b301f88d

## Summary
Four resolver-hygiene defects in `internal/policy` and `internal/setup` are fixed. `WorkflowConfig` now returns copies of `PhaseEnables` and `RemediablePhases` instead of the loaded policy's own map and slice, and it assigns the three remediation knobs once instead of twice. `ChronicleConfig` and `FailureDispositionConfig` ignore negative overrides and keep the compiled defaults. `internal/setup` drops its private one-pass `baseCLI` and calls `policy.BaseCLI`, which strips driver suffixes repeatedly.

## Rationale
A caller that wrote to `WorkflowConfig().PhaseEnables` or `.RemediablePhases` changed the loaded policy and every later resolve, because the result aliased it. The other slice and map fields were already copied, so cloning these two makes the resolver consistent. Deleting the duplicated tail block removes the second aliasing assignment of `RemediablePhases`. A negative digest budget or escalation threshold, step or cap reached the escalation boundary through the `!= 0` guards; a negative cap drives a recurrence weight below zero. Guarding on `> 0` matches the package's existing rule that numeric overrides apply only when positive. Setup's one-pass strip turned `codex-tmux-p` into a phantom `codex-tmux` family in `Detect` and `Recommend`; the shared normalizer already handled chained suffixes.

## Changed Areas
- `go/internal/policy/workflow.go` — clones `PhaseEnables` with `maps.Clone` and `RemediablePhases` with `slices.Clone`, and deletes the duplicated remediation block at the end of `WorkflowConfig`, so a result never aliases the loaded policy.
- `go/internal/policy/policy_chronicle.go` — `digest_tokens` and `digest_cycles` override only when positive, so a negative value resolves to the default.
- `go/internal/policy/policy_disposition.go` — `threshold`, `step` and `cap` override only when positive, so a negative value cannot reach the escalation boundary.
- `go/internal/setup/setup.go` — removes the private `baseCLI` and its `strings` import; `Detect` groups doctor rows by `policy.BaseCLI`.
- `go/internal/setup/recommend.go` — the default-family preference, the cross-family pair and the allowed-family set all normalize through `policy.BaseCLI`.
- `go/internal/policy/remediation_policy_test.go` — adds `TestWorkflowConfig_MutatingResultDoesNotChangeNextResolve`, which mutates one result and checks the next resolve and the loaded policy, and pins that an explicit empty list stays empty and non-nil.
- `go/internal/policy/policy_chronicle_test.go` — adds `TestChronicleConfig_NegativeOverridesResolveToDefaults`, covering negative, zero and small positive overrides.
- `go/internal/policy/policy_disposition_test.go` — adds `TestFailureDispositionConfig_NegativeOverridesResolveToDefaults`, covering negative and small positive overrides.
- `go/acs/cycle1815/predicates_test.go` — the TDD phase's eleven acceptance predicates for this task, carried in the diff.
- `.evolve/evals/policy-resolver-hygiene.md` — the TDD phase's eval with five score caps, carried in the diff.
- `docs/architecture/packages/internal-policy.md` — records that resolvers hand out copies and that the chronicle and disposition numeric overrides apply only when positive.
- `docs/architecture/packages/internal-setup.md` — records that setup normalizes CLI names through `policy.BaseCLI` and keeps no normalizer of its own.

## Design Decisions
`slices.Clone` and `maps.Clone` were chosen over `append([]string(nil), …)` because they keep nil as nil and an explicit empty list as empty, so the resolved values match the old ones exactly except for aliasing; the package already uses `slices.Clone`. Setup calls `policy.BaseCLI` rather than `profiles.BaseCLI`, because `policy.BaseCLI` is the exported normalizer the rest of the runtime uses. Zero still means "unset" for the chronicle and disposition knobs, as before; only negatives change behavior.

## Verification
The three new unit tests fail on the unfixed production files and pass on the fixed ones. All eleven cycle-1815 ACS predicates pass. `gofmt`, `go vet` and `go test -count=1` are clean over `internal/policy` and `internal/setup`, and the module-wide suite was run as well.

## Compatibility
No exported symbol, JSON key or policy schema changes. A policy that sets a negative chronicle or failure-disposition number now gets the default instead of the negative value. Setup reports one row per CLI family for chained driver names, where it previously reported a phantom `-tmux` family.

## Limitations
Zero and negative overrides are still accepted silently at load time instead of being rejected with an error, unlike the strict `checkpoint` block. The file regrouping and the bridge-manifest facade the scout mentioned stay out of scope.
