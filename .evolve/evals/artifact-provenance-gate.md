# Eval: artifact-provenance-gate

Tests that cycle artifacts carry `<!-- evolve:provenance ... -->` headers and that a
gate validates/rejects them. Covers phasecoherence.CheckProvenance and the `phases verify`
integration point.

## Code Graders (bash commands that must exit 0)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -run TestProvenance -v 2>&1 | grep -E "^(ok|PASS|--- PASS)"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -run TestProvenanceGate_MissingHeader_ReturnsViolation -v 2>&1 | grep "PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -run TestProvenanceGate_ValidHeader_NoViolation -v 2>&1 | grep "PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -run TestProvenanceGate_TamperedPhase_ReturnsViolation -v 2>&1 | grep "PASS"`

## Regression Evals (full test suite)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... 2>&1 | tail -3 | grep "^ok"`

## Acceptance Checks

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./... 2>&1 | grep -c "^" | awk '{if($1==0) exit 0; else exit 1}'`
- `[code]` `grep -r "CheckProvenance\|ProvenanceHeader\|provenanceGate" /Users/danleemh/ai/claude/evolve-loop/go/internal/phasecoherence/ --include="*.go" -l | grep -c "." | awk '{if($1>=1) exit 0; else exit 1}'`

## Negative Graders (must reject — gaming check)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -run TestProvenanceGate_MissingHeader_ReturnsViolation -v 2>&1 | grep -v "FAIL"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasecoherence/... -run TestProvenanceGate_WrongCycle_ReturnsViolation -v 2>&1 | grep "PASS"`
