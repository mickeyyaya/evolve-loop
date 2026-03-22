# CLAUDE.md

## Autonomous Execution

If the user is in bypass permissions mode (yolo mode), YOU MUST:

1. **Continue all cycles without pausing** — complete every requested cycle end-to-end without stopping to ask for user approval. Never output "should I continue?" or wait for confirmation between cycles.
2. **Run the FULL pipeline every cycle** — launch Scout, Builder, and Auditor agents for every cycle. No shortcuts, no inline edits that skip agents, no batch-claiming multiple cycles. Each cycle must produce workspace artifacts (scout-report.md, build-report.md, audit-report.md).
3. **Run phase-gate.sh at every transition** — the deterministic phase gate script (`scripts/phase-gate.sh`) must execute at every phase boundary. This is non-negotiable even in bypass mode. Bypass permissions means "don't ask the user" — it does NOT mean "skip integrity checks."
4. **Never fabricate cycles** — claiming cycle numbers without doing work is a CRITICAL violation. Every cycle number must correspond to real Scout → Build → Audit → Ship → Learn execution.

**The rule is: maximum velocity, zero shortcuts.** Go fast by being efficient, not by skipping steps.

## Evolve Loop Task Priority

When selecting tasks for `/evolve-loop` cycles, follow this priority order:

1. **New features** — Building new functionality is the top priority
2. **Bug fixes** — Fixing potential bugs is second priority
3. **Security issues** — Fixing security vulnerabilities is last priority
