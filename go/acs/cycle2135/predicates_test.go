//go:build acs

package cycle2135

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	profile    = "adversarial-review"
	passReport = "## Threat Model\nA diff that adds one function.\n## Findings\nNone.\n## Verdict\nPASS\n"
	diff       = "diff --git a/go/x.go b/go/x.go\n+func X() int { return 1 }\n"
)

var fixtureFiles = []string{
	"docs/architecture/phase-registry.json",
	".evolve/phases/adversarial-review/phase.json",
	".evolve/profiles/adversarial-review.json",
	"agents/evolve-adversarial-review.md",
}

type rowWritingBridge struct {
	rows []llmcalls.Record
	err  error
}

func (b *rowWritingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if b.err != nil {
		return core.BridgeResponse{ExitCode: 1}, b.err
	}
	if err := os.WriteFile(req.ArtifactPath, []byte(passReport), 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	for _, rec := range b.rows {
		if err := llmcalls.AppendWorkspace(req.Workspace, rec); err != nil {
			return core.BridgeResponse{}, err
		}
	}
	return core.BridgeResponse{Tokens: core.TokenUsage{Input: 1200, Output: 300}}, nil
}

func (b *rowWritingBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func newProject(t *testing.T) string {
	t.Helper()
	repo := acsassert.RepoRoot(t)
	root := t.TempDir()
	for _, rel := range fixtureFiles {
		raw, err := os.ReadFile(filepath.Join(repo, rel))
		if err != nil {
			t.Fatalf("read fixture %s: %v", rel, err)
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func agentRun(t *testing.T, root string, b core.Bridge, extra ...string) (envelope agentcmd.Envelope, stderr string, code int) {
	t.Helper()
	deps := agentcmd.Deps{
		Bridge:    func(string, *signalcenter.Center) core.Bridge { return b },
		Signals:   func(string, io.Writer) (*signalcenter.Center, func()) { return signalcenter.New(), func() {} },
		TreeState: func(context.Context, string) (string, error) { return "tree", nil },
		Getenv:    func(string) string { return "" },
		Now:       func() time.Time { return time.Date(2026, 11, 3, 9, 30, 0, 0, time.UTC) },
	}
	args := append([]string{"run", profile, "--project-root", root, "--json"}, extra...)
	var out, errBuf bytes.Buffer
	code = agentcmd.New(deps)(args, strings.NewReader(diff), &out, &errBuf)
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON envelope (exit %d): %v\n%s\nstderr=%s", code, err, out.String(), errBuf.String())
	}
	return envelope, errBuf.String(), code
}

func attempt(callID string) llmcalls.Record {
	return llmcalls.Record{
		SchemaVersion: llmcalls.SchemaVersion, CallID: callID, TS: "2026-11-03T09:30:00Z",
		Agent: profile, Phase: profile, CLI: "claude", Model: "claude-sonnet-5-5", Attempt: 1,
		Tokens: core.TokenUsage{Input: 1200, Output: 300}, Source: llmcalls.SourceUnknown,
	}
}

func durableLedger(root string) string {
	return filepath.Join(root, ".evolve", "agent-runs", llmcalls.Filename)
}

func readDurable(t *testing.T, root string) []llmcalls.Record {
	t.Helper()
	res, err := llmcalls.Read(durableLedger(root))
	if err != nil {
		t.Fatalf("RED: durable ledger %s unreadable: %v", durableLedger(root), err)
	}
	return res.Records
}

func TestC2135_001_RunWithoutKeepLeavesTaggedRowsInTheDurableLedger(t *testing.T) {
	root := newProject(t)
	bridge := &rowWritingBridge{rows: []llmcalls.Record{attempt("call-a"), attempt("call-b")}}

	env, stderr, code := agentRun(t, root, bridge)

	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr=%s)", code, stderr)
	}
	if env.Workspace != "" {
		t.Fatalf("workspace %q reported; the run must be a run without --keep", env.Workspace)
	}
	if left, _ := os.ReadDir(filepath.Join(root, ".evolve", "ephemeral", "agent-runs")); len(left) != 0 {
		t.Fatalf("the run workspace was not removed: %v", left)
	}
	rows := readDurable(t, root)
	if len(rows) != 2 {
		t.Fatalf("RED: durable ledger has %d rows, want 2", len(rows))
	}
	for _, row := range rows {
		if row.RunID != env.RunLabel {
			t.Errorf("row %s run_id = %q, want the run label %q", row.CallID, row.RunID, env.RunLabel)
		}
		if row.Profile != profile {
			t.Errorf("row %s profile = %q, want %q", row.CallID, row.Profile, profile)
		}
	}
	if rows[0].CallID == rows[1].CallID {
		t.Errorf("both rows carry call_id %q; the import must keep both distinct attempts", rows[0].CallID)
	}
}

func TestC2135_002_RunWithoutAnyRowsLeavesNoLedgerAndNoWarning(t *testing.T) {
	root := newProject(t)

	_, stderr, code := agentRun(t, root, &rowWritingBridge{})

	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr=%s)", code, stderr)
	}
	if strings.Contains(stderr, llmcalls.Filename) || strings.Contains(strings.ToLower(stderr), "model-attempt") {
		t.Errorf("a run with no model-attempt rows warned about the ledger: %s", stderr)
	}
	if _, err := os.Stat(durableLedger(root)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a run with no rows created the durable ledger (err=%v)", err)
	}
}

func TestC2135_003_SecondImportOfTheSameCallAddsNoRow(t *testing.T) {
	root := newProject(t)
	same := []llmcalls.Record{attempt("call-shared")}

	first, _, code := agentRun(t, root, &rowWritingBridge{rows: same})
	if code != 0 {
		t.Fatalf("first run exit %d, want 0", code)
	}
	if before := readDurable(t, root); len(before) != 1 {
		t.Fatalf("RED: durable ledger has %d rows after the first run, want 1", len(before))
	}
	second, _, code := agentRun(t, root, &rowWritingBridge{rows: same})
	if code != 0 {
		t.Fatalf("second run exit %d, want 0", code)
	}
	if first.RunLabel == second.RunLabel {
		t.Fatalf("both runs share the label %q; the test needs two imports", first.RunLabel)
	}

	rows := readDurable(t, root)
	if len(rows) != 1 {
		t.Fatalf("durable ledger has %d rows after a second import of call-shared, want 1", len(rows))
	}
	if rows[0].RunID != first.RunLabel {
		t.Errorf("the surviving row run_id = %q, want the first run label %q", rows[0].RunID, first.RunLabel)
	}
}

func TestC2135_004_ImportFailureWarnsAndKeepsTheExitCodeOfAPassingRun(t *testing.T) {
	root := newProject(t)
	if err := os.MkdirAll(durableLedger(root), 0o755); err != nil {
		t.Fatal(err)
	}
	bridge := &rowWritingBridge{rows: []llmcalls.Record{attempt("call-blocked")}}

	_, stderr, code := agentRun(t, root, bridge)

	if code != 0 {
		t.Errorf("exit %d, want 0: an import failure must not change the exit code (stderr=%s)", code, stderr)
	}
	if !strings.Contains(stderr, llmcalls.Filename) {
		t.Errorf("RED: stderr carries no warning that names %s: %q", llmcalls.Filename, stderr)
	}
}

func TestC2135_005_ImportFailureKeepsTheDispatchExitCodeOfAFailingRun(t *testing.T) {
	baselineRoot := newProject(t)
	_, _, baseline := agentRun(t, baselineRoot, &rowWritingBridge{err: errors.New("bridge down")})
	if baseline == 0 {
		t.Fatal("a failing bridge must give a non-zero exit code")
	}

	root := newProject(t)
	if err := os.MkdirAll(durableLedger(root), 0o755); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := agentRun(t, root, &rowWritingBridge{err: errors.New("bridge down")})

	if code != baseline {
		t.Errorf("exit %d with a blocked ledger, want %d as without the block (stderr=%s)", code, baseline, stderr)
	}
}
