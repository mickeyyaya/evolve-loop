package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	journeyPkg  = "go/internal/flaky/flaky.go"
	journeyTest = "go/internal/flaky/flaky_test.go"
)

func flakyBuild(t *testing.T, failures int) (handoffReplay, string) {
	t.Helper()
	r := newHandoffReplay(t, 2151, nil)
	r.writeRunState(t, 0)
	counter := filepath.Join(t.TempDir(), "runs")
	test := fmt.Sprintf("package flaky\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestTmuxBootUnderLoad(t *testing.T) {\n\tb, _ := os.ReadFile(%q)\n\t_ = os.WriteFile(%q, append(b, 'x'), 0o644)\n\tif len(b) < %d {\n\t\tt.Fatal(\"session boot timed out\")\n\t}\n\tif Boot() != 1 {\n\t\tt.Fatal(\"Boot\")\n\t}\n}\n", counter, counter, failures)
	r.commitBuild(t, map[string]string{
		journeyPkg:  "package flaky\n\nfunc Boot() int { return 1 }\n",
		journeyTest: test,
	})
	r.writeWorkspace(t, "build-report.md", "# Build Report — cycle 2151\n\n## Changes\n- `"+journeyPkg+"`\n- `"+journeyTest+"`\n")
	return r, counter
}

func selfcheckWithStderr(t *testing.T, r handoffReplay) (int, string) {
	t.Helper()
	var code int
	var out string
	stderr := captureProcessStderr(t, func() { code, out = r.selfcheck() })
	return code, stderr + out
}

func TestBuildFloorJourney_Cycle2151_AOneOffRedIsRetakenAndTheBuildGoesOn(t *testing.T) {
	r, counter := flakyBuild(t, 1)

	code, out := selfcheckWithStderr(t, r)

	if code != 0 || !strings.Contains(out, "GREEN") {
		t.Fatalf("cycle 2151: one red that passes on the serialized retake must not end the build; `evolve selfcheck build` exit=%d\n%s", code, out)
	}
	if !strings.Contains(out, string(core.CodeBuildFloorRetakeGreen)) || !strings.Contains(out, "TestTmuxBootUnderLoad") {
		t.Fatalf("the retake must be visible and name the test:\n%s", out)
	}

	if err := os.Remove(counter); err != nil {
		t.Fatal(err)
	}
	cs, err := core.ReadRunCycleState(r.workspace, "")
	if err != nil {
		t.Fatal(err)
	}
	in := core.ReviewInputFor(cs, core.PhaseBuild, r.root)
	in.Signals = signalcenter.New()
	var res core.ReviewResult
	captureProcessStderr(t, func() { res = probeBuildHandoffFloor(r.root).Review(context.Background(), in) })
	if !res.Approve {
		t.Fatalf("the host floor must approve the handoff after the green retake: %+v", res)
	}
	var named bool
	for _, e := range in.Signals.Recent() {
		named = named || (e.Code == core.CodeBuildFloorRetakeGreen && e.Fields["tests"] == "TestTmuxBootUnderLoad" && strings.Contains(e.Fields["pkg"], "internal/flaky"))
	}
	if !named {
		t.Fatalf("the host floor must emit %s that names the test and its package: %+v", core.CodeBuildFloorRetakeGreen, in.Signals.Recent())
	}
}

func TestBuildFloorJourney_Cycle2151_ASecondRedFailsTheFloorAndNamesTheTest(t *testing.T) {
	r, _ := flakyBuild(t, 1000)

	code, out := selfcheckWithStderr(t, r)

	if code != 1 || !strings.Contains(out, "--- FAIL: TestTmuxBootUnderLoad") || !strings.Contains(out, string(core.CodeBuildFloorRetakeRed)) {
		t.Fatalf("a red retake must fail the floor and name the test; exit=%d\n%s", code, out)
	}
}
