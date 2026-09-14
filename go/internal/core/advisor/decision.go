package advisor

import "github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"

// decision is the closed enum of the advisor's three calls — the initial
// whole-cycle Plan (the zero value), the post-scout RePlan and the
// per-transition Propose — folding the per-call literals (contract id,
// capture kind, completion contract, error prefix, span depth, signal
// origin) into ONE table. The artifact the model writes is projected from
// the phase-contract registry, which already holds the same names, so the
// (contract, artifact) pair is spelled once. The capture kind is closed by
// construction, so the <kind> token in the capture filenames is
// unconstructible from outside (the former isSafeArtifactKind guard).
type decision int

const (
	decisionPlan decision = iota
	decisionRePlan
	decisionProposal
)

type decisionRow struct {
	contract   string
	kind       string
	completion string
	errPfx     string
	origin     string
	depth      int
}

var decisionRows = [...]decisionRow{
	decisionPlan:     {contract: "router", kind: "plan", completion: "artifact", errPfx: "phase advisor", origin: "Advisor.Plan"},
	decisionRePlan:   {contract: "router-replan", kind: "replan", completion: "artifact", errPfx: "phase advisor", origin: "Advisor.RePlan", depth: 1},
	decisionProposal: {contract: "router-proposal", kind: "proposal", completion: "stdout", errPfx: "routing proposer", origin: "Advisor.Propose"},
}

// contractID is the deliverable protocol selected for the decision. It stays
// separate from the shared router agent identity used for model and profile
// resolution.
func (d decision) contractID() string { return decisionRows[d].contract }

// artifactFile is the raw artifact the model writes (routing-plan.json /
// routing-replan.json / routing-proposal.json), projected from the contract
// registry.
func (d decision) artifactFile() string { return phasecontract.ArtifactName(d.contractID()) }

// captureKind is the <kind> token in the capture filenames
// (advisor-{prompt,response,span}-<kind>.*) and the decision field on events.
func (d decision) captureKind() string { return decisionRows[d].kind }

// completion is the bridge completion contract: the plans use the uniform
// artifact contract (the brain WRITES its artifact and the bridge reads it
// back); the proposal still completes on REPL-idle stdout (ADR-0027).
func (d decision) completion() string { return decisionRows[d].completion }

// errPfx prefixes every error the decision returns — the texts the
// orchestrator prints.
func (d decision) errPfx() string { return decisionRows[d].errPfx }

// origin is the exported method whose call produced an event.
func (d decision) origin() string { return decisionRows[d].origin }

// replanDepth is the depth stamped on the decision span: a re-plan is one
// level deeper than the initial plan.
func (d decision) replanDepth() int { return decisionRows[d].depth }

// decisionForArtifact maps a plan artifact back onto its decision for the
// stamp on a compose-time event: the re-plan artifact ⇒ RePlan, anything
// else ⇒ Plan (the composer is also reached through the test facade with an
// arbitrary artifact name).
func decisionForArtifact(artifactFile string) decision {
	if artifactFile == decisionRePlan.artifactFile() {
		return decisionRePlan
	}
	return decisionPlan
}
