# Eval: Add Semantic Eval-Grader Patterns (Cycle 19)

## Code Graders (bash commands that must exit 0)

### Task A — Eval-grader best-practices doc exists and has substance
- `test -f docs/eval-grader-best-practices.md`
- `wc -l < docs/eval-grader-best-practices.md | awk '{exit($1<60)}'`
- `grep -q "Level 3.5\|Control-flow structural" docs/eval-grader-best-practices.md`
- `grep -q "awk" docs/eval-grader-best-practices.md`

### Task B — Rewritten eval has semantic graders (no absolute paths, awk/jq-e present)
- `grep -c "awk\|jq -e" .evolve/evals/add-code-reviewer-auditor-lens.md | awk '{exit($1<2)}'`
- `grep -c "grep -q" .evolve/evals/add-code-reviewer-auditor-lens.md | awk '{exit($1>1)}'`

### Task C — gate_discover_to_build filters to untracked evals via git ls-files
- `awk '/gate_discover_to_build/{f=1} f && /ls-files.*others.*evals/{found=1} END{exit(!found)}' scripts/lifecycle/phase-gate.sh`

### Task C — gate_discover_to_build uses 0.7 threshold
- `grep -c "\-\-threshold 0.7" scripts/lifecycle/phase-gate.sh | awk '{exit($1<1)}'`

### Task C — gate_build_to_audit also runs mutate-eval on new evals
- `awk '/gate_build_to_audit/{f=1} f && /mutate-eval.*threshold 0.7/{found=1} END{exit(!found)}' scripts/lifecycle/phase-gate.sh`

### Task B+C — FANOUT flag guards code-reviewer dispatch (semantic, Level 3.5)
- `awk '/EVOLVE_FANOUT_AUDITOR_CODE_REVIEWER.*=.*"1"/{p=1} p && /subagent-run.*code-reviewer/{found=1} END{exit(!found)}' scripts/lifecycle/phase-gate.sh`

### Task B — code-reviewer profile has required fields (Level 3 jq structural)
- `jq -e 'has("parallel_eligible") and has("challenge_token_required") and has("sandbox")' .evolve/profiles/code-reviewer.json`

### Task D — exit-transport-hang classifier uses multi-line awk extraction (not colon regex)
- `awk '/^##[[:space:]]+Verdict/{f=1;next} f && NF{print tolower($0);exit}' .evolve/runs/cycle-17/orchestrator-report.md | grep -qiE 'shipped'`

### Regression — phase-gate.sh is well-formed (version or function grep)
- `grep -q "gate_discover_to_build\|gate_build_to_audit" scripts/lifecycle/phase-gate.sh`

### Regression — dispatch script still has classify_cycle_failure function
- `grep -q "classify_cycle_failure" scripts/dispatch/evolve-loop-dispatch.sh`

## Thresholds
- All checks: pass@1 = 1.0
