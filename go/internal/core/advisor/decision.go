package advisor

import "github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"

// decision is the closed enum of the advisor's three calls; the zero value is the initial Plan.
// Being closed keeps the <kind> token in the capture filenames unconstructible from outside.
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
	decisionProposal: {contract: "router-proposal", kind: "proposal", completion: "artifact", errPfx: "routing proposer", origin: "Advisor.Propose"},
}

// contractID is the deliverable protocol, separate from the router identity that resolves model and profile.
func (d decision) contractID() string { return decisionRows[d].contract }

// artifactFile is projected from the contract registry, so the (contract, artifact) pair is spelled once.
func (d decision) artifactFile() string { return phasecontract.ArtifactName(d.contractID()) }

// captureKind is the <kind> token in advisor-{prompt,response,span}-<kind>.* and the decision field on events.
func (d decision) captureKind() string { return decisionRows[d].kind }

// completion is "artifact" for every decision: each prompt tells the model to write its artifact,
// so the scrollback holds only the prompt's echoed example.
func (d decision) completion() string { return decisionRows[d].completion }

func (d decision) errPfx() string { return decisionRows[d].errPfx }

// origin is the exported method whose call produced an event.
func (d decision) origin() string { return decisionRows[d].origin }

func (d decision) replanDepth() int { return decisionRows[d].depth }

// decisionForArtifact maps anything but the re-plan artifact to Plan, since the test facade passes arbitrary names.
func decisionForArtifact(artifactFile string) decision {
	if artifactFile == decisionRePlan.artifactFile() {
		return decisionRePlan
	}
	return decisionPlan
}
