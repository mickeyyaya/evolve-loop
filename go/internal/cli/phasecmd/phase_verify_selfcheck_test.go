package phasecmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestBuildSelfCheck_VerifyReportsTheContractAndTheFloorTogether(t *testing.T) {
	root, ws := floorWorkspace(t)
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("# Build Report\n\nno changes section\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	floor := &recordedFloor{failures: []string{"Explanation Documentation: build-report.md is missing the required ## Explanation Documentation section"}}
	var errb bytes.Buffer
	var got BuildSelfCheckResult
	got, err := BuildSelfCheck{Floor: floor.floorFor, Probe: core.BuildHandoffProbe{Workspace: ws, EvolveDir: filepath.Join(root, ".evolve"), ProjectRoot: root}}.Verify(&errb)
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, v := range got.Violations {
		codes[v.Code] = true
	}
	if got.OK || !codes["missing_section"] || !codes[codeBuildHandoffFloor] || got.Unbound == nil {
		t.Fatalf("one self-check reports the contract violation and the floor failure, and says it could not bind: %+v", got)
	}
}
