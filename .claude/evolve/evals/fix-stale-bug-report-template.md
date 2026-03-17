# Eval: fix-stale-bug-report-template

## Code Graders (bash commands that must exit 0)
- `grep -c "Phase 1: DISCOVER" /Users/danleemh/ai/claude/evolve-loop/.github/ISSUE_TEMPLATE/bug_report.md`
- `grep -c "Phase 2: BUILD" /Users/danleemh/ai/claude/evolve-loop/.github/ISSUE_TEMPLATE/bug_report.md`
- `grep -c "Phase 3: AUDIT" /Users/danleemh/ai/claude/evolve-loop/.github/ISSUE_TEMPLATE/bug_report.md`
- `grep -c "Phase 4: SHIP" /Users/danleemh/ai/claude/evolve-loop/.github/ISSUE_TEMPLATE/bug_report.md`
- `grep -c "Phase 5: LEARN" /Users/danleemh/ai/claude/evolve-loop/.github/ISSUE_TEMPLATE/bug_report.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -v "MONITOR-INIT\|Phase 2: PLAN\|Phase 3: DESIGN\|Phase 4.5:\|Phase 5.5:\|Phase 6: SHIP\|Phase 7:\|evolve-developer\|evolve-reviewer" /Users/danleemh/ai/claude/evolve-loop/.github/ISSUE_TEMPLATE/bug_report.md | wc -l`
- `grep "evolve-scout\|evolve-builder\|evolve-auditor\|evolve-operator" /Users/danleemh/ai/claude/evolve-loop/.github/ISSUE_TEMPLATE/bug_report.md`

## Thresholds
- All checks: pass@1 = 1.0
