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
