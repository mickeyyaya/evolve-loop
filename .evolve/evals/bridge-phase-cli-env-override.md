# Eval: bridge-phase-cli-env-override

## Task
`core/failure_advisor.go:76` and `core/phase_advisor.go:76` hardcode `"claude-tmux"` as
the default CLI without consulting `EVOLVE_CLI`. This is an abstraction violation: components
that pick a CLI for the bridge should respect `EVOLVE_CLI` the same way `retro.go:95-97` and
`adapters/observer/core_adapter.go:227-230` do. Fix: add `EVOLVE_CLI` env override support
to `NewFailureAdvisor` / `NewPhaseAdvisor` constructors (via an env map option or by updating
the constructor signature), ensuring `EVOLVE_CLI` takes effect when no explicit CLI option
is given. Reference pattern: `phaseCLI()` in `core_adapter.go`.

## Criteria

### C1 — TestFailureAdvisorRespectsCLIEnv passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/... -run TestFailureAdvisorRespectsCLIEnv -v -count=1 2>&1 | grep -E "PASS|FAIL|--- "
```
Expected: `--- PASS: TestFailureAdvisorRespectsCLIEnv` — constructing `NewFailureAdvisor`
with an env map containing `EVOLVE_CLI=codex-tmux` causes the advisor to use `codex-tmux`
rather than the hardcoded `"claude-tmux"`.

### C2 — TestPhaseAdvisorRespectsCLIEnv passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/... -run TestPhaseAdvisorRespectsCLIEnv -v -count=1 2>&1 | grep -E "PASS|FAIL|--- "
```
Expected: `--- PASS: TestPhaseAdvisorRespectsCLIEnv` — same contract for `PhaseAdvisor`.

### C3 — Negative: explicit WithProposerCLI / WithFailureAdvisorCLI still wins [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/... -run TestAdvisorExplicitCLIOverridesEnv -v -count=1 2>&1 | grep -E "PASS|FAIL|--- "
```
Expected: `--- PASS` — explicit CLI option (the existing `WithProposerCLI`) takes precedence
over the env-derived default; EVOLVE_CLI does not override an explicitly supplied CLI.

### C4 — Hardcoded "claude-tmux" string only appears inside bridge package or with env fallback comment [code]
```bash
grep -rn '"claude-tmux"' /Users/danleemh/ai/claude/evolve-loop/go/internal/core/ | grep -v "_test.go\|//" | head -10
```
Expected: any remaining `"claude-tmux"` literal in `core/` appears only as a fallback in
a function that first checks `EVOLVE_CLI` (not as a bare struct-literal default).

### C5 — Core package tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/... -count=1 -short -timeout 60s 2>&1 | grep -E "^ok|FAIL"
```
Expected: `ok`, no `FAIL`.
