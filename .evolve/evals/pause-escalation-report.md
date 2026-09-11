# Eval: pause-escalation-report

**Cycle:** 189
**Task:** When the tmux stop-reviewer issues a `pause` verdict, write a
`<workspace>/<phase>-escalation-report.json` containing the investigation evidence
(phase, cycle, elapsed_s, interval_s, attempt, reason, final_pane). Remove the
existing `// TODO(auto-respond slice)` comment.

---

## Criteria

### AC-1: `writeEscalationReport` (or equivalent) function exists in bridge package [code]

```bash
grep -n "writeEscalationReport\|escalation.*report\|escalation-report" go/internal/bridge/driver_tmux_repl.go
```

Expected: at least one match.

---

### AC-2: Escalation report written on `ReviewPause`, not on `ReviewExtend` [code]

```bash
# Positive: pause → report written
grep -rn "escalation-report\|EscalationReport\|escalation_report" go/internal/bridge/
```

Expected: ≥1 match showing the write path.

```bash
# TODO comment must be removed
grep -n "TODO.*auto-respond\|TODO.*escalation-report" go/internal/bridge/driver_tmux_repl.go
grep_status=$?
[ "$grep_status" -eq 1 ] || { echo "FAIL: expected no TODO match; grep exit=$grep_status" >&2; exit 1; }
```

Expected: the negated assertion exits 0 only when there is no match.

---

### AC-3: Escalation report JSON contains required fields [code]

```bash
# The struct or marshalled type must include phase and reason fields
grep -n "\"phase\"\|\"reason\"\|ElapsedS\|elapsed_s\|FinalPane\|final_pane" go/internal/bridge/driver_tmux_repl.go
```

Expected: ≥2 matches (the struct fields used for the report).

---

### AC-4: Unit test verifies report is written on pause, absent on extend [code]

```bash
grep -n "TestRunTmuxREPL.*Pause\|TestPause.*EscalationReport\|EscalationReport.*Pause\|escalation.*pause\|pause.*escalation" go/internal/bridge/
```

Expected: at least one matching test name.

```bash
# The test must read/stat the escalation-report.json to confirm it exists
grep -rn "escalation-report.json\|EscalationReportPath\|escalation_report" go/internal/bridge/
```

Expected: ≥1 match inside a test file.

---

### AC-5: ADR-0026 Stage 1 #3 marked SHIPPED [code]

```bash
grep -n "SHIPPED\|shipped.*189\|cycle-189" docs/architecture/adr/0026-self-healing-review-layer.md
```

Expected: ≥1 match indicating Stage 1 #3 is marked as shipped.

---

### AC-6: Tests pass [code]

```bash
cd go && go test ./internal/bridge/... -run "TestPause\|TestEscalation\|TestRunTmuxREPL" -count=1 2>&1 | tail -5
```

Expected: output contains "ok" with no FAIL lines.

---

## Negative Cases

### NC-1: Extend verdict does NOT create an escalation report [code]

```bash
cd go && go test -count=1 -run '^TestRunTmuxREPL_ExtendNoEscalationReport$' ./internal/bridge
```

Expected: the named behavioral test passes and proves extend leaves no report.
