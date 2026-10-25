package advisor

import "github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"

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
	flow       string
	depth      int
}

var decisionRows = [...]decisionRow{
	decisionPlan:     {contract: "router", kind: "plan", completion: "artifact", errPfx: "phase advisor", origin: "Advisor.Plan", flow: phasecontract.FlowSeatPlan},
	decisionRePlan:   {contract: "router-replan", kind: "replan", completion: "artifact", errPfx: "phase advisor", origin: "Advisor.RePlan", flow: phasecontract.FlowSeatRePlan, depth: 1},
	decisionProposal: {contract: "router-proposal", kind: "proposal", completion: "artifact", errPfx: "routing proposer", origin: "Advisor.Propose", flow: phasecontract.FlowSeatProposal},
}

func (d decision) contractID() string { return decisionRows[d].contract }

func (d decision) artifactFile() string { return phasecontract.ArtifactName(d.contractID()) }

func (d decision) captureKind() string { return decisionRows[d].kind }

func (d decision) completion() string { return decisionRows[d].completion }

func (d decision) errPfx() string { return decisionRows[d].errPfx }

func (d decision) origin() string { return decisionRows[d].origin }

func (d decision) flowSeat() string { return decisionRows[d].flow }

func (d decision) replanDepth() int { return decisionRows[d].depth }

func decisionForArtifact(artifactFile string) decision {
	if artifactFile == decisionRePlan.artifactFile() {
		return decisionRePlan
	}
	return decisionPlan
}
