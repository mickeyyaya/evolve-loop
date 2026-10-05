package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestBuildHandoffProbes_IterateTheCycleFloorsCheckList(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	cycleFloor := wireOrchestratorDeps(root, evolveDir, io.Discard).Orchestrator.BuildHandoffFloorNames()
	probeFloor := probeBuildHandoffFloor(root).Names()
	if !reflect.DeepEqual(cycleFloor, probeFloor) {
		t.Fatalf("the cycle's build handoff floor and the probes' floor must be one check list:\ncycle  %q\nprobes %q", cycleFloor, probeFloor)
	}
	for _, name := range []string{"explanation-documentation", productionFloorCheckName} {
		if !strings.Contains(strings.Join(cycleFloor, ","), name) {
			t.Fatalf("the floor must name %q; got %q", name, cycleFloor)
		}
	}
}

func TestComposedBuildHandoffFloor_FollowsTheWorkflowDialAndTheDocumentContract(t *testing.T) {
	enforced := policy.Policy{}.WorkflowConfig()
	if !enforced.BuildFloorEnforced {
		t.Fatal("the compiled default enforces the build floor")
	}
	code := composedBuildHandoffFloor(enforced, config.RoutingConfig{})
	if !reflect.DeepEqual(code.Names(), []string{productionFloorCheckName}) {
		t.Fatalf("a code cycle's composed floor = %q", code.Names())
	}
	if reflect.ValueOf(code[0].Run).Pointer() != reflect.ValueOf(productionBuildFloorChecks).Pointer() {
		t.Fatal("the production check is productionBuildFloorChecks itself")
	}
	document := config.RoutingConfig{DeliverableKinds: map[string]config.DeliverableKindSpec{config.DeliverableKindDocument: {}}}
	if got := composedBuildHandoffFloor(enforced, document).Names(); !reflect.DeepEqual(got, []string{productionFloorCheckName, solutionFloorCheckName}) {
		t.Fatalf("a document contract adds the solution floor to the one list; got %q", got)
	}
	off := enforced
	off.BuildFloorEnforced = false
	if got := composedBuildHandoffFloor(off, document); len(got) != 0 {
		t.Fatalf("an unenforced build floor composes nothing, for the cycle and the probes alike; got %q", got.Names())
	}
}

func TestBuildHandoffProbes_BothRunTheSeamFloorOnTheBoundCyclesInput(t *testing.T) {
	r := newHandoffReplay(t, 1788, nil)
	r.commitBuild(t, map[string]string{replayLint: replayLintBody})
	r.writeWorkspace(t, "build-report.md", r.reportWithSection(t))
	var inputs []core.ReviewInput
	prev := buildHandoffFloorFor
	t.Cleanup(func() { buildHandoffFloorFor = prev })
	buildHandoffFloorFor = func(string) core.BuildHandoffFloor {
		return core.BuildHandoffFloor{{Name: "recorded", Run: func(_ context.Context, in core.ReviewInput) []string {
			inputs = append(inputs, in)
			return []string{"recorded floor line"}
		}}}
	}
	for name, probe := range map[string]func() (int, string){"phase verify build": r.phaseVerify, "selfcheck build": r.selfcheck} {
		if code, out := probe(); code != 1 || !strings.Contains(out, "recorded floor line") {
			t.Errorf("`%s` must report the floor's failure verbatim and exit 1; exit=%d\n%s", name, code, out)
		}
	}
	want := core.ReviewInput{
		Cycle: r.cycle, RunID: r.runID, ExplanationDocumentationVersion: 1, Phase: string(core.PhaseBuild),
		WorktreeBaseSHA: r.base, Workspace: r.workspace, Worktree: r.worktree, ProjectRoot: r.root,
	}
	if len(inputs) != 2 || !reflect.DeepEqual(inputs[0], want) || !reflect.DeepEqual(inputs[1], want) {
		t.Fatalf("both probes must run the floor once on the bound cycle's input:\ngot  %+v\nwant %+v", inputs, want)
	}
}

func TestSelfcheckBuild_IsThePhaseVerifyBuildPath(t *testing.T) {
	r := newHandoffReplay(t, 1788, nil)
	r.commitBuild(t, map[string]string{
		replayLint:        replayLintBody,
		r.documentPath(t): r.explanation("- `" + replayLint + "` — the AST lint that flags a positive assertion used to prove absence."),
	})
	r.writeWorkspace(t, "build-report.md", "# Build Report — cycle 1788\n\n## Explanation Documentation\n- Status: REQUIRED\n- Document: "+r.documentPath(t)+"\n")
	const contractLine = `required section "## Changes" is missing`
	for name, probe := range map[string]func() (int, string){"phase verify build": r.phaseVerify, "selfcheck build": r.selfcheck} {
		if code, out := probe(); code != 1 || !strings.Contains(out, contractLine) || strings.Contains(out, "Explanation Documentation: ") {
			t.Errorf("`%s` is the one build self-check, the deliverable contract plus the floor: want exit 1 naming only %q\nexit=%d\n%s", name, contractLine, code, out)
		}
	}
}

func captureProcessStderr(t *testing.T, fn func()) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	captured := make(chan string)
	go func() {
		body, _ := io.ReadAll(read)
		captured <- string(body)
	}()
	orig := os.Stderr
	os.Stderr = write
	defer func() { os.Stderr = orig }()
	fn()
	os.Stderr = orig
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	return <-captured
}

func TestPhaseVerifyBuild_BoundDocsFloorReportsAtTheHostsSeverity(t *testing.T) {
	const arch = "go/internal/policy/policy.go"
	r := newHandoffReplay(t, 1790, nil)
	r.writeRunState(t, 0)
	r.write(t, arch, "package policy\n\nconst DocsFloorStage = \"enforce\"\n")
	r.writeWorkspace(t, "build-report.md", "# Build Report — cycle 1790\n\n## Changes\n- `"+arch+"`\n")
	var code int
	var out string
	probeStderr := captureProcessStderr(t, func() { code, out = r.phaseVerify() })
	cs, err := core.ReadRunCycleState(r.workspace, "")
	if err != nil {
		t.Fatal(err)
	}
	in := core.ReviewInputFor(cs, core.PhaseBuild, r.root)
	var hostFloor, hostContract core.ReviewResult
	hostStderr := captureProcessStderr(t, func() {
		hostFloor = probeBuildHandoffFloor(r.root).Review(context.Background(), in)
		hostContract = deliverable.NewReviewer(config.StageEnforce).Review(context.Background(), in)
	})
	if !hostFloor.Approve || !hostContract.Approve || !strings.Contains(hostStderr, "[docs-floor] WARN") {
		t.Fatalf("fixture: the host approves this handoff and only WARNs on the docs floor; floor=%+v contract=%+v\n%s", hostFloor, hostContract, hostStderr)
	}
	if code != 0 || strings.Contains(out, deliverable.CodeMissingArchitectureDocs) || !strings.Contains(probeStderr+out, "[docs-floor] WARN") {
		t.Fatalf("an architecture-class diff with no docs is a host WARN, so the build self-check must exit 0 with the docs floor's WARN line, never fail it: exit=%d\n%s\n%s", code, out, probeStderr)
	}
}
