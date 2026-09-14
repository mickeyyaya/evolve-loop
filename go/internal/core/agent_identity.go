package core

import "github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"

// AgentIdentity is the immutable dispatch identity shared by the control-plane
// advisors (PhaseAdvisor, FailureAdvisor) — the fields that select WHICH llm
// brain answers, independent of the per-call operand (prompt / artifact file /
// completion contract, which vary Plan vs Propose vs Advise and stay per-call
// params). It formalizes the byte-identical {cli,model,profile,persona} field
// set both advisors carried separately (ADR-0052 WS1-S1, Value Object): one
// home per identity belief, never two structs drifting apart. Since ADR-0103
// unit 04 that home is the advisor leaf's Identity; this alias keeps every
// core spelling (the failure advisor, the retry adjudicator, the tests).
//
// It is deliberately NOT the bridge-launch call itself — the advisors thread
// context differently (PhaseAdvisor uses context.Background; FailureAdvisor
// threads the caller's ctx) — so only the field-set used to build BridgeRequest
// is shared, per the ADR's critic note.
type AgentIdentity = advisor.Identity
