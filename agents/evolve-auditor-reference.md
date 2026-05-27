# Auditor Reference (Layer 3 — on-demand)

Sections here are loaded when the auditor's primary flow needs deeper rules
that are conditional on cycle history (consecutiveClean streak length, force-
full-audit flags, strategy mode). v8.64.0 Campaign D Cycle D2 split.

The auditor's compact role-card (Layer 1) lives at
`agents/evolve-auditor.md` and includes a `## Reference Index` pointing here.

---

## Section: adaptive-strictness

Loaded when the auditor needs to decide which sections of the Single-Pass
Review Checklist to run vs. skip. The full table + rationale is here; the
Layer 1 persona summarizes the rule in one line.

Read `auditorProfile` from context:

```json
{
  "feature":     {"passFirstAttempt": 0, "consecutiveClean": 0},
  "stability":   {"passFirstAttempt": 0, "consecutiveClean": 0},
  "security":    {"passFirstAttempt": 0, "consecutiveClean": 0},
  "techdebt":    {"passFirstAttempt": 0, "consecutiveClean": 0},
  "performance": {"passFirstAttempt": 0, "consecutiveClean": 0}
}
```

**Checklist rules by streak length:**

| `consecutiveClean` | Checklist | Rationale |
|---------------------|-----------|-----------|
| `< 3` (new/unstable) | Full checklist (all sections) | Not enough trust built |
| `3-7` (stable) | Can skip C (Pipeline Integrity) ONLY if no agent/skill files modified | Core quality checks always run |
| `>= 8` (long streak) | **Full checklist mandatory** (streak verification audit) | Long streaks mask slow drift — increase rigor |

**Sections A (Code Quality), B (Security), B2 (Hallucination), D (Eval
Integrity) are NEVER skipped.**

**Random full-audit sampling:** Orchestrator passes `"forceFullAudit": true`
~20% of the time. When true, run ALL sections regardless of streak. Do NOT
mention in report whether this was a random sample (blind monitoring).

**Always run full checklist when:**
- `strategy` is `harden` or `repair`
- Task modifies agent/skill files or `.claude-plugin/`
- Build report flags risks
- `forceFullAudit` is true
- `consecutiveClean >= 8` (streak verification)

**Cross-session decay (orchestrator):** New session halves all
`consecutiveClean` values (rounded down).

**Profile update (orchestrator):** PASS first attempt + no issues =
`consecutiveClean += 1`. WARN/FAIL/MEDIUM+ issue = reset to 0.

---

## Section: review-checklist

Loaded for the Single-Pass Review.

### A. Code Quality
- Matches ACs, follows patterns, no dead code.
- S-tasks >30 lines or M-tasks >80 lines = MEDIUM warning.

### B. Security & B2. Hallucination
- No secrets, injection, or unvalidated input.
- **B2**: Verify all new imports and API signatures.

### C. Pipeline Integrity & D. Eval Rigor
- Structure intact, cross-refs valid, ledger entries exist.
- **D**: Verify eval quality Level 2+. Level 0-1 only = CRITICAL FAIL.
- **D.5**: E2E Grounding for UI tasks.

---

## Section: egps-computation

Loaded for EGPS Verdict Computation (v10.1.0+).

1. **Validate predicates**: Run `validate-predicate.sh` on all `.sh` in `acs/cycle-N/`.
2. **Run suite**: `evolve acs suite --cycle "$cycle"`. This deterministic host-side runner (Go) globs `acs/cycle-N/` + `acs/regression-suite/cycle-*/` + `acs/red-team/`, executes each bash predicate, and writes `acs-verdict.json`. It replaces the deleted `run-acs-suite.sh` (ADR-0025). The standing `acs/red-team/` predicates encode past gaming incidents and fire every cycle.
3. **Cross-check**: Every AC MUST have a predicate.
4. **Verdict**: PASS (red_count == 0) or FAIL (red_count > 0).

---

## Section: handoff-json

Loaded for Structured Output: handoff-auditor.json (C3).

Emit JSON sidecar to `$WORKSPACE/handoff-auditor.json`.
Required: `cycle`, `verdict`, `confidence`, `audit_bound_tree_sha`, `acceptance_criteria_results`, `adversarial_checks`.

---

## Section: output-template

Loaded when the auditor writes `workspace/audit-report.md` and the `Ledger Entry` after the verdict is decided.

### Workspace File: `workspace/audit-report.md`

```markdown
<!-- challenge-token: {token} -->
# Cycle {N} Audit Report

audit_bound_tree_sha: {TREE_SHA}

<!-- ANCHOR:verdict -->
## Verdict: PASS / WARN / FAIL

## Code Quality
```tsv
Check	Status	Details
Matches ACs	PASS/FAIL	<detail>
Patterns	PASS/FAIL	<detail>
Complexity	PASS/WARN	<detail>
```

## Security
```tsv
Check	Status	Details
No secrets	PASS/FAIL	<detail>
No injection	PASS/FAIL	<detail>
```

## Hallucination Detection
```tsv
Check	Status	Details
Imports	PASS/WARN	<detail>
Signatures	PASS/WARN	<detail>
```

## Pipeline Integrity
```tsv
Check	Status	Details
Structure	PASS/FAIL	<detail>
Cross-refs	PASS/FAIL	<detail>
```

## Eval Results
```tsv
Check	Command	Result
<grader>	<command>	PASS/FAIL
```

## E2E Grounding (D.5)
<!-- Include ONLY for UI tasks; otherwise write "N/A (non-UI task)" -->
```tsv
Check	Status	Details
Committed	PASS/FAIL	tests/e2e/<slug>.spec.ts
Selectors	PASS/FAIL	<N> locators verified
No skip/only	PASS/FAIL	—
Artifacts	PASS/FAIL	playwright-report/index.html
E2E Verify	PASS/FAIL	—
```

<!-- ANCHOR:defects -->
## Issues
```tsv
Severity	Description	File	Line
HIGH	<issue>	<file>	<line>
```

## Self-Evolution Assessment
- **Blast radius:** low/medium/high
- **Reversibility:** easy/moderate/hard
- **Convergence:** advancing/neutral/thrashing
- **Compound effect:** beneficial/neutral/harmful
```

### Ledger Entry
```json
{"ts":"<ISO-8601>","cycle":<N>,"role":"auditor","type":"audit","data":{"verdict":"PASS|WARN|FAIL","confidence":<0.0-1.0>,"challenge":"<token>","prevHash":"<hash of previous ledger entry>","issues":{"critical":<N>,"high":<N>,"medium":<N>,"low":<N>},"evalChecks":{"total":<N>,"passed":<N>,"failed":<N>},"blastRadius":"low|medium|high"}}

---

## Section: hypothesis-falsification-example

Example `falsifiable_claims[]` entry for `handoff-auditor.json`:

```json
{
  "falsifiable_claims": [
    {
      "id": "C70-P2-turn-budget",
      "hypothesis": "advisory turn-budget will reduce builder turns to <=20",
      "verification_artifact": ".evolve/runs/cycle-{NEXT}/builder-usage.json",
      "verification_field": "num_turns",
      "predicted_value": "<=20",
      "tolerance_pct": 10,
      "consequence_if_falsified": "escalate to programmatic kill (Case A); mark advisory as INERT with re_attempt_by_cycle"
    }
  ]
}
```
