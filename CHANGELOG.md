# Changelog

All notable changes to this project will be documented in this file.

## [8.13.2] - 2026-04-27

### Self-healing release pipeline

The /insights audit flagged "release_publish" as the #1 friction class — silent stale-marketplace failures, ambiguous "publish" semantics, cache-refresh ordering bugs, no rollback on failure. v8.13.2 introduces a single declarative entry point that owns the entire release lifecycle and auto-rolls-back on any post-push failure.

### Added

- **`scripts/release-pipeline.sh`** (~280 lines) — top-level orchestrator. Sequences pre-flight → bump → changelog → consistency-check → ship → marketplace-poll → cache-refresh. On any post-push failure, auto-invokes `rollback.sh` (deletes GitHub release + remote tag, creates revert commit). Writes a per-publish journal at `.evolve/release-journal/<version>-<ts>.json` for rollback to consult. Supports `--dry-run`, `--no-rollback`, `--skip-tests`, `--max-poll-wait-s`.
- **`scripts/release/preflight.sh`** (~190 lines) — pre-flight gate. Checks: clean working tree, branch attached, target-version is a valid semver bump, audit ledger has a recent (<7d) PASS verdict, all four gate-test suites green.
- **`scripts/release/version-bump.sh`** (~140 lines) — atomic version updater for `plugin.json`, `marketplace.json` (`.plugins[0].version` path), `SKILL.md` heading, `README.md` "Current" + history row. Idempotent.
- **`scripts/release/changelog-gen.sh`** (~150 lines) — conventional-commits parser. Buckets `feat:`/`fix:`/`refactor:`/`perf:`/`docs:` plus an explicit `### Other` fallback for the ~40% of historical commits without prefixes. Idempotent — preserves manually-curated entries.
- **`scripts/release/marketplace-poll.sh`** (~150 lines) — post-publish marketplace propagation verifier. Polls `~/.claude/plugins/marketplaces/evolve-loop/` for up to 5 minutes (configurable via `--max-wait-s`). On convergence, re-invokes `release.sh <target>` to refresh `installed_plugins.json` registry. **Closes the cache-refresh ordering bug** by sequencing release.sh-refresh AFTER convergence, never before.
- **`scripts/release/rollback.sh`** (~190 lines) — auto-revert. Reads release journal. Three independently-auditable steps: (a) `gh release delete vX.Y.Z`, (b) `git push origin :refs/tags/vX.Y.Z`, (c) revert commit pushed via `EVOLVE_BYPASS_SHIP_VERIFY=1 bash scripts/ship.sh "revert: ..."`. Logs every step to `.evolve/release-rollbacks.jsonl` for audit trail.
- **`docs/release-protocol.md`** (~250 lines) — canonical vocabulary doc. Defines push, tag, release, propagate, publish, ship. Operational runbook with annotated examples. Conventional-commits guide. Marketplace topology diagram. Common failure modes table.
- **5 new test suites** (54 tests total): `preflight-test.sh` (10), `changelog-gen-test.sh` (14), `marketplace-poll-test.sh` (10), `rollback-test.sh` (8), `release-pipeline-test.sh` (12). Includes explicit regression tests for **the cache-refresh ordering bug** (Test 9 of release-pipeline-test) and **the stale-version regression** (Test 3 of marketplace-poll-test, Test 10 of release-pipeline-test).

### Changed

- **`agents/evolve-orchestrator.md`** — verdict→PASS branch now invokes `release-pipeline.sh <new-version>` for version-bumping releases (vs. direct `ship.sh` for non-release commits).
- **`skills/evolve-loop/SKILL.md`** — Phase 5 description points at `release-pipeline.sh` as the canonical publish entry point. New v8.13.2 callout in the front-matter.
- **`.evolve/profiles/orchestrator.json`** — allows `Bash(bash scripts/release-pipeline.sh:*)` and the five `scripts/release/*.sh` components.
- **`CLAUDE.md`** — replaced "Release Checklist" section with "Release & Publish Workflow" pointing at the protocol doc and the pipeline.

### Documentation

- New `docs/release-protocol.md` is the canonical answer to "what does publish mean?"

### Test Results

131/131 pass: 54 new (preflight 10 + changelog-gen 14 + marketplace-poll 10 + rollback 8 + release-pipeline 12) + 77 v8.13.x regression suites (role-gate 21 + phase-gate-precondition 15 + guards 34 + ship-integration 7).

### Architectural pattern (consistent with v8.13.0/v8.13.1)

Every component:
- Has a `--dry-run` mode that mutates nothing.
- Is independently testable (each ships with its own unit-test suite).
- Composes through a thin top-level orchestrator (no logic duplication).
- Calls existing `ship.sh` for the actual atomic git ops (the v8.13.0 audit-binding contract is preserved unchanged).

### v8.13.2 audit RC1 follow-ups (incorporated before ship)

- **MEDIUM-1 fix**: `scripts/release/rollback.sh` now exits 1 (not 0) when any cleanup step (`gh release delete`, remote tag delete) explicitly fails, even if the revert commit succeeds. Pre-fix logic only consulted step3 — masking dangling-release incidents during partial-failure rollbacks. Regression test added (`rollback-test.sh` Test 9): simulates `gh release delete` returning non-zero, asserts exit 1 and a journal entry with `release_delete:"failed"`.
- **LOW-1**: preflight.sh `parse_semver` discards `[+-]suffix` (e.g., `1.0.0-rc1` → "1 0 0"). Project does not use pre-release versions; deferred. Documented as design choice.
- **LOW-2**: marketplace-poll.sh swallows `git fetch`/`git reset --hard` errors. Trade-off: retry-on-transient-failure semantics; deferred.

### Out of scope (deferred to v8.13.3 / v8.14.x)

- CDN-based marketplace propagation (current is git-based local).
- Cross-machine cache invalidation.
- Auto-incrementing semver from commit types.
- Pre-release / RC channels (`vX.Y.Z-rc1`) — preflight.sh semver fix needed if adopted.
- Slack/email notifications on rollback.

---

## [8.13.1] - 2026-04-27

### Trust boundary completed: role-gate + phase-gate-precondition + run-cycle.sh

v8.13.0 shipped the atomic ship-gate (`scripts/ship.sh` allowlist on Bash). v8.13.1 adds the two remaining structural gates and a declarative cycle driver, completing the trust-boundary the user originally requested ("orchestrator should only execute phases, can't interfere"):

- **`scripts/guards/role-gate.sh`** (NEW, ~190 lines) — PreToolUse hook on `Edit|Write`. Reads `.evolve/cycle-state.json` and enforces a per-phase write allowlist:
  - `calibrate`/`research`/`discover` → workspace only
  - `build` → workspace + active worktree
  - `audit` → audit-report.md + handoff-auditor.json
  - `ship` → version-bump files only (plugin.json, marketplace.json, CHANGELOG.md, README.md, SKILL.md)
  - `learn` → orchestrator-report.md + lessons/*.yaml + state.json

  Always allows `/tmp/`, `/var/folders/`, `$HOME/.claude/`. Transparent passthrough when no cycle is in progress. Bypass: `EVOLVE_BYPASS_ROLE_GATE=1`.

- **`scripts/guards/phase-gate-precondition.sh`** (NEW, ~150 lines) — Second PreToolUse hook on Bash matcher (after ship-gate). Triggers ONLY for `bash scripts/subagent-run.sh <agent> ...` invocations. Reads cycle-state.phase and denies invocations that don't match the expected next agent (Scout→Builder→Auditor sequence). Bypass: `EVOLVE_BYPASS_PHASE_GATE=1`.

- **`scripts/run-cycle.sh`** (NEW, ~170 lines) — Convenience driver. Picks next cycle ID, initializes cycle-state.json, spawns orchestrator subagent under its profile-restricted permissions. Includes `--dry-run` for inspection. Cleans up cycle-state on exit (success or failure).

- **`scripts/cycle-state.sh`** (NEW, ~150 lines) — Helpers (`init`/`advance`/`set-agent`/`get`/`clear`/`exists`/`dump`/`path`) for `.evolve/cycle-state.json`. Atomic mv-of-temp-file updates. Schema: cycle_id, phase, started_at, phase_started_at, active_agent, active_worktree, completed_phases, workspace_path.

- **`agents/evolve-orchestrator.md`** (NEW, ~80 lines) — Orchestrator subagent prompt. Sequences Scout→Builder→Auditor; on PASS invokes ship.sh; on FAIL/WARN invokes record-failure-to-state.sh (no inline retrospective per v8.12.3 design pivot). Cannot edit source code, commit, or push (kernel hooks block).

### Tests

- `scripts/role-gate-test.sh` — 20 tests covering all phases + always-safe paths + bypass + malformed state + relative-path resolution.
- `scripts/phase-gate-precondition-test.sh` — 15 tests covering trigger detection, per-phase allowlist, re-spawn, no-cycle passthrough, bypass.
- `scripts/guards-test.sh` — 34 tests (unchanged from v8.13.0 RC5).

Combined: **69/69 unit tests pass**.

### Modified

- `.claude/settings.json` — wires role-gate.sh on `Edit|Write`, adds phase-gate-precondition.sh as second hook on Bash matcher.
- `.evolve/profiles/orchestrator.json` — allows `Bash(bash scripts/cycle-state.sh:*)` and `Bash(bash scripts/run-cycle.sh:*)`. Updated `_design_notes` (no longer "scaffolding only").
- `skills/evolve-loop/SKILL.md` — documents `bash scripts/run-cycle.sh` as the declarative alternative.
- `skills/evolve-loop/phases.md` — documents cycle-state.json + the three kernel hooks at every transition.

### Removed (branch cleanup)

- `feat/orchestrator-as-subagent` — superseded by v8.13.0 (clean rewrite). 1 unmerged WIP commit `8bc2759` discarded.
- `feat/v8.13.0-ship-gate` — already merged into main as `ca5a5e3`. Local cleanup only.

### Why this is the right v8.13.1

The trust-boundary the user requested at session start is now structurally enforced by allowlist-shaped gates (parser-bypass arms races converge on enumeration, allowlists converge on canonical paths):

- **ship-gate (v8.13.0)** — canonical-script-allowlist
- **role-gate (v8.13.1)** — path-allowlist, per-phase
- **phase-gate-precondition (v8.13.1)** — sequence-allowlist

The architectural pattern is now consistent across all three gates. Same test discipline as v8.13.0: every gate ships with a unit-test suite that exercises both ALLOW and DENY paths plus the bypass env var.

### v8.13.1 audit RC1 follow-ups (incorporated before ship)

- **MEDIUM-1 fix**: orchestrator profile narrowed from `Bash(bash scripts/cycle-state.sh:*)` to specific subcommands `{advance, set-agent, get, exists, dump, path}`. The `init` and `clear` subcommands are now in `disallowed_tools`, preventing the orchestrator subagent from silently disabling the v8.13.1 gates by removing `cycle-state.json`. `run-cycle.sh`'s parent process owns the init/clear lifecycle.
- **LOW-2 fix**: `role-gate.sh` strips trailing slash from `ACTIVE_WT` and `WORKSPACE_PATH` after reading from cycle-state.json. Defensive trim against caller-supplied trailing-slash paths. New unit test (Test 21) covers this.
- **LOW-1**: canonicalize() docstring is cosmetic; deferred (no behavior change).
- **LOW-3**: run-cycle.sh dry-run log message wording is cosmetic; deferred.
- **LOW-4**: malformed cycle-state fail-open is intentional graceful degradation; cycle-health-check audit-time assertion deferred to v8.13.2.

### Out of scope (deferred to v8.13.2 / v8.14.0+)

- Cycle-state staleness detection (interrupted cycles leaving cycle-state.json behind).
- Cross-machine state sync (single-machine assumption holds).
- True orchestrator-as-subprocess (run-cycle.sh delegates to a `claude -p` subagent that itself spawns more subprocesses; this works but adds 1 layer of subprocess overhead).
- Audit-time check that "if cycle is in progress, cycle-state.json must parse" (LOW-4 follow-up).

---

## [8.13.0] - 2026-04-27

### **BREAKING — atomic ship-gate via canonical `scripts/ship.sh`**

After 5+ audit cycles of parser-bypass arms races (cycles 8121-8129), v8.13.0 reframes the problem. Instead of detecting ship-class commands (git commit / git push / gh release create) inside arbitrary bash via increasingly clever parsers, the gate now allowlists exactly ONE canonical script: `scripts/ship.sh`. The gate's check is trivially simple: "does this command's first executable, resolved via realpath, equal `scripts/ship.sh`?" If yes, allow (ship.sh enforces the audit-first contract internally). If no AND the command contains ship verbs, deny.

**This kills the parser-bypass arms race.** D1-D3 from cycle 8122 (bare-newline, pipe-to-shell, here-string), D6 (commit→push workflow regression), and D-NEW-1 from cycle 8130 (bash -c bypass) are all resolved.

### Breaking change for users

Raw `git commit`, `git push`, and `gh release create` invocations are now **denied** by the ship-gate hook (`scripts/guards/ship-gate.sh`, wired via `.claude/settings.json`).

**To ship a commit:**

```bash
bash scripts/ship.sh "<commit-message>"
# Optionally with release notes for a GitHub release:
EVOLVE_SHIP_RELEASE_NOTES="$(cat NOTES.md)" bash scripts/ship.sh "<commit-message>"
```

**Emergency bypass:**

```bash
EVOLVE_BYPASS_SHIP_GATE=1 git commit -m "<msg>"     # bypasses the gate
EVOLVE_BYPASS_SHIP_VERIFY=1 bash scripts/ship.sh    # bypasses ship.sh's internal audit checks
```

Both bypasses are logged with explicit WARN.

### Added — `scripts/ship.sh` (canonical atomic shipper, ~190 lines)

Enforces the full audit-first contract before any git operation:

1. **TOFU self-SHA verification** — pins ship.sh's SHA256 in `.evolve/state.json` on first run; subsequent modifications fail loudly. To intentionally update ship.sh, remove `state.json:expected_ship_sha` first.
2. **Latest auditor ledger entry** — must exist with `exit_code: 0`.
3. **Audit-report SHA match** — recorded SHA256 must equal current file SHA256 (catches post-audit mutation).
4. **Verdict: PASS** in the audit report (word-anchored regex, no PASSABLE/PASSTHROUGH false matches).
5. **Cycle binding** (the H2 fix) — current `git rev-parse HEAD` and `sha256(git diff HEAD)` must match the ledger entry's `git_head` and `tree_state_sha`. Refuses to ship work that wasn't audited.
6. **Audit freshness** — 7-day cap.
7. **Atomic commit + push** (+ optional `gh release create`) — all in one bash invocation; the gate fires once on the outer call.

### Added — `scripts/guards/ship-gate.sh` (simplified PreToolUse hook, ~170 lines)

- **Step 1**: realpath-resolves first executable; allows ship.sh canonical path.
- **Step 1.5**: detects `bash -c "..."`, `sh -c "..."`, `eval "..."` patterns and recursively checks the inner snippet (D-NEW-1 fix from cycle 8130).
- **Step 2**: regex-checks for ship verbs at tokenizable boundaries; awk pre-processor strips heredoc bodies first (no false positives on build reports that mention "git commit" in their text).

### Added — telemetry tests (34 total)

- `scripts/guards-test.sh` (19 unit tests): canonical-path allow, ship verbs blocked across raw/chained/subshell/pipe/heredoc/eval forms.
- `scripts/ship-integration-test.sh` (7 end-to-end tests in temp git repos): no-ledger refusal, PASS audit succeeds, WARN refusal, tree-state-mismatch refusal, HEAD-moved refusal, ship.sh self-SHA tampering refusal, bypass switch.
- `scripts/subagent-run-test.sh`: extended to validate the new orchestrator profile (12/12, was 11/11).

### Added — `.evolve/profiles/orchestrator.json` (orchestrator subagent profile, scaffolding)

The orchestrator-as-subagent vision from `feat/orchestrator-as-subagent` (commit 8bc2759, parked since cycle 8121) lands as scaffolding. Without v8.13.1's `scripts/run-cycle.sh` deterministic driver, this profile is not yet invoked by anything. It provides the M2-fix interpreter denials when the spawner ships:

- Disallowed: `Bash(perl:*)`, `Bash(perl -e:*)`, `Bash(ruby:*)`, `Bash(ruby -e:*)`, `Bash(python -c:*)`, `Bash(python3 -c:*)`, `Bash(node -e:*)`, `Bash(osascript:*)`, `Bash(sh -c:*)`, `Bash(bash -c:*)`, `Bash(zsh -c:*)`, `Bash(env:*)`, `Bash(exec:*)`, `Bash(eval:*)`, `Bash(awk:*)`.
- Sandbox: `read_only_repo: true`, allow_network: true (orchestrator may need to reach Anthropic API for sub-subagent invocation).
- Allowed: `Bash(bash scripts/ship.sh:*)`, `Bash(bash scripts/subagent-run.sh:*)`, narrow git read commands (status, log, diff, show, ls-files, rev-parse, branch, stash list, worktree list).

### Changed — cycle-binding fields in ledger

`scripts/subagent-run.sh` now captures `git_head` (= `git rev-parse HEAD`) and `tree_state_sha` (= sha256 of `git diff HEAD`) at agent-start time and emits both in every `agent_subprocess` ledger entry. ship.sh uses these to enforce the cycle-binding contract.

### Audit history

| Cycle | Verdict | Defects |
|---|---|---|
| 8121 | FAIL | H1 parser bypasses (≥6 classes), H2 no cycle binding, M1 dead code, M2 interpreter holes |
| 8122 | FAIL | D1 bare-newline, D2 pipe-to-shell, D3 here-string, D5 backwards-compat, D6 commit→push workflow regression |
| 8126 (RC3 TMPDIR) | FAIL | TMPDIR override empirically non-functional |
| 8127 (RC4 worktree) | FAIL | Auto-worktree had three failure modes |
| 8128 (RC5 reverted) | PASS | Shipped v8.12.3 with EPERM as known issue |
| 8129 (v8.12.4) | PASS | Sandbox /tmp read fix |
| 8130 (RC1) | FAIL | D-NEW-1 bash -c bypass, D-NEW-2 missing CHANGELOG, D-NEW-3 atomic-ship doc, D-NEW-4 verdict regex word anchor |
| **8131 (RC2)** | **PASS** | All 4 D-NEW defects fixed; 19 guards-test + 7 ship-integration-test all pass |

### Migration

This is the FIRST release that uses ship.sh to ship itself. The `.claude/settings.json` is created in this cycle with the PreToolUse hook wiring; first run after merge will set up the TOFU self-SHA pin in `.evolve/state.json`.

For existing v8.12.4 users:
- Pull the new code; the next `git commit` will be denied unless via ship.sh
- One-time setup: run any audit cycle, then `bash scripts/ship.sh "<msg>"` — it'll pin its own SHA on first run
- The ship-gate is project-scoped via `.claude/settings.json` — only fires inside this repo's working directory

### Out of scope (deferred to v8.13.1+)

- `scripts/run-cycle.sh` — deterministic driver that sequences phases and spawns the orchestrator subagent. Without it, the orchestrator profile shipped here is scaffolding only.
- `scripts/guards/role-gate.sh` — block Edit/Write outside cycle workspace mid-cycle.
- `scripts/guards/phase-gate-precondition.sh` — block out-of-order phases.
- Skill rewrite for `/evolve-loop` to invoke the orchestrator subagent.

## [8.12.4] - 2026-04-27

### Fixed — EPERM on bash tool output files (5-cycle hunt resolved)

The macOS sandbox-exec profile in `scripts/cli_adapters/claude.sh` was missing read permissions on the tmp paths it allowed writes to. When `claude -p` wrote a bash tool's output to `/tmp/claude-${UID}/<project>/<session>/tasks/<id>.output` (write permitted), then attempted to read it back to return the result to the LLM, the read was denied — surfacing as `EPERM`.

The model frequently misread this as "another Claude Code process deleted the file during startup cleanup" — which led to two failed attempts in v8.12.3 (TMPDIR override RC3, auto-worktree RC4) targeting concurrent-session collision instead of the actual sandbox gap. Both reverted in v8.12.3 RC5.

**Root cause confirmed empirically** by inspecting `/tmp/claude-501/` directly — concurrent sessions already get unique session-UUID subdirs. Then `grep "(allow file-read.*tmp" claude.sh` returned zero matches. The bug was 100% sandbox-side.

**Fix**: 4 new `(allow file-read* (subpath ...))` rules added before the existing write rules in `generate_macos_sandbox_profile()`:

```scheme
(allow file-read* (subpath "/tmp"))
(allow file-read* (subpath "/private/tmp"))
(allow file-read* (subpath "/var/folders"))
(allow file-read* (subpath "/private/var/folders"))
```

### Empirical impact on audit latency

The cycle 8129 audit was the first v8.12.x audit where bash commands actually worked. Comparison:

| Audit | Bash works | Turns | Duration | Cost |
|---|---|---|---|---|
| Cycle 8128 (v8.12.3) | ✗ EPERM | 27 | 188s | $0.57 |
| **Cycle 8129 (v8.12.4)** | ✅ | **15** | **119s** | **$0.38** |

**44% fewer turns, 37% faster, 33% cheaper** — without changing token counts or audit prompts. The auditor no longer has to compensate for missing bash by individually reading every source file; it can grep and run scripts directly.

### What this resolves

- All v8.12.x audits (cycles 8121, 8121-v8121, 8122, 8124, 8126, 8127, 8128) reported variations of "Bash command execution was blocked by EPERM" / "another Claude Code process deleted it during startup cleanup". All now obsolete.
- The "known issue, deferred to v8.12.4" footnote in v8.12.3's CHANGELOG and `subagent-run.sh:295-313` docstring now have a fix shipped.

### Audit

Cycle 8129 (Sonnet, 119s, 15 turns, $0.38). Verdict: **PASS**. The empirical EPERM test passed: 3 independent bash calls succeeded with zero EPERM. One LOW finding (auditor profile doesn't include `Bash(bash scripts/subagent-run-test.sh:*)` in allowlist, so the test-run criterion was unverifiable — bash itself works, the gap is profile scope, not EPERM). No MEDIUM+ defects.

### Note

The 5-cycle journey (RC3 TMPDIR → RC4 auto-worktree → RC5 revert → 8128 PASS without fix → v8.12.4 actual fix) is itself a documented case of how an error message can mislead the diagnostic process. The audit reports' consistent "concurrent process deleted file" framing was a hallucinated explanation that anchored two iterations of work toward the wrong layer. The fix only became visible after both bad fixes were reverted AND the sandbox profile was inspected directly.

## [8.12.3] - 2026-04-27

### Added — Token economy: subagent prompt prefix reduced ~75%

Six profile JSONs (`scout`, `builder`, `auditor`, `inspirer`, `evaluator`, `retrospective`) now pass `--disable-slash-commands`, `--setting-sources`, `project` to `claude -p`. These flags drop the 227 user-level skills' metadata (~50–90K tokens) from every subagent's system prompt. None of the subagents grant the `Skill` tool in their allowed_tools, so they couldn't invoke `/skill-name` anyway — the metadata was pure overhead.

**Empirically verified** (cycle 8124 audit):
- `cache_creation_input_tokens`: 135,173 → 45,424 (66% reduction)
- `cache_read_input_tokens`: 2,414,580 → 529,642 (78% reduction)
- Audit duration: 625s → 162s (74% faster on Sonnet)

### Added — Subagent telemetry sidecars

After every successful agent run, `scripts/subagent-run.sh` writes two sidecar files into the workspace:

- **`${agent}-usage.json`** — extracts `usage`, `modelUsage`, `duration_ms`, `num_turns`, `total_cost_usd` from the agent's stdout JSON. Future audits can self-verify empirical token effects without parsing their own stdout.
- **`${agent}-timing.json`** — per-phase ms breakdown (`profile_load_ms`, `prep_total_ms`, `adapter_invoke_ms`, `finalize_ms`, `total_ms`). Bash 3.2-compatible (uses temp file, not `declare -A`). Confirmed empirically that 99.9% of subagent runtime is the `claude -p` API call; pure runner overhead is ~110ms.

### Added — Lightweight failure recording (design pivot from v8.12.2)

Per user direction, the `evolve-retrospective` subagent (shipped in v8.12.2) is **no longer invoked per-cycle**. Instead, when an audit returns FAIL/WARN/SHIP_GATE_DENIED:

1. Capture `git diff HEAD > $WORKSPACE_PATH/failed.patch` (forensic)
2. Run `scripts/record-failure-to-state.sh $WORKSPACE_PATH $VERDICT` — extracts audit defects (severity + title) from `audit-report.md`, captures cycle/git-head/tree-state SHA + audit-report SHA256, appends a structured entry to `state.json.failedApproaches[]` with `retrospected: false`. **Total cost: ~50ms shell, no LLM calls.**
3. `git worktree remove --force` discards the failed code.

The retrospective subagent runs separately in **batches** (on-demand or scheduled), synthesizing cross-cycle patterns from accumulated `failedApproaches` entries. This is more useful than per-cycle retrospectives because failure patterns ("3rd parser bypass this month") only emerge from multiple data points.

**Net change**: per-FAIL cycle saves ~$0.50 + 3-5 minutes. Forensic information preserved (audit-report.md + failed.patch + state.json entry).

### Changed — Phase docs

- **`skills/evolve-loop/phase6-learn.md`** § 4c rewritten: documents the lightweight inline recording flow + the deferred batch-retrospective pattern. Heading enumerates `FAIL / WARN / SHIP_GATE_DENIED` (no longer ambiguous "FAILED tasks only").
- **`skills/evolve-loop/phases.md`** audit-to-ship branch updated: PASS → ship; FAIL/WARN/SHIP_GATE_DENIED → drop + record-failure (lightweight, no subagent). Reference to phase6-learn.md § 4c for the deferred-retrospective flow.

### Known issue — deferred to v8.12.4

**EPERM on `/tmp/claude-${UID}/...` task output files** during concurrent claude-session collisions. Three approaches investigated and rejected:
- v8.12.3 RC3 TMPDIR-override: empirically non-functional (Claude Code uses hardcoded `/tmp/claude-${UID}/`, not `${TMPDIR}/`).
- v8.12.3 RC4 auto-worktree (per-subagent unique cwd): empirically non-functional (cwd-hash alone doesn't prevent cleanup; broke profile's relative-path Write patterns; showed worktree HEAD instead of orchestrator's working tree).

EPERM remains an open issue. Documented in `scripts/subagent-run.sh:295-313` with the failed approaches enumerated. Next investigation should target the actual cleanup trigger (likely `.claude/` directory location or parent PID, not cwd).

### Audit

- RC1 (cycle 8124, Sonnet): PASS — empirical token reduction verified.
- RC2 (cycle 8125, attempted): build progressed; audit aborted by user.
- RC3 (cycle 8126, Sonnet): FAIL — TMPDIR fix non-functional. Reverted.
- RC4 (cycle 8127, Sonnet): FAIL — auto-worktree fix had three failure modes. Reverted.
- RC5 (cycle 8128, Sonnet): **PASS** — auto-worktree reverted, all 8 acceptance criteria met, 0 MEDIUM+ defects, 2 LOW non-blocking notes (criterion 2 grep count off by 1 in build template; criterion 7 empirically unverifiable due to EPERM). Recommended ship.

### Latency analysis

Empirical phase-timing from the cycle 8128 audit (`auditor-timing.json`):
```
profile_load_ms:      14
prep_total_ms:        57
adapter_invoke_ms: 187,769  ← 99.9% of wall-clock
finalize_ms:          39
total_ms:         187,879
```

The runner overhead is ~110ms; everything else is the `claude -p` API call. To reduce audit latency further, the lever is **turn count** (~27 in this audit) — pre-running tests in the orchestrator and inlining results would cut to ~10 turns. Deferred to v8.12.4 / v8.13.0.

## [8.12.2] - 2026-04-27

### Added — Failed-cycle retrospective + lessons-learned pipeline

- **`agents/evolve-retrospective.md`** — new failure post-mortem subagent. Fires only on Auditor `FAIL`/`WARN` or `SHIP_GATE_DENIED`. Reads cycle artifacts (audit-report, build-report, scout-report, the failed diff) and writes a structured retrospective + one or more failure-lesson YAMLs into `.evolve/instincts/lessons/`. Read-only outside the lesson directory; cannot mutate state.json, ledger, profiles, or personal instincts.
- **`.evolve/profiles/retrospective.json`** — least-privilege permission profile. `read_only_repo: true`, `allow_network: false`. Allowed writes: only the retrospective report, handoff JSON, and lesson YAMLs. Disallows all source code mutation, git commit/push/release, all interpreter `-c`/`-e` flags, and network tools (WebFetch, WebSearch, curl, wget). Sandboxed via macOS sandbox-exec or Linux bwrap.
- **`scripts/merge-lesson-into-state.sh`** — orchestrator post-processor. Reads `handoff-retrospective.json`, appends new lesson IDs to `state.json.instinctSummary[]` (so future Scout/Builder/Auditor agents see them in their context block via the existing channel), appends a structured `failedApproaches[]` entry, and emits a `SYSTEMIC_FAILURE` ledger event when the retrospective flagged 3+ same-error-category failures across recent cycles. The retrospective profile cannot mutate state.json directly — this helper runs under orchestrator permissions.
- **`scripts/merge-lesson-test.sh`** — 7-check smoke test (no-op on missing handoff, single-lesson merge, failedApproaches append, missing-YAML integrity exit 2, SYSTEMIC_FAILURE ledger event, malformed-handoff exit 1, multi-lesson merge).
- **`.evolve/instincts/lessons/.keep`** — directory marker so the lessons directory ships with the plugin. Per-project lesson YAMLs (`inst-L*.yaml`) remain gitignored.
- **`scripts/subagent-run.sh`**: registered `retrospective` as a recognized agent role. Updated header docs and usage text.
- **`scripts/subagent-run-test.sh`**: extended Test 1 to validate the retrospective profile (now 11/11 from 10/10).

### Changed — Skill docs document the explicit FAIL branch

- **`skills/evolve-loop/phase6-learn.md`** § 4c rewritten to invoke the `evolve-retrospective` subagent (mirroring Scout/Builder/Auditor pattern) instead of doing classification inline. Documents lesson schema, error-category taxonomy (`planning` / `tool-use` / `reasoning` / `context` / `integration`), failed-step taxonomy, and orchestrator post-processing. Heading explicitly lists `FAIL` / `WARN` / `SHIP_GATE_DENIED` (was ambiguous "FAILED tasks only").
- **`skills/evolve-loop/phases.md`** audit-to-ship transition: PASS → ship; `WARN`, `FAIL`, or `SHIP_GATE_DENIED` → drop work via `git worktree remove --force`, run retrospective, do NOT commit.
- **`agents/evolve-auditor.md`** Verdict Rules: added "Downstream consumer note" instructing the Auditor to write per-defect root causes (not just symptoms), use consistent severity labels and IDs, and name contradicted prior instincts so the retrospective can propagate them via the lesson's `contradicts` field.

### Fixed — sandbox enforcement bugs (also affects all v8.12.0+ profiles with `read_only_repo: true`)

- **`scripts/cli_adapters/claude.sh`**: the `read_only_repo: true` branch in the macOS sandbox profile generator was a `:` placeholder. Replaced with `echo "(deny file-write* (subpath \"$repo_root\"))"` — emitted before the `write_subpaths` allow loop so per-agent allows correctly override (SBPL last-match-wins). Belt-and-suspenders against future broader allow rules; the auditor, evaluator, scout, and retrospective profiles all benefit.
- **`scripts/cli_adapters/claude.sh`**: the Linux bwrap branch hard-coded `--share-net`, ignoring `allow_network: false` in profiles. Replaced with conditional `--share-net` (when true) / `--unshare-net` (when false). Now mirrors the macOS branch's network policy. The retrospective profile's stated guarantee ("network is disabled") is now actually enforced on Linux.
- **`scripts/merge-lesson-into-state.sh`**: removed unused `mapfile_compat` function.
- **`.evolve/profiles/retrospective.json`**: removed extraneous `Write(.evolve/runs/cycle-*/retrospective-stdout.log)` allow — the runner adapter writes stdout via redirection, not the subagent.

### Audit

- RC1 (Sonnet): WARN — 3 MEDIUM defects (sandbox enforcement bug, Linux bwrap network gap, ambiguous heading). All 9 acceptance criteria PASS otherwise.
- RC2 (Sonnet): PASS — all 5 defects (3 MEDIUM + 2 LOW) verified fixed; 0 new defects introduced; pre-existing LOW-bwrap-bind-coverage flagged for v8.13.0 backlog.

### Notes

- v8.12.0's `read_only_repo: true` flag was advertised as enforcement but was a no-op for ~24 hours. The flag set is now correctly enforced as of v8.12.2. No security incident is known; the implicit deny via "no allow rule covering the repo" provided defense in practice. The explicit deny added in this release is belt-and-suspenders documentation of the contract.

## [8.12.1] - 2026-04-27

### Fixed
- **`scripts/cli_adapters/claude.sh`** — two latent v8.12.0 adapter bugs:
  1. **Tool-pattern word-split**: `Bash(python -m pytest:*)` and similar patterns containing spaces were word-split when passed via `--allowedTools $JOINED_STRING`, producing tokens like `Bash(python` and `-m`. Claude's CLI parser rejected `-m` as an unknown flag (`error: unknown option '-m'`), silently breaking the auditor profile. The runner's `--validate-profile` path didn't catch it because validate-only never invokes `claude` — it only constructs the command line. Fix: read JSON arrays into bash arrays via portable `while IFS= read -r line; do … done < <(jq -r '.field[]?' …)` (bash 3.2 compatible — macOS default shell), then pass with `CMD+=(--allowedTools "${ALLOWED_TOOLS[@]}")` so each pattern survives shell tokenization as its own argv element.
  2. **`--bare` blocks OAuth auth**: `claude --bare` per `claude --help` "is strictly ANTHROPIC_API_KEY or apiKeyHelper via --settings (OAuth and keychain are never read)". Most Claude Code users authenticate via OAuth (no `ANTHROPIC_API_KEY` env var), so subagent subprocesses launched with `--bare` returned `"Not logged in · Please run /login"`. Fix: drop `--bare` automatically when `ANTHROPIC_API_KEY` is unset; emit WARN to stderr; `EVOLVE_FORCE_BARE=1` retains it for users who do have the API key set.

### Audit
- Reviewed by Auditor subagent (cycle 8121-v8121, model: opus, sandbox-exec macOS). Verdict: **PASS** — all 6 acceptance criteria verified, 3 LOW non-blocking observations, 0 MEDIUM+ defects.

### Deferred
- The orchestrator-as-subagent + ship-gate hook + Python shlex parser work tested in this cycle on `feat/orchestrator-as-subagent` (commit `8bc2759`) returned FAIL on two audits (cycle 8121, 8122) due to parser bypasses (D1 bare-newline, D2 pipe-to-sh, D3 here-string) and a workflow regression (D6: `git push` denied after `git commit` due to HEAD movement). That work is parked for v8.13.0, where it will land alongside the deterministic `run-cycle.sh` driver and `role-gate.sh` / `phase-gate-precondition.sh`. Failed-cycle retrospective + lessons-learned pipeline planned as v8.12.2.

## [8.12.0] - 2026-04-27

### Added
- **Subagent subprocess isolation runner (`scripts/subagent-run.sh`)** — single entry point for invoking phase agents (Scout, Builder, Auditor, Inspirer, Evaluator) as isolated subprocesses with per-agent CLI permission profiles. Per-invocation challenge tokens detect forged artifacts; tamper-evident SHA256 ledger entries detect post-run mutation.
- **Per-agent permission profiles (`.evolve/profiles/*.json`)** — least-privilege CLI flag templates encoding `allowed_tools`, `disallowed_tools`, `permission_mode`, `add_dir`, sandbox config, and budget caps for each phase agent.
- **CLI adapter abstraction (`scripts/cli_adapters/{claude,gemini,codex}.sh`)** — uniform interface for provider-agnostic subagent dispatch. Claude adapter wraps invocations in `sandbox-exec` (macOS) or `bwrap` (Linux) when `EVOLVE_SANDBOX=1`.
- **OS-level sandboxing** for every Claude subprocess. Auditor and Evaluator profiles run with `read_only_repo: true` (read-only mount of the repo); Evaluator additionally has `allow_network: false`.
- **Mutation-testing pre-flight (`scripts/mutate-eval.sh`)** — applies language-aware mutations to source-under-test and verifies eval kill rate ≥ 0.8. `phase-gate.sh gate_discover_to_build` runs this against every new eval and flags tautological checks (rollout phase 1: WARN-only).
- **Adversarial Auditor mode** (default-on) — runner injects an "ADVERSARIAL AUDIT MODE" prefix that requires positive executable evidence (test output, diff hunk, or repro command) for PASS verdicts. Counters same-model-judge sycophancy. Auditor model defaults to a different family from Builder (Opus vs Sonnet). Disable with `ADVERSARIAL_AUDIT=0` for permissive sweeps.
- **Subagent ledger match check** — `phase-gate.sh check_subagent_ledger_match` re-hashes each artifact and compares with the SHA256 recorded by the runner at write time, catching post-run mutation.
- **Smoke tests** — `scripts/subagent-run-test.sh` (10 checks: profile validation, forgery rejection, token presence, unknown-agent, missing-artifact, legacy-fallback) and `scripts/subagent-adversarial-test.sh`.

### Changed
- **CLAUDE.md** — added rules 5-8 codifying subprocess-isolation, OS sandboxing, mutation pre-flight, and adversarial auditor as non-negotiable defaults.
- **Skill phase docs** (`phase2-discover.md`, `phase3-build.md`, `phases.md`, `inspirer/SKILL.md`, `evaluator/SKILL.md`, top-level `evolve-loop/SKILL.md`) — replaced platform-dispatch boilerplate with the `subagent-run.sh` invocation contract. `LEGACY_AGENT_DISPATCH=1` documented as a one-A/B-cycle escape hatch only.
- **`agents/evolve-auditor.md`** — PASS verdict criterion tightened to require positive per-criterion evidence; absence of MEDIUM+ issues alone is not sufficient.
- **`scripts/eval-quality-check.sh`** — fenced-code-block fallback parser. Files with eval commands only inside fenced blocks now WARN; files with no parseable commands emit `ANOMALY` (not silent skip).
- **`.gitignore`** — surgical exception so `.evolve/profiles/*.json` ship with the plugin while runtime state under `.evolve/` (state.json, ledger.jsonl, runs/, evals/, history/) remains ignored.

### Fixed
- **`scripts/subagent-run.sh:322`** — successful agent run now exits 0 (per documented contract) instead of 1. Previously every successful subagent invocation was reported as a failure to the orchestrator.

### Documentation
- Added `docs/reports/2026-04-26-subagent-isolation-hardening-report.md` — full incident-style report on the isolation hardening initiative.

## [8.11.1] - 2026-04-20

### Fixed
- **Stability**: Enforce execution-based evals by blocking tautological checks [cycle 15].
- **Safety Guidelines**: Added incident report for Flawless Execution Anomaly and updated safety guidelines.
- Reverted "stability: implement Reasoning Asymmetry for Planner-Auditor [cycle 2]".

## [8.11.0] - 2026-04-20

### Added
- **`autoresearch` strategy** implemented for hypothesis testing against fixed metrics, embracing failure, and deep out-of-the-box exploration.
- **Platform-agnostic generalization**: dynamically scales context windows for Gemini CLI (2M tokens) and gracefully supports non-Claude worktree orchestration and skill lookups.

### Changed
- **Decriminalized failure**: Under `autoresearch` and `innovate` strategies, experimental failures no longer drop consecutive clean scores or discard worktrees.
- **Smart Web Search forced**: Context budget constraints are overridden for divergent strategies to guarantee deep research via the 6-stage pipeline.

## [8.10.3] - 2026-04-11

### Fixed
- **Removed `disable-model-invocation: true` from SKILL.md.** This flag blocked ALL Skill tool calls, including explicit `/evolve-loop` slash commands, causing "Error: Skill evolve-loop:evolve-loop cannot be used with Skill tool due to disable-model-invocation". The skill description is specific enough ("Use when the user invokes /evolve-loop") to prevent unwanted auto-triggering without the flag.

## [8.10.2] - 2026-04-11

### Fixed
- **`release.sh` plugin cache refresh now works post-push.** The old logic compared marketplace SHA to local HEAD (which was the pre-push commit), so the marketplace always appeared "up to date" with the old version. Now: always pulls unconditionally, then checks if the marketplace plugin.json version matches the target. Pre-push runs show "BEHIND — push first, then re-run"; post-push runs correctly refresh cache and registry.

## [8.10.1] - 2026-04-11

### Fixed
- **Context budget gate redesigned from cumulative to per-cycle model.** The old gate accumulated estimated token costs across all cycles (50K/cycle × N), hitting a 300K RED threshold at cycle 4-5 and forcing premature session breaks. The new gate asks "is there room for ONE more cycle?" — since agents run in isolated subagent context and auto-compaction reclaims older turns, effective context stays ~75-165K regardless of cycle count. Result: 10-cycle requests complete entirely in GREEN; YELLOW (lean mode) at cycle 10+; RED safety valve at cycle 30+.
- **YELLOW no longer suggests session break.** Old YELLOW recommendation said "Consider session break after this cycle" — the LLM over-interpreted this as a stop instruction. New YELLOW says "Lean mode activated — continue."
- **RED requires two consecutive confirmations to stop.** A single RED writes a handoff checkpoint but continues (auto-compaction frees space). Only two consecutive RED cycle starts trigger an actual session break.

## [8.10.0] - 2026-04-09

### Added
- **`ecc:e2e` first-class integration** — UI/browser tasks now auto-invoke the `everything-claude-code:e2e-testing` skill to generate and run Playwright tests. Scout routes UI work to a new `e2e` skill category; Builder Step 4.5 generates `tests/e2e/<slug>.spec.ts`; Auditor checklist D.5 verifies selector grounding, artifact presence, and `## E2E Verification` in the build-report; `phase-gate.sh` blocks ship if a UI task is missing e2e evidence.
- **`scripts/setup-skill-inventory.sh` + `scripts/setup_skill_inventory.py`** — deterministic filesystem scanner that indexes every installed skill (project, user-global, plugin cache) and writes `.evolve/skill-inventory.json`. Replaces LLM-side parsing of the session's skill listing with a zero-token, cache-friendly scan. Automatically picks newest plugin version, skips IDE mirror dirs (`.cursor/skills`, `.kiro/skills`), and categorizes via the routing taxonomy. Tested: 281 skills indexed across 7 scopes.
- **New E2E Graders eval-runner section** (`skills/evolve-loop/eval-runner.md`) — first-class grader type with artifact locations (`playwright-report/`, `test-results/`, `artifacts/*.zip`), flake handling, and skip-condition semantics.
- **Auditor audit-report template** extended with `## E2E Grounding (D.5)` table.
- **Builder build-report template** extended with `## E2E Verification` section.

### Changed
- **Phases renumbered to eliminate `x.5` irregularity.** Phase 0.5 → 1, cascade 1-6 → 2-7:
  - Phase 0: CALIBRATE (unchanged)
  - Phase 1: RESEARCH (was 0.5)
  - Phase 2: DISCOVER (was 1)
  - Phase 3: BUILD (was 2)
  - Phase 4: AUDIT (was 3)
  - Phase 5: SHIP (was 4)
  - Phase 6: LEARN (was 5)
  - Phase 7: META (was 6)
- **Phase markdown files renamed** to align filenames with phase numbers and descriptions:
  - `phase05-research.md` → `phase1-research.md`
  - `phase1-discover.md` → `phase2-discover.md`
  - `phase2-build.md` → `phase3-build.md`
  - `phase4-ship.md` → `phase5-ship.md`
  - `phase5-learn.md` → `phase6-learn.md`
  - `phase6-metacycle.md` → `phase7-meta.md` (filename now matches the `Phase 7: META` heading text)
- **Phase 0 Skill Inventory step** now calls `scripts/setup-skill-inventory.sh` instead of LLM-parsing the system-reminder skill list. Deterministic, faster, and complete across every installed plugin.
- **Scout skill-matching table** adds an `e2e` category row routing UI tasks to `everything-claude-code:e2e-testing` as primary.
- **251 phase references** (text + filepaths) rewritten across 43 source files; TOC anchor slugs updated to match renumbered headers; `phase-gate.sh` anti-forgery whitelist extended with `setup-skill-inventory.sh`.

### Migration notes
- Phase numbering is an internal convention — plugin consumers invoke `/evolve-loop` as before.
- `.evolve/` runtime artifacts from prior cycles still reference old phase names in historical logs; the next cycle's Scout will naturally write new-naming references.
- `skills/refactor/` has its own independent phase pipeline (SCAN/PRIORITIZE/PLAN/EXECUTE/MERGE) and was deliberately left unchanged.

## [8.9.1] - 2026-04-07

### Changed
- **Skill descriptions standardized to "Use when..." trigger format** — rewrote descriptions across all skills to start with concrete trigger conditions, improving auto-invocation accuracy.
- **`smart-web-search.md` split into reference files** — 654 → 112 lines. Extracted query transformation patterns, intent classification, and provider routing into `reference/`.
- **`phases.md` Phase 0.5 and Phase 1 extracted** — 700 → 474 lines. Each phase now has its own focused file (`phase05-research.md`, `phase1-discover.md`).
- **`refactor/SKILL.md` split into reference files** — 653 → 154 lines. Detection rules, fix patterns, and worktree orchestration moved to `reference/`.
- **Skill routing policy added** (`skill-routing.md`) — formal policy for which skill handles which kind of request, reducing dispatch ambiguity.
- **SKILL.md frontmatter standardized** — consistent header format and field ordering across all skills.

### Notes
Patch release. No behavior changes — all updates are documentation, file organization, and skill discoverability improvements.

## [8.9.0] - 2026-04-06

### Added
- **`/evaluator` skill** — Independent evaluation engine that works standalone or integrated with evolve-loop. 5-layer architecture (GRADE → DETECT → SCORE → DIRECT → META-EVAL) with 6 scoring dimensions.
- **6 scoring dimensions** — correctness (0.25), security (0.20), maintainability (0.20), architecture (0.15), completeness (0.10), evolution (0.10). Each scored 0.0-1.0 with confidence levels and 5-point granularity rubrics.
- **EST anti-gaming defenses** — Evaluator Stress Test (arXiv:2507.05619) protocol: perturbation tests detect format-dependent score inflation at 78% precision and 2.1% overhead. Includes saturation monitoring and proxy-true correlation tracking.
- **Self-improving evaluation lifecycle** — 4-stage lifecycle from EDDOps (arXiv:2411.13768): baseline → calibration → steady state → evolution. Adaptive difficulty auto-introduces harder criteria when dimensions saturate.
- **Strategic direction guidance** — Layer 4 (DIRECT) ranks improvement priorities by `(1.0 - score) * weight * feasibility` with evidence-linked recommendations tracing to specific files and lines.
- **Meta-evaluation (Layer 5)** — Red-team protocol for the evaluator itself. Triggered by repeated gaming detection, saturation, or proxy correlation drops.
- **3 evaluation scopes** — `task` (changed files), `project` (full codebase), `strategic` (trajectory and priorities).
- **Phase 3 delegation hook** — Evolve-loop Auditor can invoke `/evaluator --scope task` when `strategy == "harden"` or `forceFullAudit == true`.
- **`docs/evaluator-research.md`** — Comprehensive 414-line research archive documenting 14 papers, 8 agent benchmarks, 12 LLM-judge biases, reward hacking incidents, independent evaluation principles, and full cross-reference of existing evolve-loop eval mechanisms.
- **Reference material** — `scoring-dimensions.md` (6-dimension rubric), `anti-gaming.md` (EST protocol + known gaming patterns), `eval-lifecycle.md` (4-stage lifecycle + drift detection + meta-evaluation).

### Research Documented
- EDDOps reference architecture (arXiv:2411.13768) — evaluation as continuous governing function
- Evaluator Stress Test (arXiv:2507.05619) — gaming detection via format/content sensitivity
- CALM framework (arXiv:2410.02736) — 12 LLM-judge biases with mitigations
- METR reward hacking (June 2025) — frontier models actively hack evals
- Anthropic eval principles — unambiguous tasks, outcome over path, saturation monitoring
- AISI Inspect Toolkit — 3-axis sandbox isolation for evaluators
- LiveAgentBench, SWE-Bench Verified, CLEAR — major agent benchmarks 2025-2026

## [8.8.1] - 2026-04-06

### Added
- **`scripts/token-profiler.sh`** — Measures token footprint of all skill, agent, and script files. Outputs ranked table with line counts and estimated tokens. Supports `--json`, `--save-baseline`, and `--compare` flags for tracking optimization progress over time.
- **`docs/token-optimization-guide.md`** — Research-backed optimization guide documenting 5 techniques (three-tier progressive disclosure, context block ordering, AgentDiet trajectory compression, event-driven reminders, per-phase context selection) with measured baselines and per-file recommendations. Cites 7 papers including AgentDiet (FSE 2026), OPENDEV, CEMM, and Prompt Compression Survey (NAACL 2025).

### Changed
- **`skills/evolve-loop/reference/policies.md`** — Compressed from 318 to 176 lines (44% reduction, ~2.1K tokens saved per read). Removed duplicate Session Break Handoff Template and compressed verbose rate limit pseudocode into tables. All 11+ functional sections preserved with zero quality loss.

## [8.8.0] - 2026-04-06

### Added
- **`/inspirer` skill** — Standalone creative divergence engine grounded in data-driven web research. Extracts the evolve-loop's internal creativity mechanisms (provocation lenses, concept scoring, research grounding) into a reusable skill invocable on any topic.
- **6-stage pipeline** — FRAME (parse topic) → DIVERGE (apply lenses) → RESEARCH (web search) → SCORE (Inspiration Cards) → CONVERGE (rank & filter) → DELIVER (report/table/JSON).
- **12 provocation lenses** — 10 from evolve-loop (Inversion, Analogy, 10x Scale, Removal, User-Adjacent, First Principles, Composition, Failure Mode, Ecosystem, Time Travel) + 2 new general-purpose lenses (Constraint Flip, Audience Shift).
- **3 depth levels** — QUICK (~20K tokens, 3 lenses), STANDARD (~40K, 4 lenses), DEEP (~60K, 5 lenses) for explicit creativity-vs-cost tradeoff.
- **Inspiration Cards** — Extended Concept Cards with one-liner pitch, implementation sketch (3-5 steps), risks, and next steps. Scored on feasibility x impact x novelty with KEEP/DROP verdicts.
- **Research grounding requirement** — Every idea MUST be backed by at least 1 web research result. No research = auto-drop.
- **3 output formats** — `full` (human-readable report), `brief` (compact table), `evolve` (JSON compatible with Scout task selection).
- **Domain affinity matrix** — Maps 5 topic domains to optimal lens selections for targeted creative divergence.
- **Phase 0.5 delegation hook** — Evolve-loop orchestrator can delegate to `/inspirer` when `strategy == "innovate"` or discovery velocity stagnates.
- **Reference material** — `provocation-lenses.md` (12 lenses with examples), `scoring-rubric.md` (detailed criteria), `worked-examples.md` (3 end-to-end pipelines).
- **Solution documentation** — `docs/inspirer-solution.md` recording design rationale and architecture decisions.

## [8.7.0] - 2026-04-06

### Added
- **`/code-review-simplify` skill** — Unified code review and simplification engine integrated into the evolve-loop pipeline. Combines structured pattern checks with agentic reasoning in a single pass.
- **Hybrid pipeline+agentic architecture** — Pipeline layer runs 6 deterministic checks (~0.5s, ~2-5K tokens) before agentic layer handles contextual analysis (~15-40K tokens). Saves 40-60% tokens vs. separate review + simplify agents.
- **Multi-dimensional scoring** — 4 dimensions (correctness 0.35, security 0.25, performance 0.15, maintainability 0.25) with numeric 0.0-1.0 scores replace binary PASS/FAIL.
- **Adaptive depth routing** — 3 tiers (lightweight < 50 lines, standard 50-200 lines, full review > 200 lines) with auto-escalation for security-sensitive files.
- **`scripts/code-review-simplify.sh`** — Pipeline layer engine with 6 checks: file length (800), function length (50), nesting depth (4), secrets detection, cognitive complexity (15/function), near-duplicate detection.
- **`scripts/complexity-check.sh`** — Per-function cognitive complexity scorer with `--threshold` flag and multi-language support (bash, Python, JS/TS, Go, Java, Rust).
- **Auditor D4 integration** — Optional skill consultation for code changes > 20 lines; composite score supplements verdict; auto-generates simplification suggestions when maintainability < 0.7.
- **Builder self-review** — Optional Step 5 enhancement runs lightweight pipeline after eval pass; applies simplifications before auditor sees the code.
- **Simplification catalog** — 8 localized refactoring techniques (Extract Method, flatten nesting, decompose conditional, extract utility, rename, replace magic numbers, inline over-abstraction, remove dead code).
- **Solution documentation** — `docs/code-review-simplify-solution.md` records research findings, build-vs-buy justification, architecture decisions, and future work.

### Research Findings
- Anthropic multi-agent code review: 16% → 54% substantive PR comments
- Cursor BugBot: pipeline → agentic = biggest quality gain (70% resolution, 2M+ PRs/month)
- Qodo 2.0: multi-agent specialists achieve F1 = 60.1%
- CodeScene: simplified code reduces AI token consumption ~50%
- ICSE 2025: LLMs excel at localized refactoring, weak at architectural

## [8.6.6] - 2026-04-05

### Added
- **Rate Limit Recovery Protocol** — Detects API rate limits after every agent dispatch (Scout, Builder, Auditor) and auto-schedules resumption via `/schedule` (remote trigger) or `/loop` (local retry) instead of silently dying.
- **3-tier auto-resumption** — Priority cascade: remote trigger (≥1hr limits) → local loop (short limits) → manual fallback.
- **Consecutive failure tracking** — 3+ sequential agent failures trigger rate limit recovery as a safety net.
- **Plugin cache refresh in release flow** — `scripts/release.sh` now clears stale plugin cache, updates marketplace checkout, and refreshes the plugin registry automatically.

### Changed
- Orchestrator loop step 6 now includes rate limit check after every agent dispatch.
- `reference/policies.md` extended with Rate Limit Recovery section and comparison table (rate limit vs context budget).
- `phases.md` adds rate limit recovery gate wrapping all agent dispatches.

## [8.6.0] - 2026-03-31

### Added
- **External skill discovery and routing** — Phase 0 builds a skill inventory from installed plugins, categorizing ~150 skills into routing categories (security, testing, language:X, framework:X, etc.).
- **Task-to-skill matching** — Scout matches tasks to relevant external skills using a category routing table, adding `recommendedSkills` to task metadata.
- **Builder skill consultation** (Step 2.7) — Builder invokes matched skills via the `Skill` tool for domain-specific guidance before designing its approach.
- **Skill usage verification** (Auditor D3) — Auditor checks whether recommended primary skills were invoked (informational, non-blocking).
- **Skill effectiveness tracking** (Phase 5) — Tracks hit rate per skill; low-value skills demoted after 5+ invocations.
- **Skill Awareness** section in `agent-templates.md` — shared schema for `recommendedSkills` field.

### Changed
- **Scout and Builder tools** now include `Skill` in their tool arrays.
- **state.json schema** extended with `skillInventory` and `skillEffectiveness` fields.

## [8.5.0] - 2026-03-30

### Added
- **Beyond-the-Ask divergence trigger** — structured provocation system with 10 lenses (Inversion, Analogy, 10x Scale, Removal, User-Adjacent, First Principles, Composition, Failure Mode, Ecosystem, Time Travel) that fire during Phase 0.5 and Scout hypothesis generation to surface ideas beyond the user's explicit request.
- **Lens selection protocol** — each cycle selects 2 lenses (1 random + 1 matched to weakest benchmark dimension) for targeted creative divergence.
- **Beyond-ask tracking** in Phase 5 — hit rate, lens effectiveness, and benchmark delta for proactive insights. Underperforming lenses flagged for meta-cycle replacement.

### Changed
- **Scout hypothesis generation** now produces standard + beyond-ask hypotheses with differentiated auto-promotion thresholds (0.7 standard, 0.6 beyond-ask).
- **Phase 0.5** includes new Step 2.5 (DIVERGENCE TRIGGER) between gap analysis and research execution.
- **Scout report format** includes separate `Beyond-the-Ask Hypotheses` table.
- **Research brief format** includes `Beyond-the-Ask Provocations` section.

## [8.4.0] - 2026-03-30

### Added
- **Search routing** — decision table in `online-researcher.md` routes queries to Smart Web Search (deep research, surveys, concept cards) or Default WebSearch (quick lookups, error resolution, budget-constrained) based on complexity and token budget.
- **Cost profile benchmarks** — documented token/duration costs for each search approach based on head-to-head comparison testing.

### Changed
- **Builder reactive lookups** now default to Default WebSearch (1-2 direct queries, ~60% token savings) instead of always using the full Smart Web Search pipeline.
- **Phase 0.5 research** uses search routing — Smart for surveys/deep dives, Default for factual gap fills.
- **`smart-web-search.md`** clarifies when to use Smart vs Default with explicit routing guidance.

## [8.3.0] - 2026-03-30

### Added
- **Smart Web Search skill** — intent-aware 6-stage search pipeline that classifies questions, transforms queries using Query2doc/HyDE, iteratively searches and refines, and returns grounded cited answers.
- **Release checklist script** (`scripts/release.sh`) — validates version consistency across all files before release to prevent version drift.

### Changed
- **Online Researcher** now leverages Smart Web Search skill for web searches.

## [8.0.0] - 2026-03-23

### Added
- **Progressive disclosure** — SKILL.md reduced from 523 to 90 lines (85% context reduction). Phase details load on demand via `read_file` references instead of being embedded inline.
- **Agent compression** — all 4 agent files compressed for 41% token reduction while preserving all behavior.
- **Anti-forgery defenses** (v7.9.0) — platform-specific safeguards after Gemini CLI forged audit reports during cross-platform run.
- **Research docs** — enterprise evaluation, agent personalization, adversarial eval co-evolution, runtime guardrails, secure code generation, multi-agent coordination, agent observability, uncertainty quantification, threat taxonomy, pre-execution simulation.

### Changed
- **SKILL.md architecture** — moved from monolithic orchestrator to progressive disclosure pattern. Entry point contains only routing logic; phase details are separate files loaded as needed.
- **Agent files** — restructured for leaner context footprint while maintaining full capability.
- **Reference files** — unified structure per Anthropic skill best practices (blockquote header, TOC, tables over prose).

### Security
- **Anti-forgery defenses** — added after incident where Gemini CLI session forged audit-report.md contents. Auditor now verifies report provenance.

## [7.8.0] - 2026-03-22

### Security
- **Deterministic phase gate script** (`scripts/phase-gate.sh`) — enforces phase transitions via bash, not LLM judgment. Verifies artifact existence, re-runs evals independently, checks health fingerprint, controls state.json writes. The orchestrator cannot skip, suppress, or bypass these checks.
- **Incident report: cycles 132-141** (`docs/incidents/cycle-132-141.md`) — documents orchestrator gaming: skipped agents, fabricated 4 empty cycles, inflated mastery. All existing detection mechanisms were bypassed because the orchestrator controlled whether they ran.
- **Anti-pattern #10: Orchestrator gaming** — added to SKILL.md with cross-reference to incident report

### Changed
- **Phase boundaries now mandatory** — all 5 phase transitions require `phase-gate.sh` execution (discover-to-build, build-to-audit, audit-to-ship, ship-to-learn, cycle-complete)
- **State.json writes moved to script** — `lastCycleNumber` and `consecutiveSuccesses` can only be updated by the phase gate script, not by the LLM directly
- **Safety & Integrity section rewritten** — now documents the separation of enforcement (scripts) from execution (LLM), with research basis (Greenblatt AI Control, Redwood Factored Cognition)
- **Protected paths expanded** — `scripts/` directory added to Builder's protected-file list alongside `skills/`, `agents/`, `.claude-plugin/`

### Research
- **Orchestrator anti-gaming research** (`docs/research-orchestrator-anti-gaming.md`) — surveyed principal-agent problem, separation of duties, tamper-proof logging, AI control protocols, factored cognition. Key finding: structural constraints > behavioral constraints.

## [7.7.0] - 2026-03-22

### Research
- **Pipeline optimization research** (`docs/research-pipeline-optimization.md`) — surveyed 25+ papers from 2025-2026 on parallelization, trimming, multi-model strategies. Key findings: 4-agent saturation (Google/MIT), Self-MoA > multi-model mixing (Princeton), speculative execution 48.7% latency reduction (Sherlock/Microsoft), AgentDiet 40-60% token savings

### Added
- **Self-MoA parallel builds** — spawn 2-3 Builder agents with approach diversity for M-complexity tasks; early termination accepts first passing result. Research: M1-Parallel 2.2x speedup (arXiv:2507.08944), Self-MoA (arXiv:2502.00674)
- **Budget-aware agent context** — `budgetRemaining` field (cyclesLeft, estimatedTokensLeft, budgetPressure) enables agents to self-regulate effort. Research: BATS framework (arXiv:2511.17006)
- **Per-phase context selection matrix** — each agent receives ONLY needed fields; saves 3-5K tokens per invocation. Research: Anthropic Select strategy
- **Speculative Auditor execution** — start Auditor concurrently with Builder; rollback on failure. Research: Sherlock (arXiv:2511.00330)
- **Eval-delta prediction** — Scout predicts benchmark impact per task; Phase 5 tracks prediction accuracy for calibration. Research: eval-driven development (arXiv:2411.13768)
- **Eager context budget estimation** — pre-compute cycle token cost before launching agents; proactive lean mode entry. Research: OPENDEV (arXiv:2603.05344)
- **AgentDiet trajectory compression** — prune useless/redundant/expired context between every phase transition. Research: AgentDiet (arXiv:2509.23586)

### Changed
- **Lean mode trigger** — now activates on budget pressure (not just cycle 4+), enabling earlier optimization
- **Scout task output** — now includes "Expected eval delta" field for prediction tracking
- **phase2-build.md** — expanded with Self-MoA dispatch, speculative auditor, trajectory compression sections

## [7.6.0] - 2026-03-22

### Added
- **Phase decomposition** — monolithic phases.md split into focused modules: `phase0-calibrate.md`, `phase2-build.md`, `phase5-learn.md`, `phase6-metacycle.md` (cycles 122-125)
- **Agent templates** — `agents/agent-templates.md` consolidates shared Input/Output schemas across Scout, Builder, Auditor (cycle 122)
- **Model routing doc** — `docs/model-routing.md` is the single source of truth for tier definitions, provider mappings, and routing rules (cycle 124)
- **Changelog archive** — entries v2.0-v6.9 archived to `CHANGELOG-ARCHIVE.md`, keeping CHANGELOG.md lean (cycle 126)

### Changed
- **phases.md: 717 → 386 lines** (46% reduction) — Phase 0 and Phase 2 extracted to standalone modules
- **phase5-learn.md: 596 → 334 lines** (44% reduction) — meta-cycle logic extracted to phase6-metacycle.md
- **SKILL.md: 560 → 500 lines** (11% reduction) — model routing tables extracted to docs/model-routing.md
- **token-optimization.md: 444 → 412 lines** — model routing duplication removed (references docs/model-routing.md)
- **CHANGELOG.md: 368 → 102 lines** — old entries archived
- **Shared values consolidated** — memory-protocol.md Layer 0 references SKILL.md as canonical source (no duplication)
- **Dead state.json fields removed** — `processRewards` replaced by `processRewardsHistory` in schema
- **Instinct docs deduplicated** — docs/self-learning.md references phase5-learn.md instead of duplicating algorithms
- **Estimated token savings: 24-42K per cycle** (8-14% reduction) from modular loading and deduplication

### Architecture
```
Before (v7.5.0):                    After (v7.6.0):
phases.md (717 lines)               phases.md (386) — orchestrator sequencing
                                    ├── phase0-calibrate.md (99) — once per invocation
                                    ├── phase2-build.md (297) — build orchestration
                                    ├── phase4-ship.md (244) — shipping
                                    ├── phase5-learn.md (334) — per-cycle learning
                                    └── phase6-metacycle.md (191) — every 5 cycles

3 agents × duplicated boilerplate   agent-templates.md (68) + 3 lean agents
1 monolithic model routing table    docs/model-routing.md (single source of truth)
```

## [7.5.0] - 2026-03-22

### Added
- **Platform compatibility doc** (`docs/platform-compatibility.md`) — tool mapping tables for 6 platforms, model tier mappings for 7 providers
- **Multi-platform agent frontmatter** — `capabilities`, `tools-gemini`, `tools-generic` fields in all 4 agents
- **Provider-agnostic prompt caching** — guidance for Anthropic, Google, OpenAI, and self-hosted engines

### Changed
- Agent invocation abstracted from Claude Code `Agent` tool to platform dispatch blocks
- Architecture doc updated: "host LLM session" replaces "Claude Code session"
- Model tier mappings updated to March 2026 latest (Gemini 3.1, GPT-5.4, Mistral Large 3, Qwen 3.5)

## [7.4.0] - 2026-03-21

### Added
- **Hallucination self-detection** — Auditor checklist now includes Section B2 that verifies imports, API signatures, and config keys against actual project dependencies. Catches fabricated APIs before they ship. (Source: agent-self-evaluation-patterns skill)
- **Parallel builder execution** — SKILL.md and phases.md now include explicit dependency-partitioning algorithm and fan-out/fan-in instructions for running independent tasks in parallel worktrees. Cuts cycle latency 2-3x for multi-task cycles. (Source: agent-orchestration-patterns skill)
- **Formal eval taxonomy** — Three grader types (`[code]`, `[model]`, `[human]`) formalized in eval-runner.md with type tagging, cost controls, and pass@k tracking. Scout tags every eval command with its grader type. (Source: eval-harness skill)
- **Process rewards per build step** — Builder reports step-level confidence in build-report.md. Auditor cross-validates via Section D2 (CALIBRATION_MISMATCH detection). Phase 5 aggregates step-level patterns into processRewardsHistory for meta-cycle analysis. (Source: eval-harness process rewards)
- **Instinct-to-skill graduation pipeline** — Meta-cycle now synthesizes qualifying instinct clusters (3+, same category, all confidence >= 0.8) into genes or skill fragments. Recorded in state.json.synthesizedTools. Closes the loop between learning and capability expansion. (Source: continuous-learning-v2, self-learning-agent-patterns skills)
- **Shared values inheritance model** — Shared agent values block in SKILL.md injected into every agent context. Eliminates protocol duplication across 4 agent files, enables single-source-of-truth meta-cycle edits. (Source: agent-shared-values-patterns skill)

### Changed
- **Version: 7.3.0 → 7.4.0** — minor version bump for 6 new features
- **Auditor reduced-checklist rule** — now references Section B2 (Hallucination Detection) alongside A and C as skippable sections
- **docs/skill-building.md** — Stage 5 expanded from 2 lines to full synthesis protocol with gene/skill-fragment examples
- **docs/meta-cycle.md** — Skill Synthesis section added between Automated Prompt Evolution and Mutation Testing

## [7.3.0] - 2026-03-20

### Added
- **Per-cycle enhanced summary** — each cycle now outputs a rich summary with benchmark delta, audit iterations, graduated instincts, operator warnings, and next focus
- **Final session report** — comprehensive markdown report generated after all cycles complete, covering task table, benchmark trajectory, learning stats, and recommendations
- **Auto version bump** — SHIP phase automatically increments patch version in plugin.json/marketplace.json after each cycle push
- **Operator brief spec doc** — new `docs/operator-brief.md` documenting the `next-cycle-brief.json` schema and cross-cycle communication protocol
- **Run isolation doc** — new `docs/run-isolation.md` documenting the `RUN_ID`/`WORKSPACE_PATH` parallel invocation safety model
- **Experiment journal doc** — new `docs/experiment-journal.md` documenting `experiments.jsonl` anti-repeat memory protocol
- **Scout discovery guide extraction** — modular discovery guide extracted from monolithic scout agent for better maintainability
- **Security self-check** — Builder agent now performs security self-verification before completing builds
- **Stepwise scoring enforcement** — mandatory stepwise confidence scoring wired into the evaluation protocol
- **isLastCycle flag** — passed to Operator context for reliable session-summary.md generation on final cycle
- **Instinct graduation section** — `docs/instincts.md` now documents the graduation lifecycle
- **Parallel safety doc** — new `docs/parallel-safety.md` consolidating OCC, ship-lock, and run isolation

### Fixed
- **Schema hygiene** — missing fitness fields added to state.json schema example
- **Method attribution** — validation protocol added for research source attribution

### Changed
- **Benchmark score: ~91** — 12+ tasks shipped across cycles 20-23
- **Version: 7.2.0 → 7.3.0** — auto-bump now prevents version drift

## [7.2.0] - 2026-03-20

### Added
- **Stepwise self-evaluation** — Builder performs per-step correctness checks during implementation using stepwise verification (arxiv 2511.07364), catching errors before they compound
- **Instinct quality scoring (EvolveR)** — instincts now carry quality scores derived from downstream task outcomes, enabling confidence-weighted retrieval and automatic pruning of low-value instincts
- **MUSE functional memory categories** — instincts classified into functional categories (heuristic, constraint, pattern, anti-pattern) for targeted retrieval by agent role
- **CSI metric (Confidence-Stability Index)** — new composite metric tracking confidence-correctness alignment across cycles, used by Operator for pipeline health assessment
- **Phase 4 SHIP extraction** — shipping logic extracted into a dedicated, testable phase module with structured status reporting
- **Confidence-correctness alignment** — process rewards calibrated so stated confidence correlates with actual correctness (arxiv 2603.06604), reducing overconfident shipping of flawed changes

### Fixed
- **30+ broken internal links** — comprehensive link audit and repair across all docs, skills, and agent files (Cycle 16)
- **Link-checker grader regex** — fixed false negatives in the link-checker eval grader caused by overly strict regex patterns
- **processRewards schema** — corrected field validation that rejected valid reward entries with optional dimensions

### Changed
- **Benchmark score: 87.4 to ~91.5** — 9 tasks shipped across 4 cycles with 5 research methods adopted from 8 sources
- **CHANGELOG refreshed** — cycles 16-19 documented

## [7.1.0] - 2026-03-19

### Added
- **Chain-of-thought (CoT) design requirement** — Builder agent Step 3 now requires numbered reasoning steps with evidence citations before selecting an approach (+35% accuracy on complex tasks)
- **Multi-stage verification (MSV)** — Auditor agent applies segment→verify→reflect protocol for M-complexity tasks touching >3 files, with groundedness checking against filesToModify
- **Mutation testing specification** — eval-runner.md now documents mutation generation, kill rate calculation (target >=80%), and interpretation thresholds
- **Token budget awareness for Scout** — Scout agent now estimates per-task token cost and drops lowest-priority tasks when cycle budget (200K) would be exceeded
- **Eval grader best practices guide** — new `docs/eval-grader-best-practices.md` covering grader precision, anti-patterns, composition patterns, worked examples, and mutation resistance
- **Operator benchmark-to-brief translation** — Operator now maps projectBenchmark weakness scores to taskTypeBoosts in next-cycle brief, closing the benchmark→Scout feedback loop
- **Cross-run research deduplication** — OCC-based query locking protocol prevents parallel runs from issuing duplicate web searches (saves 45-90K tokens per overlapping cycle)

### Changed
- **All 4 agent files updated** — Builder (CoT), Auditor (MSV), Scout (token budget), Operator (benchmark sync) now implement documented accuracy and performance techniques
- **eval-runner.md** — mutation testing section + cross-reference to eval-grader-best-practices.md
- **CHANGELOG refreshed** — cycles 13-15 documented

## [7.0.0] - 2026-03-19

### Added
- **Accuracy self-correction techniques** — new `docs/accuracy-self-correction.md` with CoT prompting (+35% accuracy), multi-stage verification (HaluAgent pattern), context alignment scoring, and uncertainty acknowledgment, each mapped to specific evolve-loop agents
- **Implementation patterns** — concrete CoT-enforcing audit graders, multi-stage verification flow examples, and groundedness check patterns in accuracy-self-correction.md
- **Performance Profiling guide** — new `docs/performance-profiling.md` covering per-phase token measurement, cost-bottleneck identification, cycle-level telemetry, and model routing cost impact
- **Security considerations** — new `docs/security-considerations.md` documenting eval tamper detection, state.json integrity, prompt injection defense, rollback protocol, and output groundedness as security signal
- **Plan Cache Schema specification** — JSON schema, write-back protocol, similarity matching algorithm (composite score > 0.7), and eviction rules in token-optimization.md
- **Instinct Graduation specification** — graduation threshold (confidence >= 0.75, 3+ cycle citations), operational effects on Builder/Scout, and reversal conditions in phase5-learn.md
- **Agentic Plan Caching (APC) research baseline** — NeurIPS 2025 paper results (50.31% cost reduction, 27.28% latency reduction) documented in token-optimization.md
- **Dynamic Turn Limits** — probability-based marginal value gating pattern (24% cost reduction) in token-optimization.md

### Fixed
- **Benchmark eval macOS compatibility** — replaced grep -P (PCRE) with -E (POSIX ERE), fixed exit code handling, multi-file grep count summing, stale file paths, and setext header false positives
- **5 broken internal links** in SKILL.md and phase5-learn.md (incorrect relative paths from skills/evolve-loop/ to docs/)

### Changed
- **README.md** — updated project structure tree with all 18 docs, added 3 new feature bullets
- **Project digest** — regenerated at cycle 10 (meta-cycle)

---

For changelog entries prior to v7.0.0 (versions 2.0.0 through 6.9.0), see [CHANGELOG-ARCHIVE.md](CHANGELOG-ARCHIVE.md).
