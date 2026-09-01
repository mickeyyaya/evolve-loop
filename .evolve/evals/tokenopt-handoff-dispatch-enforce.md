---
score_cap:
  - criterion: "Enforced dispatch supplies bounded handoffs after the static prompt boundary"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/core/... ./internal/phases/runner/... -run 'Handoff|StaticPrefix|PhaseIO'"
  - criterion: "Off/advisory compatibility and malformed input safety are retained"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/core/... -run 'PhaseIO.*(Off|Advisory|Malformed)'"
---

# Eval: token-opt handoff dispatch enforcement

## Criteria

- [code] `cd go && go test -count=1 ./internal/core/... ./internal/phases/runner/... -run 'Handoff|StaticPrefix|PhaseIO'` exits 0.
- [code] At enforce, a completed edge passes the configured bounded digest through the production dispatch seam; no phase re-reads a raw upstream report for that edge.
- [code] At off/advisory, existing prompt behavior remains byte-compatible, including the provider-cache static prefix.
- [model] The digest is dynamic context after the canonical cycle-context boundary, not embedded in the reusable persona prefix.

## Adversarial cases

- Negative: a malformed upstream handoff produces an empty/degraded typed view, never a raw-artifact fallback.
- Edge/OOD: an edge with no completed predecessor gets no handoff and does not change the static prefix.
- Cheapest gaming fake: append a digest to every persona body. The static-prefix equality test fails that fake.
