package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
)

type gateOutcome struct {
	files                     []string
	verdict                   string
	shipEligible              bool
	green, red, skip, results int
	redIDs                    []string
}

func auditGateOutcome(t *testing.T, stage string, cycle int) gateOutcome {
	t.Helper()
	root, ws := tiaFixture(t, stage, cycle)
	return gateOutcomeOf(t, root, ws, stage, cycle)
}

func gateOutcomeOf(t *testing.T, root, ws, stage string, cycle int) gateOutcome {
	t.Helper()
	if err := generateACSVerdict(core.PhaseRequest{Worktree: root, ProjectRoot: root, Workspace: ws, Cycle: cycle}); err != nil {
		t.Fatalf("generateACSVerdict (stage %q): %v", stage, err)
	}
	entries, err := os.ReadDir(ws)
	if err != nil {
		t.Fatal(err)
	}
	var out gateOutcome
	for _, e := range entries {
		out.files = append(out.files, e.Name())
	}
	slices.Sort(out.files)
	raw, err := os.ReadFile(filepath.Join(ws, "acs-verdict.json"))
	if err != nil {
		t.Fatalf("no acs-verdict.json (stage %q): %v", stage, err)
	}
	var v acssuite.Verdict
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	out.verdict, out.shipEligible, out.redIDs = v.Verdict, v.ShipEligible, v.RedIDs
	out.green, out.red, out.skip, out.results = v.GreenCount, v.RedCount, v.SkipCount, len(v.Results)
	return out
}

func TestGenerateACSVerdict_TheShadowStageChangesNoGateDecision(t *testing.T) {
	t.Parallel()
	off := auditGateOutcome(t, "off", 4250)
	shadow := auditGateOutcome(t, "shadow", 4250)

	wantFiles := slices.Sorted(slices.Values(append(slices.Clone(off.files), regressiontia.ArtifactName)))
	if !reflect.DeepEqual(shadow.files, wantFiles) {
		t.Errorf("shadow files = %v, want the files of off %v plus %s alone", shadow.files, off.files, regressiontia.ArtifactName)
	}
	off.files, shadow.files = nil, nil
	if !reflect.DeepEqual(shadow, off) {
		t.Errorf("shadow verdict %+v differs from off %+v: the shadow stage must decide nothing", shadow, off)
	}
}

func corpusFixture(t *testing.T, stage string, cycle int) (string, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(".evolve/policy.json", `{"regression_tia":{"stage":"`+stage+`"}}`+"\n")
	repo.Write("go/go.mod", "module example.com/m\n\ngo 1.23\n")
	repo.Write("go/acs/regression/guard/guard_test.go", "//go:build acs\n\npackage guard\n\nimport \"testing\"\n\nfunc TestGuardHolds(t *testing.T) {}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "a regression corpus")
	ws := filepath.Join(repo.Dir, ".evolve", "runs", "cycle-"+itoa(cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	return repo.Dir, ws
}

func TestGenerateACSVerdict_AShadowDecisionThatWouldSkipTheCorpusStillRunsIt(t *testing.T) {
	orig := tiaCompute
	t.Cleanup(func() { tiaCompute = orig })
	tiaCompute = func(stage, repoRoot, moduleDir string, changed []string) regressiontia.Decision {
		d := orig(stage, repoRoot, moduleDir, changed)
		d.Selected, d.WouldSkip, d.WouldSkipCount = nil, []string{"./acs/regression/guard"}, 1
		return d
	}
	offRoot, offWS := corpusFixture(t, "off", 4251)
	shadowRoot, shadowWS := corpusFixture(t, "shadow", 4251)

	off := gateOutcomeOf(t, offRoot, offWS, "off", 4251)
	shadow := gateOutcomeOf(t, shadowRoot, shadowWS, "shadow", 4251)

	raw, err := os.ReadFile(filepath.Join(shadowWS, regressiontia.ArtifactName))
	if err != nil {
		t.Fatal(err)
	}
	var d regressiontia.Decision
	if err := json.Unmarshal(raw, &d); err != nil || !slices.Contains(d.WouldSkip, "./acs/regression/guard") {
		t.Fatalf("precondition: the shadow decision must skip the corpus package (%+v, %v)", d, err)
	}
	if off.green == 0 {
		t.Fatalf("precondition: the off run must execute the corpus predicate: %+v", off)
	}
	off.files, shadow.files = nil, nil
	if !reflect.DeepEqual(shadow, off) {
		t.Errorf("shadow verdict %+v differs from off %+v: a would-skip package must still run in shadow", shadow, off)
	}
}

func TestEmitTIADecision_AnEmptyDecisionWritesNoArtifact(t *testing.T) {
	orig := tiaCompute
	t.Cleanup(func() { tiaCompute = orig })
	tiaCompute = func(string, string, string, []string) regressiontia.Decision { return regressiontia.Decision{} }
	root, ws := tiaFixture(t, "shadow", 4252)

	emitTIADecision(core.PhaseRequest{Worktree: root, ProjectRoot: root, Workspace: ws, Cycle: 4252}, root)

	if _, err := os.Stat(filepath.Join(ws, regressiontia.ArtifactName)); !os.IsNotExist(err) {
		t.Errorf("an empty decision must write no %s (stat err %v)", regressiontia.ArtifactName, err)
	}
}
