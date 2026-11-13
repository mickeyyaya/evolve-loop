//go:build acs

package cycle2357

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modulePath = "github.com/mickeyyaya/evolve-loop/go/"

var baseTree = map[string]string{
	"src/mat.go":      "package src // base\n",
	"src/other.go":    "package src // other base\n",
	"src/gen/keep.go": "package gen // base\n",
}

const (
	grantFileOnly = `{"writable":["src/mat.go"]}`
	grantFileDir  = `{"writable":["src/mat.go"],"writable_dirs":["src/gen"]}`
	seatEdit      = "package src // seat edit\n"
)

type seatBridge struct {
	writes  map[string]string
	removes []string
}

func (b seatBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	for rel, content := range b.writes {
		abs := filepath.Join(req.Worktree, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return core.BridgeResponse{}, err
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return core.BridgeResponse{}, err
		}
	}
	for _, rel := range b.removes {
		if err := os.Remove(filepath.Join(req.Worktree, filepath.FromSlash(rel))); err != nil {
			return core.BridgeResponse{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755); err != nil {
		return core.BridgeResponse{}, err
	}
	return core.BridgeResponse{Stdout: "ok"}, os.WriteFile(req.ArtifactPath, []byte("# ok\n"), 0o644)
}

func (seatBridge) Probe(context.Context) (core.BridgeProbe, error) { return core.BridgeProbe{}, nil }

func runSeat(t *testing.T, fenceJSON string, bridge seatBridge) (string, core.PhaseResponse) {
	t.Helper()
	repo := gittest.Fixture(t)
	for rel, content := range baseTree {
		abs := filepath.Join(repo.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	var fence core.WorktreeFence
	if err := json.Unmarshal([]byte(fenceJSON), &fence); err != nil {
		t.Fatalf("fence %s: %v", fenceJSON, err)
	}
	spec := phasespec.PhaseSpec{Name: "builder", Classify: &phasespec.ClassifyRules{VerdictOnPass: core.VerdictPASS}}
	phase := specrunner.New(spec, specrunner.Config{Bridge: bridge, Prompts: prompts.NewFromFS(fstest.MapFS{}), PromptBody: "PERSONA"})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: repo.Dir, Fence: &fence})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return repo.Dir, resp
}

func content(dir, rel string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	return string(raw), err == nil
}

func errorNaming(resp core.PhaseResponse, needles ...string) bool {
	for _, d := range resp.Diagnostics {
		if d.Severity == "error" && !slices.ContainsFunc(needles, func(n string) bool { return !strings.Contains(d.Message, n) }) {
			return true
		}
	}
	return false
}

func anyError(resp core.PhaseResponse) bool {
	return slices.ContainsFunc(resp.Diagnostics, func(d core.Diagnostic) bool { return d.Severity == "error" })
}

func requireTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	args := []string{"test", "-v", "-count=1", "-run", "^(" + strings.Join(names, "|") + ")$", modulePath + pkg}
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	if code != 0 {
		t.Fatalf("RED: go %v exited %d\n%s", args, code, acsassert.OutputTail(stdout+stderr, 30))
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Fatalf("RED: %s never ran to PASS in %s\n%s", name, pkg, acsassert.OutputTail(stdout+stderr, 30))
		}
	}
}

func TestC2357_001_AWriterSeatThatCreatesAFileOutsideItsGrantIsNotPASSAndTheSignalNamesThePathAndTheNextStep(t *testing.T) {
	requireTestsPass(t, "internal/cli/agentcmd",
		"TestAgentRun_TheBuildSeatThatCreatesAFileOutsideItsGrantEndsWithAVerdictThatIsNotPASS",
		"TestAgentRun_TheBuildSeatThatCreatesAFileOutsideItsGrantGetsAnErrorThatNamesThePathAndTheGrant")
}

func TestC2357_007_TheAgentRunJourneyEndsNotPASSAndNamesTheRemovedPath(t *testing.T) {
	requireTestsPass(t, "internal/cli/agentcmd",
		"TestAgentRun_TheJourneyOfASeatThatCreatesAFileOutsideItsGrantExitsNonZeroAndRecordsTheVerdict",
		"TestAgentRun_TheBuildSeatKeepsANewFileInAWritableDirAndRestoresAnExistingFileOutsideIt")
}

func TestC2357_002_AWriterSeatThatStaysInsideItsGrantStillPasses(t *testing.T) {
	dir, resp := runSeat(t, grantFileOnly, seatBridge{writes: map[string]string{"src/mat.go": seatEdit}})

	if got, _ := content(dir, "src/mat.go"); got != seatEdit {
		t.Errorf("the granted file lost the seat edit: %q", got)
	}
	if resp.Verdict != core.VerdictPASS || anyError(resp) {
		t.Errorf("verdict %s diagnostics %+v: a seat inside its grant must pass without an error", resp.Verdict, resp.Diagnostics)
	}
}

func TestC2357_003_AnExistingFileOutsideTheGrantIsStillRestored(t *testing.T) {
	cases := []struct {
		name   string
		bridge seatBridge
	}{
		{"edited", seatBridge{writes: map[string]string{"src/mat.go": seatEdit, "src/other.go": "package src // seat tampered\n"}}},
		{"deleted", seatBridge{writes: map[string]string{"src/mat.go": seatEdit}, removes: []string{"src/other.go"}}},
		{"edited beside a granted directory", seatBridge{writes: map[string]string{"src/gen/new.go": "package gen // new\n", "src/other.go": "package src // seat tampered\n"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, _ := runSeat(t, grantFileDir, tc.bridge)

			if got, ok := content(dir, "src/other.go"); !ok || got != baseTree["src/other.go"] {
				t.Errorf("an existing file outside the grant was not restored: present=%v content=%q", ok, got)
			}
		})
	}
}

func TestC2357_004_ANewFileInAGrantedDirectorySurvivesAndTheSeatPasses(t *testing.T) {
	dir, resp := runSeat(t, grantFileDir, seatBridge{writes: map[string]string{"src/mat.go": seatEdit, "src/gen/new.go": "package gen // new\n"}})

	if got, kept := content(dir, "src/gen/new.go"); !kept || got != "package gen // new\n" {
		t.Errorf("the new file in the granted directory did not survive: kept=%v content=%q", kept, got)
	}
	if resp.Verdict != core.VerdictPASS || anyError(resp) {
		t.Errorf("verdict %s diagnostics %+v: a new file inside the grant is no fault", resp.Verdict, resp.Diagnostics)
	}
}

func TestC2357_005_ADirectoryGrantDoesNotCoverASiblingOrALaunderedStray(t *testing.T) {
	cases := []struct {
		name    string
		writes  map[string]string
		removed string
	}{
		{"a sibling that shares the name prefix", map[string]string{"src/gen2/x.go": "package gen2\n"}, "src/gen2/x.go"},
		{"a file beside the granted directory", map[string]string{"src/gen/new.go": "package gen // new\n", "src/stray.go": "package src\n"}, "src/stray.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, resp := runSeat(t, grantFileDir, seatBridge{writes: tc.writes})

			if _, kept := content(dir, tc.removed); kept {
				t.Errorf("%s is outside every grant and must be removed", tc.removed)
			}
			if !errorNaming(resp, tc.removed, "--writable") {
				t.Errorf("diagnostics %+v: want an error that names %s and the --writable next step", resp.Diagnostics, tc.removed)
			}
		})
	}
}

func TestC2357_006_ARequestThatGrantsOnlyADirectoryIsAWriter(t *testing.T) {
	var req core.PhaseRequest
	if err := json.Unmarshal([]byte(`{"worktree_fence":{"writable_dirs":["src/gen"]}}`), &req); err != nil {
		t.Fatal(err)
	}

	if req.Fence == nil || !req.WritesSource() {
		t.Errorf("fence %+v WritesSource=%v: a request with a directory grant and no file grant must still write source", req.Fence, req.WritesSource())
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var again core.PhaseRequest
	if err := json.Unmarshal(raw, &again); err != nil || !again.WritesSource() || !strings.Contains(string(raw), "writable_dirs") {
		t.Errorf("the directory grant was lost across a JSON round trip: err=%v WritesSource=%v wire=%s", err, again.WritesSource(), raw)
	}
}

func TestC2357_008_TheAgentRunWritableDirGrantReachesTheFenceAndIsRefusedWhenUnsafe(t *testing.T) {
	requireTestsPass(t, "internal/cli/agentcmd",
		"TestAgentRun_TheBuildSeatThatCreatesAFileOutsideItsGrantGetsAnErrorThatNamesThePathAndTheGrant",
		"TestAgentRun_TheBuildSeatKeepsANewFileInAWritableDirAndRestoresAnExistingFileOutsideIt",
		"TestAgentRun_TheBuildSeatAcceptsAWritableDirAsItsOnlyGrant",
		"TestAgentRun_TheBuildSeatRefusesAWritableDirThatIsNotACleanLocalDirectory",
		"TestAgentRun_AReaderRefusesAWritableDir",
		"TestAgentRun_TheUsageStatesTheWritableDirGrant")
}
