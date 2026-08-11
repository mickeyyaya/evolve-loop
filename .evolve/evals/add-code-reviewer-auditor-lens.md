# Eval: Add evolve-code-reviewer Auditor Fan-out Lens

## Code Graders (bash commands that must exit 0)

### G1 — Profile structural completeness (Level 3 / jq multi-field)
- `jq -e 'has("parallel_eligible") and has("challenge_token_required") and has("sandbox") and has("model_tier_default") and (.model_tier_default | test("^(sonnet|haiku|opus)$"))' .evolve/profiles/code-reviewer.json`

### G2 — Profile boolean invariants (Level 3 / jq)
- `jq -e '.parallel_eligible == true and .challenge_token_required == true and .sandbox.read_only_repo == true' .evolve/profiles/code-reviewer.json`

### G3 — Persona adversarial mandate in correct section (Level 3.5 / awk)
- `awk '/^#/{section=$0} /MUST find at least one HIGH/{if(section ~ /[Aa]udit|[Mm]andat|[Aa]dversar/){found=1}} END{exit(!found)}' agents/evolve-code-reviewer.md`

### G4 — Flag guards non-trivial dispatch body (Level 3.5 / awk)
- `awk '/EVOLVE_FANOUT_AUDITOR_CODE_REVIEWER.*==.*1/{p=1} p && /subagent-run.*code-reviewer/{found=1} END{exit(!found)}' scripts/lifecycle/phase-gate.sh`

### G5 — subagent-run.sh has named routing for code-reviewer (Level 3.5 / awk)
- `awk '/code-reviewer\)/{p=1} p && /\.json/{found=1} END{exit(!found)}' scripts/dispatch/subagent-run.sh`

### G6 — Regression: flag appears at least once in phase-gate (count-based)
- `grep -c "EVOLVE_FANOUT_AUDITOR_CODE_REVIEWER" scripts/lifecycle/phase-gate.sh | awk '{exit($1<1)}'`

## Thresholds
- All checks: pass@1 = 1.0
