# flag-campaign-10: autonomous-loop hardening + operational learnings (2026-06-23)

Operational companion to **ADR-0063** (the design-of-record for the integrity hardening) and
**ADR-0061** (the live-feature-flag metric). This doc records the *narrative* — how the campaign
exposed each weakness, the mechanic for running it, and the recurring gotchas — so a future
operator can resume or re-run without rediscovering them.

Goal: drive `len(flagregistry.All)` toward 0 (`no_feature_flags`) by rewiring every `EVOLVE_*`
env reader to policy.json / DI / cobra flag / split-const IPC. Plan: `knowledge-base/research/flag-campaign-plan.json` (v3) + `flag-reduction-47-to-0-design-2026-06-21.md`.

## Trajectory (this session)

| Milestone | rows | live | how |
|---|---|---|---|
| campaign start | 35 | 18 | (post flag-campaign-8 salvage) |
| wave-1 → main (#215) | 30 | 14 | PHASE_RECOVERY, FLEET, FLEET_SCOPE, WORKTREE_ROOT, POLICY_BYPASS |
| wave-2 → main (#217) | 24 | 13 | SYSTEM_PROMPT, ACS_GO_TIMEOUT_S, KB_SEARCH_PATHS, MODEL_CATALOG_DIR, PHASE_ROOTS, CLI_MAX_CONCURRENT_CODEX |
| wave-3 (in flight) | — | — | CLI, CLI_HEALTH, INTENT_DELTA, STRICT_AUDIT |
| remaining | → 0 | → 0 | w3-bootstrap-locators (PROJECT_ROOT×14, PLUGIN_ROOT×7, GO_BIN, WORKTREE_BASE, PROMPTS_DIR), w4-configload-inversion (12), w5-terminal-allowlist (asserts `len==0`), + convergence pass |

## The five escapes → durable fixes (the trust story)

Each weakness shipped a green verdict over a defect; each became a deterministic gate or a reviewed fix:

1. **Wedge** (no bridge liveness) → per-call tmux timeout (#209) + dedicated socket (#210). ADR-0063 §A.
2. **Fake progress** (zero-row ship passed audit) → `flagprogress` gate (#212): rows must strictly decrease. ADR-0063 §B.2.
3. **Broken toolchain** (`go vet` failure = silent recovery-disable, audit missed it) → `buildselfcheck` enforcement (#214): changed pkgs must build/vet/test green. ADR-0063 §B.3.
4. **Semantic cross-process contract** (`EVOLVE_PHASE_RECOVERY_STAGE` renamed but never injected → manual `phase-observer --enforce` silently no-op'd) → caught by **review on the integration PR**; fixed + behavioral test (#216). Gates can't test operator-env on a standalone subprocess.
5. **Per-symbol apicover** (new exported symbol tested cross-package) → caught by **CI apicover-enforce**; fixed with same-package tests. Per-cycle `go test` doesn't run apicover.

**Meta-lesson:** any check that lives *only* in CI lets the autonomous loop ship the gap it covers. Move mechanical-correctness checks into the per-cycle deterministic gate; reserve review for the semantic contracts gates structurally cannot test. (Follow-up: fold `apicover -enforce` + `golangci-lint unused` into `buildselfcheck`.)

## Running the campaign

- **Launch (detached, claude-only, gated):**
  `nohup env EVOLVE_FLAG_CAMPAIGN=1 EVOLVE_CLI=claude-tmux EVOLVE_SANDBOX=on ./go/bin/evolve campaign run --plan knowledge-base/research/flag-campaign-plan.json --project-root <worktree> --concurrency 1 < /dev/null > campaign.log 2>&1 &`
  - `EVOLVE_FLAG_CAMPAIGN=1` activates `flagprogress`; it reaches the gate by plain environment inheritance — the launch sets it on the orchestrator, the host-side `acssuite` passes `os.Environ()` through, and the per-cycle `go test -tags acs` subprocess inherits it (the predicate reads `os.Getenv`).
  - `EVOLVE_CLI=claude-tmux` pins all phases to claude (precedence `EVOLVE_<AGENT>_CLI` > `EVOLVE_CLI` > profile.cli > default); the per-cycle usage-probe still boots agy/codex to *check* quota but never *dispatches* to them. Cross-family auditor≠builder is a preference, not a hard gate — claude-only audit still ships (opus-auditor vs sonnet-builder = model separation).
  - `EVOLVE_SANDBOX=on` (NOT `=1` — grammar is `auto|on|off`); nested-Claude auto-skips the inner sandbox (macOS `sandbox_apply()` EPERM would hang REPL boot).
- **Liveness check during a quiet build:** `campaign.log` only gets a line at phase *transitions*, not per-edit. To tell a working build from a stuck one, check the cycle worktree's file mtimes + `git -C <cycle-worktree> diff --stat` (growing = active), not the log.

## Integration mechanic (branch → main, per wave)

The campaign branch keeps its base while `main` advances (the flagceiling guard tolerates this:
branch live ≤ main live while reducing), so it runs uninterrupted. At each wave boundary:

1. `git checkout -b integ/<wave> main`.
2. Bring the wave's reductions. A plain `git merge --squash <branch>` **conflicts** on the few files where main has fixes the branch lacks (ceiling consts, removed dead code, the phase_observer fix). Cleaner: `git reset --hard main; git checkout <branch> -- .` (branch content is the superset) then `git checkout main -- <main-only fix files>`.
3. **Ratchet the ceilings** (`registry_ceiling_test.go`): `FlagCeiling` and `LiveFeatureFlagCeiling` to the new counts. Cycles never touch these (ceiling-decoupling); the operator ratchets at integration. A row deletion lowers `len(All)`; only an *Active, non-core-infra* deletion lowers live.
4. **Run `apicover -enforce` locally before shipping** (GOTCHA #2): new exported symbols need a *same-package* test naming them. `bin/apicover -cover <go-tool-cover-func-output> <pkg>` (without grep) lists `UNCOVERED  func Name`. Find new exports via `git diff main...HEAD -- <pkg> ':!*_test.go' | grep '^+func|const|var|type [A-Z]'`. Add `apicover_*_test.go` per gap.
5. Regenerate `control-flags.md` (rebuild the binary first — a stale dev binary regenerates the *old* flag count), commit-gate, `evolve ship --class manual`, PR, CI, merge.

## Under-delivery → convergence pass

`flagprogress` requires ≥1 row deleted per cycle, **not** that a multi-flag cycle finishes its
*whole* contract (that would need per-flag manifests). Cycle 16 (`w2-profile-seams`, 3-flag
contract) deleted only SYSTEM_PROMPT, orphaning PERSONA_OVERRIDE + PROFILE_DIR. With multi-flag
cycles ahead (w3-bootstrap=5, w4=12), expect stragglers, and `w5`'s `len==0` would otherwise
become an impossible giant cleanup.

**Plan:** after the planned waves, run a **one-flag-per-cycle convergence pass** for every
remaining flag (one flag *is* its full contract → under-delivery is structurally impossible) until
`len(flagregistry.All)==0`. Known stragglers so far: PERSONA_OVERRIDE, PROFILE_DIR.

## Recurring gotchas

- **SELF_SHA_TAMPERED on every ship after a dev rebuild:** `jq 'del(.expected_ship_sha)' .evolve/state.json` (gitignored, no attestation impact) + re-ship (TOFU re-pins). The dev `go/bin/evolve` also gets deleted by ship — rebuild per worktree as needed.
- **Branch divergence + force-push:** rebasing the campaign branch onto a new main rewrites history → `evolve ship` push is rejected (fast-forward only). The ship-guard (`internal/guards/ship.go`) blocks bare `git push` / `git commit` / `gh release create|edit` — its regex substring-matches those verbs anywhere in a Bash command (so even a `grep "git push …"` pattern trips it; note `gh api` is NOT in the regex). Don't try to force-push around it: rename the branch to a fresh name so it has no diverged remote; the first cycle ship creates origin clean.
- **Plan edits invalidate progress:** editing the plan after launch changes its SHA → `campaign-progress` is dropped → a completed cycle would be re-attempted and FAIL flagprogress (no row left). Remove completed cycles from the plan instead.
- **macOS CI bridge-tmux flake:** the integration-tier tests launch a real `claude` via `sandbox-exec`; on macOS CI they intermittently time out ("REPL prompt never appeared after 60s"). Disjoint from non-bridge changes → rerun the failed job.
- **Stale exact-count acs/cycleN predicates:** each cycle may write a `RegistryRowCountDroppedToN` predicate that goes stale as the campaign reduces below N. CI-excluded (non-blocking), but the auditor's broader acs run flags them — the same retired anti-pattern as PR #162.
- **Doc references lag:** removing/converting a flag leaves prose references in runtime-reference.md / ADRs / CODEBASE-MAP (the *generated* control-flags.md index stays correct automatically). Batch one doc-sweep at campaign end rather than per-wave.
