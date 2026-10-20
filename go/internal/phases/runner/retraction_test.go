package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	amplifiedPredicate = "go/acs/cycle7/amplified_test.go"
	builderPredicate   = "go/acs/cycle7/builder_red_test.go"
	builderRedBody     = "package cycle7 // builder-authored red predicate\n"
)

type amplifyingBridge struct {
	stated        string
	deleteBuilder bool
	noArtifact    bool
}

func (b *amplifyingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	path := filepath.Join(req.Worktree, filepath.FromSlash(amplifiedPredicate))
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte("package cycle7 // $-anchored meta-test\n"), 0o644)
	if b.deleteBuilder {
		_ = os.Remove(filepath.Join(req.Worktree, filepath.FromSlash(builderPredicate)))
	}
	if !b.noArtifact {
		body := "## Generated Tests\n## Results\n"
		if b.stated != "" {
			body += `<!-- evolve-verdict: {"phase":"test-amplification","verdict":"` + b.stated + `","schema_version":1} -->` + "\n"
		}
		_ = os.WriteFile(req.ArtifactPath, []byte(body), 0o644)
	}
	return core.BridgeResponse{Stdout: "ok"}, nil
}

func (b *amplifyingBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func amplificationRepo(t *testing.T) string {
	t.Helper()
	dir := fenceRepo(t)
	path := filepath.Join(dir, filepath.FromSlash(builderPredicate))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(builderRedBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runAmplification(t *testing.T, bridge *amplifyingBridge, retract bool, worktree, workspace string) core.PhaseResponse {
	t.Helper()
	hooks := &fakeHooks{phase: "test-amplification", agent: "evolve-test-amplification", model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	r := New(Options{Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-test-amplification", "body"), RetractOnFail: retract})
	resp, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 7, ProjectRoot: t.TempDir(), Workspace: workspace, Worktree: worktree})
	if err != nil && !bridge.noArtifact {
		t.Fatalf("Run: %v", err)
	}
	return resp
}

func fileBody(t *testing.T, root, rel string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	return string(b), true
}

func retractionDiagnostic(resp core.PhaseResponse) string {
	for _, d := range resp.Diagnostics {
		if strings.HasPrefix(d.Message, "retraction:") {
			return d.Message
		}
	}
	return ""
}

func TestRun_AStatedFailRetractsThePhaseWritesAndCannotLaunderABuilderRed(t *testing.T) {
	t.Parallel()
	dir, ws := amplificationRepo(t), t.TempDir()

	resp := runAmplification(t, &amplifyingBridge{stated: core.VerdictFAIL, deleteBuilder: true}, true, dir, ws)

	if _, present := fileBody(t, dir, amplifiedPredicate); present {
		t.Error("the self-FAILed phase's own test must leave the EGPS tree (cycle-1493 shape)")
	}
	if got, _ := fileBody(t, dir, builderPredicate); got != builderRedBody {
		t.Errorf("a builder red the phase deleted must come back byte-identical, got %q", got)
	}
	keep := filepath.Join(ws, "test-amplification-retracted")
	if got, _ := fileBody(t, keep, amplifiedPredicate); !strings.Contains(got, "meta-test") {
		t.Errorf("the retracted test must be kept as evidence in the workspace, got %q", got)
	}
	msg := retractionDiagnostic(resp)
	for _, want := range []string{"test-amplification stated FAIL", "restored 2 path(s)", amplifiedPredicate, builderPredicate, keep} {
		if !strings.Contains(msg, want) {
			t.Errorf("retraction diagnostic %q must name %q", msg, want)
		}
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("the retraction must not change the routed verdict, got %s", resp.Verdict)
	}
}

func TestRun_AStatedWarnKeepsTheAmplifiedTestThatCaughtADefect(t *testing.T) {
	t.Parallel()
	dir := amplificationRepo(t)

	resp := runAmplification(t, &amplifyingBridge{stated: core.VerdictWARN}, true, dir, t.TempDir())

	if _, present := fileBody(t, dir, amplifiedPredicate); !present {
		t.Error("an amplified test that caught a defect must stay in the tree, where its red blocks ship")
	}
	if msg := retractionDiagnostic(resp); msg != "" {
		t.Errorf("no retraction without a stated FAIL, got %q", msg)
	}
}

func TestRun_AStatedFailIsKeptWhenThePhaseDoesNotOptIn(t *testing.T) {
	t.Parallel()
	dir := amplificationRepo(t)

	runAmplification(t, &amplifyingBridge{stated: core.VerdictFAIL}, false, dir, t.TempDir())

	if _, present := fileBody(t, dir, amplifiedPredicate); !present {
		t.Error("a phase without retract_on_fail keeps its writes")
	}
}

func TestRun_NoReadableReportMeansNoRetraction(t *testing.T) {
	t.Parallel()
	dir := amplificationRepo(t)

	runAmplification(t, &amplifyingBridge{noArtifact: true}, true, dir, t.TempDir())

	if _, present := fileBody(t, dir, amplifiedPredicate); !present {
		t.Error("only a stated FAIL retracts; a missing report keeps the writes for the audit to judge")
	}
}

func TestRun_RetractionSnapshotFailureIsReported(t *testing.T) {
	t.Parallel()
	resp := runAmplification(t, &amplifyingBridge{stated: core.VerdictFAIL}, true, t.TempDir(), t.TempDir())

	if msg := retractionDiagnostic(resp); !strings.Contains(msg, "snapshot unavailable") {
		t.Errorf("an untakeable retraction snapshot must be reported, got %q", msg)
	}
}

func TestRun_RetractionFailureIsReportedAndKeepsTheWrites(t *testing.T) {
	t.Parallel()
	dir, ws := amplificationRepo(t), t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "test-amplification-retracted"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	resp := runAmplification(t, &amplifyingBridge{stated: core.VerdictFAIL}, true, dir, ws)

	if msg := retractionDiagnostic(resp); !strings.Contains(msg, "failed") || !strings.Contains(msg, "restored 0 path(s)") {
		t.Errorf("a failed retraction must be reported, got %q", msg)
	}
	if _, present := fileBody(t, dir, amplifiedPredicate); !present {
		t.Error("a retraction that cannot keep its evidence leaves the writes for the audit")
	}
}

func TestRetractsOnFail_ReportsTheOption(t *testing.T) {
	t.Parallel()
	hooks := &fakeHooks{phase: "test-amplification"}
	if !New(Options{Hooks: hooks, RetractOnFail: true}).RetractsOnFail() {
		t.Error("an opted-in runner must report that it retracts")
	}
	if New(Options{Hooks: hooks}).RetractsOnFail() {
		t.Error("a runner retracts only when its phase opts in")
	}
}
