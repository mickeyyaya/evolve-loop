package core

// AgentIdentity is the immutable dispatch identity shared by the control-plane
// advisors (PhaseAdvisor, FailureAdvisor) — the fields that select WHICH llm
// brain answers, independent of the per-call operand (prompt / artifact file /
// completion contract, which vary Plan vs Propose vs Advise and stay per-call
// params). It formalizes the byte-identical {cli,model,profile,persona} field
// set both advisors carried separately (ADR-0052 WS1-S1, Value Object): one
// home per identity belief, never two structs drifting apart.
//
// It is deliberately NOT the bridge-launch call itself — the advisors thread
// context differently (PhaseAdvisor uses context.Background; FailureAdvisor
// threads the caller's ctx) — so only the field-set used to build BridgeRequest
// is shared, per the ADR's critic note.
type AgentIdentity struct {
	CLI        string // dispatch CLI (claude-tmux / codex / agy); resolved from profile + env by the composition root
	Model      string // model tier requested (haiku/sonnet/opus or a raw family model)
	Profile    string // profile path; when empty the advisor derives it from RouteInput.ProjectRoot
	Persona    string // persona body (agents/evolve-*.md); empty ⇒ legacy inline framing
	AgentLabel string // bridge "Agent" role tag (router / failure-advisor)
}
