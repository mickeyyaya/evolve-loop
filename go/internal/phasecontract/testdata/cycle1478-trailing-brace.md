# Audit Report — Cycle 1478

<!-- challenge-token: 24a0236dcf2517c0 -->

audit_bound_tree_sha: 97e833711a70f5e6087d860a5b3f4e0dc4160731

- **Cycle:** 1478 · **goal_hash:** `805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55`
- **Worktree:** `/Users/danleemh/ai/claude/evolve-loop-runtime/.evolve/worktrees/cycle-42824668-1478`
- **Continuation of:** cycle-1465 (`continuation-manifest.json:7`)
- **Challenge token:** `24a0236dcf2517c0` — present in BOTH `scout-report.md:1` and `build-report.md:3` (verified by `grep -c`, 1 hit each). No forgery signal.
- **Ledger:** `scout` (4 entries) and `build`/`builder` entries exist for cycle 1478 in `.evolve/ledger.jsonl` (`grep '"cycle":1478' | jq -r .role | sort | uniq -c`). Build is legitimate.
- **Eval existence (keyed off `scout-report.md ## Selected Tasks`):** `.evolve/evals/context-fill-driver-window-coverage.md` and `.evolve/evals/context-fill-warning-attribution.md` both present in the lane worktree (staged, `git diff --cached --name-only`). Not yet on main — expected for an unshipped lane.
- **Confidence:** 0.90

## Verdict

**WARN** — both committed `top_n` slugs are genuinely delivered, production-wired, and independently verified green; two MEDIUM defects (one report-integrity, one telemetry trust boundary) block a clean PASS. No CRITICAL/HIGH. No eval check fails.

## Evidence table (constitutional citations)

| AC | Status | Evidence (executed by this audit) | Principles |
|---|---|---|---|
| `context-fill-driver-window-coverage`: measured non-claude families yield a finite fill through the REAL resolver | PASS | Ran `go test -tags acs -count=1 ./acs/cycle1478/` → `ok … 1.539s`; predicate `TestC1478_001` drives `tokenusage.DefaultResolver`, not `EffectiveWindow` alone (`go/acs/cycle1478/predicates_test.go:61-71`) | P1, P2 |
| Anti-no-op axis: unsupported/adjacent families stay unmeasured | PASS | `TestC1478_002` asserts `ollama`, `codexx`, `agyx`, `"   "`, `"-"` → 0 and unknown-family resolve → `FillPctUnmeasured` while `Usage.Input` survives (`predicates_test.go:81-107`); table is exact-match (`go/internal/tokenusage/fillpct.go:60-63`) | P1, P8 |
| Claude calibration unchanged by the widening | PASS | `predicates_test.go:103-107` asserts `""`/`claude`/`claude-tmux` → 200000; `isClaudeDriver` still consulted first (`fillpct.go:116-118`) | P1 |
| `context-fill-warning-attribution`: breakdown basis == fill numerator, single-sourced | PASS | Diff hunk `fillpct.go:189-215` — `OccupancyUsage` is the sole owner of the `PeakPromptTokens != 0` rule; `engine.go:702-705` now delegates and no longer re-derives it | P1, P8 |
| The dispatched WARN (not just the seam) carries the basis label — WIRING proof | PASS | Ran `go test -count=1 -v -run TestContextFillWarn_DispatchLineLabels ./internal/bridge` → `--- PASS: …PeakTurnBasis`, `--- PASS: …LaunchTotalBasis`, `ok … 0.166s`. Both drive `Engine.recordTokenUsage` and assert on the CONTEXT-FILL stderr line | P1, P2 |
| Eval graders (both eval files' `evidence:` commands) | PASS | Ran the builder's quoted `-run 'TestFillTelemetry_…\|TestFillWarn_…' ./internal/tokenusage` → `ok … 0.137s` (exit 0, no "no tests to run") | P1, P2 |
| Regression axis (cycles 1456/1458/1462/1465 ACS) | PASS | Ran `go test -tags acs -count=1 ./acs/cycle1458/ ./acs/cycle1462/ ./acs/cycle1465/ ./acs/cycle1456/` → all `ok` (6.154s / 8.352s / 6.281s / 3.225s) | P1 |
| `gofmt` / `go vet` clean on touched packages | PASS | Ran `gofmt -l internal/tokenusage internal/bridge acs/cycle1478` → no output; `go vet ./internal/tokenusage/... ./internal/bridge/...` → no output, exit 0 | P1 |
| Build report's `Files changed` / `Lines added/removed` | **FAIL (M1)** | `build-report.md:192-193` reads `pending` with both POSTHOC markers unexecuted. Ground truth executed here: `git diff --cached --numstat \| wc -l` → **43**; `git diff --cached --shortstat` → `43 files changed, 3699 insertions(+), 133 deletions(-)` | P1, P2 |
| Diff grounding — claims match the tree | PASS with note | `git diff HEAD --stat` → 43 files / 3699 insertions. The 8 paths in the Git Tracking Attestation are all present and correct; the other 35 are the ancestor-cycle carry-in this lane inherited. No claimed change is absent from the diff | P1, P3 |
| Evaluator tamper | PASS | No `package.json`/`Makefile`/test-harness edit in the diff; no test weakened or deleted. `FillWarnWithContributors` preserved by delegation and cycle-1458's ACS package is still green (executed above) | P1, P8 |
| Predicate quality | PASS | All 5 cycle-1478 predicates are native Go driving the exported API in-process; `blocking_count: 0` (`acs-verdict.json .predicate_quality.summary`). No `.sh` predicates exist for this cycle | P1 |
| Self-referential safety | PASS | Change is confined to `internal/tokenusage` + one call site in `internal/bridge`. No agent/skill/`.claude-plugin/` file touched; the loop's own phases dispatched normally through this tree | P1 |

## Defects

| ID | Severity | Class | Location | Root cause |
|---|---|---|---|---|
| M1 | MEDIUM | `posthoc-unresolved` / prescription-misdisposition | `build-report.md:192-193`, `build-report.md:108` | The builder left both POSTHOC markers unexecuted (`Files changed: pending`, `Lines added/removed: pending`) and then DEFERRED the inherited prescription `df5acc71b15d1bc39a0d1c723e04fff78` that names exactly this, on the incorrect ground that the defect "is in cycle-1465's own build-report.md, which this phase cannot amend". **Root cause: a forward-directed prescription was read as a retroactive request against the ancestor's artifact.** The consequence is substantive, not cosmetic: the Git Tracking Attestation attests to 8 paths while 43 are staged (35 undeclared ancestor carry-in), and executing the very POSTHOC that was skipped is what would have surfaced that split — which is the second half of the same prescription. This is the same class as inherited `d1564170130d1bd5aa0ee38bdeb64b379`, now on its second consecutive cycle. Contradicts no prior instinct; reinforces the `consumption/attestation must ride the landing` family. |
| M2 | MEDIUM | telemetry trust boundary / input-validation | `go/internal/core/failure_learning.go:98-109`, `go/internal/bridge/engine.go:729`, `go/internal/core/ports.go:343` | A driver-supplied `PeakPromptTokens` is validated against negatives only and then becomes the ENTIRE fill numerator, discarding the summed `Tokens` cross-check. A transcript emitting `input: 1` records a 98%-occupancy launch as the coldest in the corpus; `input: 10_000_000` stamps `context_window_hot` on every phase and pushes `FireRate.Rate` past `WarnFireRateCeiling`, at which point the correlation report certifies the whole warning class as noise. Independently found by adversarial-review F1 (`adversarial-review-report.md:61-97`). Bounded to MEDIUM: no gate reads the value, so the damage is to operator decisions, not to a ship decision. |
| L1 | LOW | calibration coherence | `go/internal/tokenusage/fillpct.go:36-45` | `codexEffectiveWindow = 200_000` against a 400K advertised window reports ~2× true occupancy (agy: 200K against 1M, ~5×), while the `claudeEffectiveWindow` the comment cites as precedent is AT claude's advertised 200K, not below it — so the three constants do not in fact share the stated rationale. Over-warning is the safe direction for a *warning*, but the same number feeds the fire-rate *ceiling* added in this tree, where over-firing is scored as a defect. The builder flagged the judgement honestly under `build-report.md:200-205`; recorded, not blocking. Matches adversarial-review F3. |
| L2 | LOW | duplicated-rule divergence | `go/internal/contextfill/contextfill.go:48-52` | `occupied := tokens.Input + tokens.CacheRead + tokens.CacheWrite` is unguarded against wrap while its sibling `tokenusage.PromptTokens` (`fillpct.go:78-88`) guards the identical sum. Inherited defect `df92b8875044b89e337a28a6a06152535`, legitimately DEFERRED (different package from both committed slugs) with the concrete fix recorded. Second consecutive cycle open. |

No CRITICAL. No HIGH. `red_count == 0`.

## Inherited defect dispositions

`defect-dispositions.json` re-authored in full this cycle, one entry per inherited id (7/7), all DEFERRED with re-verified reasons. Each was independently confirmed still open rather than taken on the builder's word:

- `d35a625c…` / `d15044c10…` — `firerate_test.go:227-241` still walks up to `go.mod` for the corpus root; no git-common-dir resolution.
- `dce8f8e39…` — `contextfillcorrelate.go:396` still emits one merged insufficient-data message.
- `df92b887…` / `df67fed2c…` — `contextfill.go:48-52` still sums unguarded.
- `d1564170…` / `df5acc71…` — not fixed; the class recurred (M1).

## Plan Adherence (advisory)

- Status: ADVISORY — divergences do NOT affect `acs-verdict.json` or the EGPS verdict.
- `build-plan.md` cited by Builder: no (`build-plan.md` is not present in the workspace).
- Directive adherence: n/a — no plan artifact to compare against.
- Assessment: the build followed `triage-decision.json:3-4` exactly, delivering both committed slugs and declining to widen scope into `internal/contextfill`/`internal/contextfillcorrelate`. Scope discipline was good; the reporting discipline (M1) was not.

## Sibling phase signals

| Phase | Verdict | Carried into this audit |
|---|---|---|
| adversarial-review | WARN (`severity_max: MEDIUM`) | F1 → M2, F2 → L2, F3 → L1. F5 is out-of-diff and excluded. |
| security-scan | PASS | No finding contradicted. |
| telemetry-coverage-check | PASS | No finding contradicted. |
| test-amplification | WARN (`tests_added: 0`) | Reported no `tdd-contract.md` in the workspace; whole-module `go test ./...` PASS. Not a defect against the builder. |

## Hypothesis falsification

The build's stated hypothesis — that 200_000 is a defensible effective window for codex and agy — is falsifiable and unfalsified today (P4). Verification for a later cycle: after ≥20 codex/agy launches, `evolve context-fill correlate` should show their `context_fill_ratio` distribution overlapping claude's rather than sitting systematically ~2× higher. If codex/agy launches routinely read >100%, the constants are what to revisit, not the mechanism.

## Reflection
<!-- BEGIN reflection -->

### What slowed this phase (required)

- `artifact-misplacement`: `acs-verdict.json` was written by the build into the LANE worktree's `.evolve/runs/cycle-1478/`, not the real workspace, so the workspace listing showed it absent. Cost a `find` across the worktree before the suite numbers could be read. Re-emitted into the workspace here with the predicate-quality block added.
- `no intent.md in the workspace`: the intent link of the reasoning chain had to be answered from `triage-decision.json` + `scout-report.md ## Selected Tasks` instead, and is recorded `unverifiable` rather than `coherent` for that reason.

### Pipeline friction received from upstream (required)

- The preseeded `defect-dispositions.json` arrived with all 7 ids at `status: "OPEN"` — a status the schema does not accept from this phase. Re-authoring it in full is the documented contract, but the preseed's shape is one edit away from being mistaken for a valid file.

### Suggested improvement for next cycle (required)

- Make the build phase's POSTHOC markers a gate rather than a convention: a `build-report.md` shipping the literal token `pending` next to an unexecuted `<!-- POSTHOC:` marker is mechanically detectable and has now recurred across two consecutive cycles despite an explicit prescription.

### Reflection confidence (required)

- `confidence: 0.9`

<!-- END reflection -->

<!-- evolve-audit-chain
intent-fidelity | unverifiable | triage-decision.json:3-4 | no intent.md exists in the workspace; the committed top_n reads as a faithful narrowing of the fleet-scoped inbox item, but the intent artifact itself could not be opened
selection-fidelity | coherent | triage-decision.json:3-4 | top_n names exactly the two slugs scout-report.md ## Selected Tasks proposes, and both have eval files on disk
specification-fidelity | coherent | go/acs/cycle1478/predicates_test.go:76-108 | TestC1478_002 is a real anti-no-op axis: a fix mapping every named driver to some window passes 001 and fails here
implementation-fidelity | coherent | go/internal/tokenusage/fillpct.go:189-215 | the code satisfies the predicates as written; no test was relaxed, and FillWarnWithContributors was preserved by delegation with cycle-1458's ACS package still green
narrative-fidelity | incoherent | build-report.md:192-193 | the report leaves two POSTHOC metrics unexecuted as 'pending' and attests to 8 staged paths where 43 are staged; the ancestor prescription naming exactly this was deferred on an incorrect rationale
delivery-fidelity | coherent | go/internal/bridge/engine.go:702-705 | the change lands at the single production chokepoint the intent targets, not an adjacent seam; the duplicated basis conditional is gone from engine.go
evidence-fidelity | coherent | acs-verdict.json:1 | the cited gate results were reproduced by this audit against these bytes: acs/cycle1478 + 1456/1458/1462/1465 all ok, eval graders ok, gofmt and vet clean
-->

<!-- evolve-verdict: {"phase":"audit","verdict":"WARN","schema_version":2,"failure":{"class":"code-audit-fail","defects":["M1 MEDIUM posthoc-unresolved: build-report.md:192-193 leaves both POSTHOC markers unexecuted as 'pending' (ground truth: 43 files, 3699 insertions, 133 deletions) and the Git Tracking Attestation declares 8 staged paths where 43 are staged; inherited prescription df5acc71b15d1bc39a0d1c723e04fff78 naming exactly this was DEFERRED on the incorrect ground that it targets cycle-1465's report","M2 MEDIUM telemetry trust boundary: driver-supplied PeakPromptTokens is validated against negatives only and then used as the entire fill numerator, discarding the summed Tokens cross-check, so a lying transcript can falsify context_window_hot in both directions and push FireRate past WarnFireRateCeiling (go/internal/core/failure_learning.go:98-109, go/internal/bridge/engine.go:729)","L1 LOW calibration coherence: codexEffectiveWindow=200000 against a 400K advertised window reports ~2x true occupancy (agy ~5x) while the claudeEffectiveWindow cited as precedent sits AT claude's advertised 200K, and the same number feeds the fire-rate ceiling where over-firing is scored as a defect (go/internal/tokenusage/fillpct.go:36-45)","L2 LOW duplicated-rule divergence: contextfill.FillRatio sums Input+CacheRead+CacheWrite unguarded against wrap while tokenusage.PromptTokens guards the identical sum (go/internal/contextfill/contextfill.go:48-52); inherited df92b8875044b89e337a28a6a06152535, second consecutive cycle open"],"evidence_paths":[".evolve/runs/cycle-1478/build-report.md",".evolve/runs/cycle-1478/adversarial-review-report.md",".evolve/runs/cycle-1478/acs-verdict.json",".evolve/runs/cycle-1478/defect-dispositions.json","go/internal/tokenusage/fillpct.go","go/internal/contextfill/contextfill.go"]},"prescription":["Execute the two POSTHOC markers in build-report.md's tracking section and state the delivered-versus-inherited staged split (8 delivered vs 43 staged) at that line; treat an inherited PRESCRIPTION as a forward directive to this cycle's own artifact, never as a request to amend the ancestor's","Cross-check driver-supplied PeakPromptTokens against the summed Tokens envelope before using it as the fill numerator — reject or fall back to the launch total when the peak turn exceeds the summed prompt side, closing the cheap single-integer forgery","Delegate contextfill.FillRatio's prompt-side sum to tokenusage.PromptTokens so the wrap guard is single-sourced, closing df92b8875044b89e376a68a6a06152535's remaining half","Reconcile the effective-window constants against a stated fraction of each family's advertised window (or document why claude sits at 100% of its advertised 200K while codex sits at 50%), and decide whether WarnFireRateCeiling should normalise for that fraction"]}} -->
