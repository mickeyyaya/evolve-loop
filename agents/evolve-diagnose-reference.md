---
name: evolve-diagnose-reference
description: Layer-3 on-demand reference for diagnosing runtime failures in Evolve Loop. 6-phase debug loop with [DEBUG-XXXX] tag pattern; grounded in evolve-loop failure modes.
---

# Evolve Diagnose Reference (Layer 3 — on-demand)

> Layer-3 on-demand reference. Adapted from mattpocock's 6-phase diagnose loop (mattpocock.com / Total TypeScript). Read this when a phase agent or orchestrator is facing a recurring failure and needs a systematic approach to trace and isolate the root cause. When diagnosing a WARN or FAIL cycle, come here before writing guesses into the retrospective. Do NOT read this file on every cycle — it is on-demand only.

## Contents

- [When to use this reference](#when-to-use-this-reference)
- [Phase summary](#phase-summary)
- [Phase 1: Observe](#phase-1-observe)
- [Phase 2: Hypothesize](#phase-2-hypothesize)
- [Phase 3: Instrument](#phase-3-instrument)
- [Phase 4: Reproduce](#phase-4-reproduce)
- [Phase 5: Isolate / bisect](#phase-5-isolate--bisect)
- [Phase 6: Fix, verify, remove tags](#phase-6-fix-verify-remove-tags)
- [[DEBUG-XXXX] Tag Convention](#debug-xxxx-tag-convention)
- [Evolve-Loop Examples](#evolve-loop-examples)

---

## When to use this reference

| Trigger | Action |
|---------|--------|
| Audit returns FAIL and the root cause is unclear | Start at Phase 1 — do not skip to a fix |
| Same WARN class appears in 3+ consecutive `recentFailures` entries | Start at Phase 2 — symptom is already captured |
| `subagent-run.sh` exits non-zero and stderr is empty | Start at Phase 3 — instrument before re-running |
| `ship.sh` exits non-zero with an audit SHA mismatch | Use Example 2 below as the instrumentation template |
| Cycle cost is $0.00 and no ledger entry was written | Start at Phase 3 with the sandbox EPERM template (Example 3) |

---

## Phase summary

| # | Phase | Core action |
|---|-------|-------------|
| 1 | **Observe** | Capture the exact symptom: observed behavior vs. expected behavior. Record the precise error message, exit code, artifact path, and stack trace line. Do NOT hypothesize here. |
| 2 | **Hypothesize** | Generate 3–5 candidate root causes. Rank by likelihood. Write them down before testing any. |
| 3 | **Instrument** | Add `[DEBUG-XXXX]` tagged log/assertion points to test the top hypothesis. Instrument to test a specific hypothesis — NOT to "see what happens." |
| 4 | **Reproduce** | Reliably reproduce the failure with minimum steps. No fix is valid unless the failure can be triggered on demand. |
| 5 | **Isolate / bisect** | Eliminate half the candidate causes per iteration. Binary search to the smallest reproduction case. |
| 6 | **Fix + verify + remove tags** | Apply fix. Verify against the reproduction case. Search `grep -r '\[DEBUG-XXXX\]'` and remove all diagnostic artifacts before committing. |

---

## Phase 1: Observe

1. Read the full error output without filtering. Copy the exact error message, exit code, and artifact path.
2. State the observed behavior in one sentence: "Script X returned rc=N with stderr: '…'."
3. State the expected behavior in one sentence: "Script X should return rc=0 and write artifact Y."
4. Identify the artifact that SHOULD have been created and verify whether it exists:
   ```bash
   ls -l "$WORKSPACE/audit-report.md"
   ```
5. Record timestamp of the failure from the ledger:
   ```bash
   tail -5 .evolve/ledger.jsonl | jq '.ts, .role, .exit_code'
   ```
6. Stop. Do not hypothesize yet. Write down only what you observed.

---

## Phase 2: Hypothesize

1. Generate exactly 3–5 candidate root causes for the observed symptom.
2. Rank them by likelihood — most likely first.
3. For each candidate, state what evidence would confirm or refute it.
4. Write the ranked list before testing any hypothesis.

Example format:

| Rank | Hypothesis | Confirmation evidence |
|------|-----------|----------------------|
| 1 | Phase advance was skipped — cycle-state shows wrong phase | `cycle-state.sh get phase` returns unexpected value |
| 2 | Audit-report SHA changed after audit ran | `sha256sum audit-report.md` differs from ledger `audit_sha` |
| 3 | Builder wrote to main tree instead of worktree | `git diff HEAD -- agents/` in project root shows unexpected changes |

---

## Phase 3: Instrument

1. Pick the **top-ranked hypothesis** from Phase 2.
2. Generate a 4-char hex session tag: `printf '[DEBUG-%04x]' $((RANDOM & 0xFFFF))`
3. Add `[DEBUG-XXXX]` log lines AT THE SPECIFIC CODE PATH that would confirm or refute the hypothesis. Do NOT scatter tags broadly.
4. Each instrumented line must start with the tag:
   ```bash
   echo "[DEBUG-XXXX] phase=$(cycle-state.sh get phase)"
   echo "[DEBUG-XXXX] active_worktree=$(cycle-state.sh get active_worktree)"
   ```
5. Re-run only the failing step — not the full cycle.
6. Read the output. Does it confirm or refute hypothesis 1?
7. If refuted, move to hypothesis 2 and repeat.

---

## Phase 4: Reproduce

1. Write the minimum command sequence that reproduces the failure from a clean state.
2. Run it twice — confirm the failure is deterministic before proceeding.
3. If the failure is intermittent: add timing information to the instrumentation (timestamp each tagged line).
4. Document the reproduction steps — these become the test case for the fix.

Example minimum reproduction:
```bash
# Reproduce phase-gate denial
cycle-state.sh advance research scout  # sets phase=research
subagent-run.sh builder 99 /tmp/fake-workspace  # should fail: phase != build
echo "exit: $?"
```

---

## Phase 5: Isolate / bisect

1. Remove variables from the reproduction case one at a time.
2. After each removal, re-run and confirm the failure still occurs.
3. When removing a variable makes the failure disappear, that variable is the root cause.
4. Binary-search approach: split the reproduction steps in half; test each half separately to find which half contains the root cause.
5. Stop isolating when you can point to a single function call, script line, or state value as the cause.

---

## Phase 6: Fix, verify, remove tags

1. Apply the minimal fix targeting the isolated root cause.
2. Re-run the reproduction case — confirm the failure is gone.
3. Run targeted tests:
   ```bash
   bash scripts/tests/<relevant-suite>.sh
   ```
4. Remove ALL `[DEBUG-XXXX]` instrumentation:
   ```bash
   grep -r '\[DEBUG-[0-9a-f]\{4\}\]' .
   ```
   This must return zero matches before commit.
5. Verify `git diff --stat` shows only the intended files changed.

---

## [DEBUG-XXXX] Tag Convention

| Aspect | Rule |
|--------|------|
| Format | `[DEBUG-XXXX]` — 4 lowercase hex chars (e.g., `[DEBUG-3a7f]`) |
| Scope | One unique tag per debugging session — never reuse across sessions |
| Generation | `printf '[DEBUG-%04x]' $((RANDOM & 0xFFFF))` in bash |
| Placement | Prepend to every `echo`, `printf`, comment, or assertion added during the session |
| Removal | Before any commit: `grep -r '\[DEBUG-3a7f\]' .` must return 0 matches |
| Bulk grep | `\[DEBUG-[0-9a-f]\{4\}\]` matches all debug tags across sessions (for audit) |

Do NOT commit files containing `[DEBUG-XXXX]` tags. The tag is a diagnostic artifact, not documentation.

---

## Evolve-Loop Examples

### Example 1: Phase-gate denial

**Symptom**: `subagent-run.sh builder` exits non-zero, stderr contains:
```
[PHASE-GATE] DENIED: current phase=calibrate, role=builder requires phase=build
```

**Hypotheses**:
1. Orchestrator called `subagent-run.sh builder` without first calling `cycle-state.sh advance build builder`
2. Stale `completed_phases` in cycle-state.json caused the orchestrator to skip the phase-advance step

**Instrumentation target**: Add before `subagent-run.sh builder`:
```bash
echo "[DEBUG-XXXX] phase=$(cycle-state.sh get phase)"
echo "[DEBUG-XXXX] completed=$(cycle-state.sh get completed_phases)"
```
If output shows `phase=calibrate` or `phase=intent`, the advance calls are missing.

**Fix check**: `cycle-state.sh get phase` returns `build` before invoking builder.

---

### Example 2: Ledger SHA mismatch on ship.sh

**Symptom**: `ship.sh` exits non-zero, stderr contains:
```
[SHIP-GATE] FAIL: audit SHA mismatch — audit_sha=abc12345 != cycle_binding=def67890
```

**Hypotheses**:
1. `audit-report.md` was written before Builder made additional workspace edits
2. File content changed (even whitespace) after audit ran
3. ship.sh's cycle-binding check is reading the wrong path

**Instrumentation target**: Add after Auditor exits, before ship.sh:
```bash
sha_at_audit=$(sha256sum .evolve/runs/cycle-28/audit-report.md | cut -c1-8)
echo "[DEBUG-XXXX] audit_sha=$sha_at_audit"
echo "[DEBUG-XXXX] current_binding=$(cycle-state.sh get audit_sha)"
```

**Fix check**: No writes to `$WORKSPACE/audit-report.md` after Auditor exits. Verify with
`git diff --stat` in the worktree between Auditor exit and ship.sh invocation.

---

### Example 3: Sandbox EPERM (Darwin 25.4+ nested Claude)

**Symptom**: `subagent-run.sh` ledger entry shows `exit:71` or `exit:1` + empty stderr.
No claude session output. Cycle cost $0.00.

**Hypotheses**:
1. `sandbox-exec` returns EPERM when the parent Claude Code process is itself sandboxed (Darwin 25.4+ nested-sandbox disallowed)
2. `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` is not set, so the runner does not retry unsandboxed

**Instrumentation target**:
```bash
echo "[DEBUG-XXXX] EVOLVE_SANDBOX_FALLBACK_ON_EPERM=${EVOLVE_SANDBOX_FALLBACK_ON_EPERM:-unset}"
echo "[DEBUG-XXXX] CLAUDECODE=${CLAUDECODE:-unset}"
sandbox-exec -n "allow default" echo "sandbox-ok" 2>&1 | sed "s/^/[DEBUG-XXXX] /"
```

**Fix check**: Set `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` and re-run. `scripts/dispatch/detect-nested-claude.sh`
should also auto-set this when `CLAUDECODE` env var is present.

---

### Example 4: Worktree drift (path in cycle-state but not on disk)

**Symptom**: Builder writes succeed (no role-gate denial) but post-cycle `git diff` in the
main tree shows no changes. `git log` confirms no new commit.

**Hypotheses**:
1. `cycle-state.json:active_worktree` points to a temp path that was cleaned by a prior EXIT trap or OS temp purge
2. Builder wrote to a non-existent path and the filesystem silently created new dirs without any git worktree backing

**Instrumentation target**:
```bash
wt=$(cycle-state.sh get active_worktree)
echo "[DEBUG-XXXX] active_worktree=$wt"
ls -d "$wt/.git" 2>&1 | sed "s/^/[DEBUG-XXXX] git-check: /"
git worktree list | grep "$wt" | sed "s/^/[DEBUG-XXXX] worktree-list: /"
```

**Fix check**: `git worktree list` includes the path AND `ls $active_worktree/.git` succeeds.
If the path is absent, the cycle must be restarted — there is no in-cycle recovery path.
