package ciparity

const (
	gateJobUbuntu = "go / build + test (Go 1.23, ubuntu, every package and every gate)"
	gateJobDarwin = "go / build + test (Go 1.23, macOS, the darwin package set)"
)

type GateStep struct {
	Gate  string
	Job   string
	Step  string
	Bound int
}

var GateSteps = []GateStep{
	{"make cover-changed", gateJobUbuntu, "changed functions at 100% (cover-changed; hard-fail)", 200},
	{"make apicover-check", gateJobUbuntu, "api-coverage enforce (apicover; per-package hard-fail, Phase 5)", 2000},
	{"make cover-strict", gateJobUbuntu, "line-coverage strict (go/.cover-strict; per-package hard-fail, ADR-0101)", 1200},
	{"make cover-changed-darwin", gateJobDarwin, "changed functions of the darwin set at 100% (cover-changed; hard-fail)", 200},
	{"make cover-strict-darwin", gateJobDarwin, "line-coverage strict of the darwin set (go/.cover-strict floors of go/.darwin-set packages; hard-fail)", 1200},
}

func LookupGateStep(job, step string) (GateStep, bool) {
	for _, row := range GateSteps {
		if row.Job == job && row.Step == step {
			return row, true
		}
	}
	return GateStep{}, false
}
