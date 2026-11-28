package looppreflight

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

func stageSource(res stageevidence.CheckResult, err error) func() (stageevidence.CheckResult, error) {
	return func() (stageevidence.CheckResult, error) { return res, err }
}

func TestCheckStageContract_MapsEachOutcomeToALevel(t *testing.T) {
	t.Parallel()
	finding := []stageevidence.Finding{{Key: "a.stage", Reason: "stage is enforce with no accepted flip record"}}
	cases := map[string]struct {
		source func() (stageevidence.CheckResult, error)
		level  CheckLevel
		detail string
	}{
		"not wired passes":                   {nil, LevelPass, ""},
		"inactive without findings passes":   {stageSource(stageevidence.CheckResult{Mode: stageevidence.ModeInactive}, nil), LevelPass, ""},
		"inactive with findings warns":       {stageSource(stageevidence.CheckResult{Mode: stageevidence.ModeInactive, Findings: finding}, nil), LevelWarn, "a.stage"},
		"report-only with findings warns":    {stageSource(stageevidence.CheckResult{Mode: stageevidence.ModeReportOnly, Findings: finding}, nil), LevelWarn, "a.stage"},
		"report-only without findings pass":  {stageSource(stageevidence.CheckResult{Mode: stageevidence.ModeReportOnly}, nil), LevelPass, ""},
		"active with findings still warns":   {stageSource(stageevidence.CheckResult{Mode: stageevidence.ModeActive, Findings: finding}, nil), LevelWarn, "a.stage"},
		"refuse halts":                       {stageSource(stageevidence.CheckResult{Mode: stageevidence.ModeRefuse, Findings: finding}, nil), LevelHalt, "a.stage"},
		"an unreadable source warns":         {stageSource(stageevidence.CheckResult{}, errors.New("ledger down")), LevelWarn, "ledger down"},
		"a broken ledger does not halt here": {stageSource(stageevidence.CheckResult{}, stageevidence.ErrLedgerBroken), LevelWarn, "ledger broken"},
	}
	for name, c := range cases {
		got := CheckStageContract(c.source)
		if got.Name != "stage-contract" || got.Level != c.level || !strings.Contains(got.Detail+got.Message, c.detail) {
			t.Errorf("%s: %+v, want level %v containing %q", name, got, c.level, c.detail)
		}
	}
}

func TestRun_StageContractCheckRunsAndOnlyTheRefuseRowHalts(t *testing.T) {
	t.Parallel()
	opts := goodPipelineOptions(t)
	opts.StageContract = stageSource(stageevidence.CheckResult{Mode: stageevidence.ModeReportOnly}, nil)
	r, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	if c := findCheck(t, r, "stage-contract"); c.Level != LevelPass || r.Halted() {
		t.Fatalf("report-only: check %+v halted=%v", c, r.Halted())
	}
	opts.StageContract = stageSource(stageevidence.CheckResult{Mode: stageevidence.ModeRefuse, Findings: []stageevidence.Finding{{Key: stageevidence.GenesisFindingKey}}}, nil)
	r, err = Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	if c := findCheck(t, r, "stage-contract"); c.Level != LevelHalt || !r.Halted() {
		t.Fatalf("refuse: check %+v halted=%v", c, r.Halted())
	}
}
