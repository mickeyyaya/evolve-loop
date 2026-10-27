package agentcmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const diff = "diff --git a/go/x.go b/go/x.go\n+func X() int { return 1 }\n"

func ledgerEntries(t *testing.T, root string) []core.LedgerEntry {
	t.Helper()
	f, err := os.Open(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer f.Close()
	var out []core.LedgerEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e core.LedgerEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("ledger line %q: %v", sc.Text(), err)
		}
		out = append(out, e)
	}
	return out
}

func TestAgentRun_AdversarialReviewDiffOnStdinPrintsTheReportAndLeavesNoCycleFootprint(t *testing.T) {
	root := project(t)
	before := snapshot(t, root)
	fb := &fakeBridge{artifact: passReport, cost: 0.12}
	var stdout, stderr bytes.Buffer

	code := New(depsWith(fb))(runArgs(root), strings.NewReader(diff), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr=%s)", code, stderr.String())
	}
	if stdout.String() != passReport {
		t.Errorf("stdout = %q, want the report exactly", stdout.String())
	}
	if len(fb.prompts) != 1 || !strings.Contains(fb.prompts[0], "## Agent Input") || !strings.Contains(fb.prompts[0], diff) {
		t.Fatalf("the dispatched prompt does not carry the diff as agent input: %v", fb.prompts)
	}
	req := fb.requests[0]
	if req.Cycle != 0 || req.Worktree != "" {
		t.Errorf("bridge request cycle=%d worktree=%q, want cycle 0 and no worktree", req.Cycle, req.Worktree)
	}
	if !strings.HasPrefix(req.Workspace, filepath.Join(root, ".evolve", "ephemeral", "agent-runs")+string(filepath.Separator)) {
		t.Errorf("workspace %q is not under the ephemeral agent-runs directory", req.Workspace)
	}
	for _, path := range changedPaths(before, snapshot(t, root)) {
		switch {
		case isLedgerFile(path):
		case path == filepath.Join(".evolve", "ephemeral")+"/", path == filepath.Join(".evolve", "ephemeral", "agent-runs")+"/":
		default:
			t.Errorf("the run changed %s; only the ledger may change", path)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "runs")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".evolve/runs exists after a standalone run (err=%v)", err)
	}
	if left, _ := os.ReadDir(filepath.Join(root, ".evolve", "ephemeral", "agent-runs")); len(left) != 0 {
		t.Errorf("the run workspace was not removed: %v", left)
	}
	entries := ledgerEntries(t, root)
	if len(entries) != 1 {
		t.Fatalf("ledger has %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.Cycle != 0 || e.Kind != "agent_run" || e.Role != "adversarial-review" || !strings.HasPrefix(e.CycleLabel, "agent-adversarial-review-20261021T093000Z-") {
		t.Errorf("ledger entry = %+v, want cycle 0, kind agent_run, role adversarial-review and a run label", e)
	}
	if e.RunID != e.CycleLabel || e.ArtifactSHA256 == "" || e.Source != "evolve agent run" {
		t.Errorf("ledger entry = %+v, want run_id = label, a report hash and the source", e)
	}
}

func TestAgentRun_JSONEnvelopeCarriesReportVerdictCostAndTokens(t *testing.T) {
	root := project(t)
	fb := &fakeBridge{artifact: passReport, cost: 0.12}
	var stdout, stderr bytes.Buffer

	code := New(depsWith(fb))(runArgs(root, "--json"), strings.NewReader(diff), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d (stderr=%s)", code, stderr.String())
	}
	var env Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not one JSON envelope: %v\n%s", err, stdout.String())
	}
	if env.Profile != "adversarial-review" || env.Verdict != core.VerdictPASS || env.Report != passReport {
		t.Errorf("envelope = %+v", env)
	}
	if env.CostUSD != 0.12 || env.Tokens.Input != 1200 || env.Tokens.Output != 300 {
		t.Errorf("envelope cost=%v tokens=%+v, want the bridge cost and tokens", env.CostUSD, env.Tokens)
	}
	if env.Model == "" || !strings.HasPrefix(env.RunLabel, "agent-adversarial-review-") || env.Workspace != "" || env.Error != "" {
		t.Errorf("envelope model=%q label=%q workspace=%q error=%q", env.Model, env.RunLabel, env.Workspace, env.Error)
	}
}

func TestAgentRun_KeepLeavesTheWorkspaceWithInputAndReport(t *testing.T) {
	root := project(t)
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{artifact: passReport}))(runArgs(root, "--json", "--keep"), strings.NewReader(diff), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d (stderr=%s)", code, stderr.String())
	}
	var env Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	input, err := os.ReadFile(filepath.Join(env.Workspace, "agent-input.md"))
	if err != nil || string(input) != diff {
		t.Errorf("kept agent-input.md = %q err=%v, want the diff", input, err)
	}
	if report, err := os.ReadFile(filepath.Join(env.Workspace, "adversarial-review-report.md")); err != nil || string(report) != passReport {
		t.Errorf("kept report = %q err=%v", report, err)
	}
}

func TestAgentRun_FailVerdictExitsOneAndStillPrintsTheReport(t *testing.T) {
	root := project(t)
	failing := "## Findings\nno threat model section\n"
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{artifact: failing}))(runArgs(root), strings.NewReader(diff), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit %d, want 1 for a FAIL verdict (stderr=%s)", code, stderr.String())
	}
	if stdout.String() != failing {
		t.Errorf("stdout = %q, want the report", stdout.String())
	}
}

func TestAgentRun_NoReportIsAFailVerdictWithAnEmptyStdout(t *testing.T) {
	root := project(t)
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{}))(runArgs(root), strings.NewReader(diff), &stdout, &stderr)

	if code != 1 || stdout.Len() != 0 {
		t.Errorf("exit %d stdout %q, want 1 and nothing on stdout", code, stdout.String())
	}
	if entries := ledgerEntries(t, root); len(entries) != 1 || entries[0].ArtifactSHA256 != "" {
		t.Errorf("ledger = %+v, want one entry without a report hash", entries)
	}
}

func TestAgentRun_DispatchErrorExitsTwentyOneAndNamesTheError(t *testing.T) {
	root := project(t)
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{err: errBridgeDown}))(runArgs(root, "--json"), strings.NewReader(diff), &stdout, &stderr)

	if code != exitDispatch {
		t.Fatalf("exit %d, want 21 (stderr=%s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "bridge down") {
		t.Errorf("stderr = %q, want the dispatch error", stderr.String())
	}
	var env Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil || !strings.Contains(env.Error, "bridge down") {
		t.Errorf("envelope error = %q err=%v, want the dispatch error", env.Error, err)
	}
}

func TestAgentRun_LedgerFailureIsADispatchError(t *testing.T) {
	root := project(t)
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "ledger.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{artifact: passReport}))(runArgs(root), strings.NewReader(diff), &stdout, &stderr)

	if code != exitDispatch || !strings.Contains(stderr.String(), "ledger") {
		t.Errorf("exit %d stderr %q, want 21 and a ledger error", code, stderr.String())
	}
	if stdout.String() != passReport {
		t.Errorf("stdout = %q, want the report even when the ledger fails", stdout.String())
	}
}

func TestAgentRun_WorkspaceCreateFailureIsADispatchError(t *testing.T) {
	root := project(t)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "ephemeral"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{artifact: passReport}))(runArgs(root), strings.NewReader(diff), &stdout, &stderr)

	if code != exitDispatch || !strings.Contains(stderr.String(), "workspace") {
		t.Errorf("exit %d stderr %q, want 21 and a workspace error", code, stderr.String())
	}
}

func TestAgentRun_LedgerFailureIsNamedInTheJSONEnvelope(t *testing.T) {
	root := project(t)
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "ledger.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := New(depsWith(&fakeBridge{artifact: passReport}))(runArgs(root, "--json"), strings.NewReader(diff), &stdout, &stderr)

	var env Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if code != exitDispatch || env.Error == "" || env.Verdict != core.VerdictPASS {
		t.Errorf("exit %d envelope error %q verdict %q, want 21, the ledger error and the PASS verdict", code, env.Error, env.Verdict)
	}
}

func TestAgentRun_AWorkspaceThatCannotBeRemovedIsNamedOnStderr(t *testing.T) {
	root := project(t)
	fb := &fakeBridge{artifact: passReport}
	var locked string
	fb.onLaunch = func(req core.BridgeRequest) {
		locked = req.Workspace
		if err := os.Chmod(locked, 0o555); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	var stdout, stderr bytes.Buffer

	code := New(depsWith(fb))(runArgs(root), strings.NewReader(diff), &stdout, &stderr)

	if code != 0 || !strings.Contains(stderr.String(), "remove the run workspace") {
		t.Errorf("exit %d stderr %q, want 0 and the removal error", code, stderr.String())
	}
}
