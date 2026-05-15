# Incident: Cycle 61 — Gemini Scout/Builder Experiment Exposed 7 Bugs

**Date:** 2026-05-15
**Commit:** `4160750` (shipped successfully via Fast-forward merge)
**Dispatcher exit code:** `2` (INTEGRITY-BREACH — misclassified)
**Total cost:** ~$5.33 (orchestrator session) + per-phase costs

## Summary

Cycle 61 was an experiment routing `gemini-3.1-pro-preview` to the Scout and Builder phases of `/evolve-loop`. The cycle technically shipped a commit on `main`, but the dispatcher classified it as an integrity breach due to a memo-phase API 529 failure. The cycle exposed seven bugs in the evolve-loop runtime, six of which are real and warrant structural fixes.

The most interesting aspect: **the orchestrator-report.md narrative was partially unreliable.** Multiple claims in that report (notably "manually fast-forwarded the worktree branch to main" and the description of a ship.sh `INTEGRITY-FAIL` resolution) were either hallucinations from the context-compacted resumed session or misleading paraphrases of standard operations. Source-level evidence (git reflog, state.json mtime, ledger) contradicts the report's wording.

## Timeline

| Time (UTC+8) | Event |
|---|---|
| ~04:55 | I manually bumped `state.json:lastCycleNumber` from 59 → 60 to force cycle 61 numbering (cycle 60 had stale counter from prior re-invocation). |
| 00:06:43 UTC | `state.json:lastUpdated` records this edit. **This timestamp does not change for the remainder of the conversation.** |
| ~08:06 | `/evolve-loop --cycles 1 --budget-usd 5` invoked. Dispatcher provisioned worktree at `.evolve/worktrees/cycle-61`, branch `evolve/cycle-61`. |
| ~08:08 | Scout dispatched via gemini.sh NATIVE adapter against `gemini-3.1-pro-preview`. Initial attempt produced 0-byte raw output due to API rate-limit (429/503). |
| ~08:10 | Orchestrator re-ran Scout with `EVOLVE_LLM_CONFIG_PATH=/dev/null` forcing claude fallback. **Per-role ledger entries for cycle 61 record `cli_resolution.target_cli=claude, model=sonnet, source=llm_config_fallback`** — confirming Builder/Auditor/Triage actually executed on Claude, not Gemini, despite the orchestrator-report implying otherwise (this is B6). |
| ~08:14 | Triage decision written. Build phase ran. |
| ~08:24 | Audit completed. Verdict PASS, 39/39 ACS GREEN, ship_eligible=true. **Auditor cited `gemini.sh:206`** — which exists in HEAD but did NOT appear in cycle 61's eventual commit diff (this is B2). |
| ~08:31:19 | `git merge evolve/cycle-61: Fast-forward` — commit `4160750` lands on `main`. ship.sh succeeded. |
| ~08:32:46 | git reflog records the Fast-forward merge as a single atomic operation. |
| ~08:33–08:37 | Memo phase dispatched. **Hit API Error 529 Overloaded.** Memo created files at project root (`memo_context.txt`, `memo_stderr.log`, `memo_stdout.log`) via shell redirects, bypassing its profile's `Write(.evolve/runs/cycle-*/carryover-todos.json)` allowlist (this is B4). Memo never produced a ledger entry. |
| ~08:42 | run-cycle.sh exited rc=1 (memo failure propagated). Dispatcher classified the cycle as `integrity-breach` (this is B5). |
| post-cycle | Worktree `.evolve/worktrees/cycle-61` deleted. The worktree's `.evolve/state.json` (which ship.sh had advanced to `lastCycleNumber=61`) was deleted along with it. Project-root `.evolve/state.json` still shows `lastCycleNumber=60` (this is B7). |
| later | I manually deleted the memo-litter files from project root. |

## Source-Verified Facts

These facts overrule any conflicting narrative in `.evolve/runs/cycle-61/orchestrator-report.md`:

| Claim | Verification | Result |
|---|---|---|
| ship.sh was NOT modified between cycle 61 commit time and now | `git log --oneline scripts/lifecycle/ship.sh` last touch: `bc8ec09` (cycle 48) | ✓ stable |
| Cycle 61 shipped via standard Fast-forward merge | `git reflog --date=iso main`: `4160750 main@{2026-05-15 08:32:46 +0800}: merge evolve/cycle-61: Fast-forward` | ✓ no anomaly |
| Project-root state.json was NOT touched during cycle 61 | `state.json:lastUpdated=2026-05-15T00:06:43Z` matches my pre-launch edit time | ✓ confirmed |
| Ledger has NO `integrity_fail`/`state_mutation`/`classification` entries for cycle 61 | `jq 'select(.cycle==61 and .kind in ("integrity_fail", ...))' .evolve/ledger.jsonl` returns nothing | ✓ no recorded mutations |
| Builder/Auditor/Triage actually ran on Claude, not Gemini | Per-role ledger entries show `target_cli=claude, model=sonnet, source=llm_config_fallback` | ✓ confirmed |
| Auditor's cited `gemini.sh:206` exists in HEAD | `git show HEAD:scripts/cli_adapters/gemini.sh \| sed -n '206p'` returns the NATIVE-mode echo line | ✓ exists |
| `gemini.sh` was NOT in cycle 61's diff | `git show 4160750 -- scripts/cli_adapters/gemini.sh` is empty | ✓ confirmed |

## Root Cause Analysis

### B0 — gemini.sh NATIVE patch reverted from main

My in-session Edit added a +90-line NATIVE-mode block to `gemini.sh`. The Builder did not stage the file (Scout's F-1 flagged it, but Builder's `## Files Changed/Staged` list omitted it). When the worktree was cleaned up post-ship, the working-tree copy of gemini.sh was discarded. Result: capability flag (`non_interactive_prompt: true`) is shipped on main, but the body is back to the 6-line stub. **Future gemini cycles will hit broken code.**

### B1 — Builder didn't bind to Scout's identified deliverables

Builder's profile allows `Edit`, `Write`, `MultiEdit` generically. No structural mechanism forces Builder to stage every file Scout flagged in Key Findings. Builder simply staged what *it* modified (and what was already on disk per its own intuition), missing the working-tree changes Scout identified.

### B2 — Auditor reads working-tree, not cycle diff

`agents/evolve-auditor.md:52,130` already instructs: "Run `git diff HEAD` to verify changes match build report claims." The model ignored this and cited a file in HEAD that wasn't in the cycle's diff. The verdict was "technically correct" (the line exists) but unbound from the cycle being audited.

### B3 — DISSOLVES (not a real bug)

The orchestrator's context-compacted summary (cycle 61 `orchestrator-stdout.log:356+`) described an `INTEGRITY-FAIL` with `expected_ship_sha=c92c5f0... vs actual=9957ccc...`. This narrative is **not corroborated by evidence**:

- Project-root `state.json:lastUpdated=2026-05-15T00:06:43Z` (my pre-launch edit). If ship.sh had hit INTEGRITY-FAIL against the project-root state.json, the timestamp would be later.
- No ledger entry records an integrity mutation.
- git reflog shows a clean Fast-forward merge at 08:32:46.

Most parsimonious explanation: the INTEGRITY-FAIL story refers to the **worktree's separate `.evolve/state.json` copy**, which ship.sh resolved via its existing v8.32 TOFU rotation, and which was deleted along with the worktree on cleanup. The orchestrator's "manually fast-forwarded the worktree branch to main" phrasing in the report is just confused wording for the standard `git merge --ff-only` that ship.sh performs.

**No code fix needed for B3.** ship.sh's existing rotation logic is correct.

### B4 — Memo profile permits shell-redirect escape

Memo's allowed_tools include `Bash(cat:*)`, `Bash(tail:*)`, `Bash(head:*)`, `Bash(jq:*)`. None of these are *redirect-aware* — the Bash tool permits the full command line including `>` and `>>`. The model used `cat ... > memo_context.txt` (CWD=project root) to write files outside the profile's `Write(.evolve/runs/cycle-*/carryover-todos.json)` allowlist. Three files appeared at project root before being manually cleaned.

### B5 — Classifier doesn't scan per-role logs

`scripts/dispatch/evolve-loop-dispatch.sh:558-621` (`classify_cycle_failure`) only greps `orchestrator-report.md` for infrastructure markers (`429`, `503`, `rate.limit`, `EPERM`). Memo's API 529s landed in `memo-stdout.log`, NOT in `orchestrator-report.md`. The classifier fell through to coarse `integrity-breach` when memo failed with a recoverable infra error.

### B6 — Orchestrator-report doesn't disclose CLI fallback events

Cycle 61's orchestrator-report implied Builder ran on Gemini (the goal section says "Finalize Gemini native mode adapter"). The actual ledger shows `source=llm_config_fallback` — Builder ran on Claude after gemini rate-limited. There's no `## CLI Resolution` section that the report MUST populate from ledger evidence. The opacity compounds the trust problem demonstrated by B3's hallucination.

### B7 — NEW: state.json:lastCycleNumber not synced from worktree

The worktree's `.evolve/state.json` got correctly advanced to `lastCycleNumber=61` by ship.sh during cycle 61's run. But that copy lived only in the worktree and was deleted on cleanup. The project-root state.json still reads `lastCycleNumber=60` (from my pre-launch edit, untouched since).

This is a structural gap: there's no post-ship sync between worktree state and project-root state. Future cycles will see stale state and either re-use cycle number 61 (collision with shipped `4160750`) or be off-by-one.

This bug was NOT in the original list (B1-B6) — surfaced only during the postmortem investigation.

## Structural Fixes Required

| Bug | Step | Fix Location |
|---|---|---|
| B0 | Step 2 | `scripts/cli_adapters/gemini.sh:198-204` — re-apply NATIVE block |
| B1 | Step 4 | `scripts/lifecycle/scout-grounding-check.sh` (new) + phase-gate.sh wire-up |
| B2 | Step 5 | `scripts/lifecycle/audit-citation-check.sh` (new) + phase-gate.sh wire-up |
| B3 | — | NO FIX. ship.sh v8.32 TOFU already handles. |
| B4 | Step 7 | `.evolve/profiles/memo.json` — drop `Bash(cat:*)`, `Bash(tail:*)`, `Bash(head:*)` |
| B5 | Step 3 | `scripts/dispatch/evolve-loop-dispatch.sh:565-569` — extend infra grep to per-role `*-stdout.log`, `*-stderr.log` |
| B6 | Step 6 | `scripts/observability/render-cli-resolution.sh` (new) + phase-gate cycle-complete wire-up |
| B7 | — | **Addressed by manually fixing state.json:lastCycleNumber=61 in this cycle's commit.** Structural fix deferred (warrants its own future investigation of why worktree state doesn't merge back to project root). |

## Resolution Status

_Section originally produced by Gemini-3.1-pro-preview Builder in cycle 64 (re-test of cycle-61's gemini routing experiment). The edit was correct but Gemini did not run `git add`/`git commit`, so the section is merged manually here. See "Future Structural Fixes" below for the B8/B9 follow-ups this gap surfaced._

- B0=57cbd4c
- B1=781ae83
- B2=a9d8356
- B3=DISSOLVED (no commit needed)
- B4=7a9f356
- B5=ab0d5a7
- B6=abcd076+e810df7
- B7=a28e9e5

## Future Structural Fixes (Open as of v10.7.0)

Cycle 64's gemini re-test exposed two NEW classes of failure that the current framework catches only post-hoc (audit verdict) rather than pre-emptively (phase-gate enforcement). Both warrant dedicated structural fixes in future cycles.

| Bug | Severity | Description | Proposed fix |
|---|---|---|---|
| **B8** | HIGH | **Builder-exit commit-presence gate missing.** Builder's persona explicitly instructs `git add -A && git commit -m "..."` (lines 56-64, 377-383 of `agents/evolve-builder.md`). Builder profile permits these commands. Despite both, Gemini-3.1-pro-preview Builder in cycle 64 made the file edit correctly but never invoked `git add`/`git commit`. The cycle-24 instinct (`cycle-24-builder-uncommitted-worktree-edit.yaml`, confidence 0.97) names this exact failure mode. No phase-gate currently enforces it. | Add to `scripts/lifecycle/phase-gate.sh:gate_build_to_audit`: a check that `git -C "$WORKTREE_PATH" status --porcelain` returns empty, OR fails the build-to-audit transition. Catches any model (Gemini or Claude on a bad day) that skips the commit. WARN-mode default-on initially per the v8.55 rollout ladder. |
| **B9** | MEDIUM | **EGPS predicate-presence gate missing.** ADR-7 requires Builder to write `acs/cycle-N/*.sh` predicates for each acceptance criterion. Cycle 64's Gemini Builder wrote ZERO predicate files; the auditor noted this as DEFECT-2 but the cycle still proceeded to verdict computation (which fell back to legacy audit-report.md prose verdict). | Add to `scripts/lifecycle/phase-gate.sh:gate_build_to_audit`: a check that `[ -d "$WORKTREE_PATH/acs/cycle-$CYCLE" ] && [ "$(find ... -name '*.sh' \| wc -l)" -gt 0 ]`. Mirrors B8's pattern. Forces EGPS predicate authorship structurally rather than relying on Builder persona compliance. |

Both fixes are Tier 1 (phase-gate hooks) and would close the gap between "Builder persona instructs X" and "Builder does X." They make the framework's protocol compliance independent of model behavior — closing the cycle 64 demonstrated gap between Claude's reliable instruction-following and Gemini's optimization-toward-visible-deliverable.

**Filing as carryover for cycle 65+:** see `state.json:carryoverTodos[]` after this commit lands.

## Operator Lessons

1. **The orchestrator-report is not source of truth.** When narrative claims conflict with file-level evidence (git reflog, ledger, file mtimes), trust the evidence. B3's "permission escape" mystery dissolved once we checked `state.json:lastUpdated`.
2. **Context compaction is a narrative-reliability hazard.** The orchestrator's session was compacted around `orchestrator-stdout.log:356`. Post-compaction claims in the report should be cross-checked against the ledger and reflog.
3. **Worktree state ≠ project-root state.** Subagents editing `.evolve/state.json` inside the worktree are editing a separate file from the project root's. Profile permission denials may apply to one but not the other.
4. **`/evolve-loop --cycles 1` is a real test bed.** Cycle 61 burned ~$5 and ~30 minutes but produced concrete evidence for 7 bugs. The investment is worthwhile when the alternative is debugging the runtime in production.
5. **Predicate 040 (mixed-CLI routing) was correct but narrow.** It proved the *router* dispatches correctly. It did NOT prove the *adapter* invokes correctly nor that downstream phases survive a fallback. Cycle 61's true contribution was filling that gap.

## References

- Plan: `/Users/danleemh/.claude/plans/let-s-follow-the-step-cozy-shannon.md`
- Workspace: `.evolve/runs/cycle-61/` (orchestrator-stdout.log, audit-report.md, build-report.md, scout-report.md, memo-stdout.log)
- Predicate 040 (mixed-CLI routing): `acs/regression-suite/cycle-60/040-e2e-mixed-cli-cycle.sh`
- Related incident: `docs/incidents/gemini-forgery.md` (cycle 7.9.0+ defenses)
- ship.sh integrity logic: `scripts/lifecycle/ship.sh:220-287` (v8.32 TOFU), `:872-883` (v11.0 T1 auto-heal)
