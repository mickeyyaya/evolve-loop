//go:build acs

package cycle1498

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const consumedFleetAlias = "pipeline-defect-pipeline-blocker"

const liveSibling = "todo-live-unrelated-sibling"

const namedRegressionTest = "TestCarryoverApplyDecisions_DropsConsumedFleetAlias"

var evolveBin string

var buildErr string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1498-bin-")
	if err != nil {
		buildErr = fmt.Sprintf("mktemp for evolve build: %v", err)
		m.Run()
		return
	}
	defer os.RemoveAll(dir)

	goMod, err := moduleRoot()
	if err != nil {
		buildErr = err.Error()
		m.Run()
		return
	}
	bin := filepath.Join(dir, "evolve-under-test")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = goMod
	if out, err := cmd.CombinedOutput(); err != nil {
		buildErr = fmt.Sprintf("go build ./cmd/evolve (dir=%s): %v\n%s", goMod, err, out)
	} else {
		evolveBin = bin
	}
	m.Run()
}

func moduleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no go.mod found walking up from %s", wd)
}

func requireBinary(t *testing.T) string {
	t.Helper()
	if buildErr != "" {
		t.Fatalf("cannot exercise the CLI under test: %s", buildErr)
	}
	if evolveBin == "" {
		t.Fatalf("evolve binary was not built (empty path, no recorded error)")
	}
	return evolveBin
}

func writeAliasFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	state := map[string]any{
		"stateRevision": float64(1),
		"carryoverTodos": []any{
			map[string]any{"id": consumedFleetAlias, "action": "consumed fleet alias; refuted premise", "priority": "low"},
			map[string]any{
				"id":       liveSibling,
				"action":   "still live: reconcile the " + consumedFleetAlias + " premise against the inbox",
				"priority": "high",
			},
		},
		"someOtherKey": "preserved",
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture state: %v", err)
	}
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write fixture state: %v", err)
	}
	return path
}

func writeDecisions(t *testing.T, aliasReason string) string {
	t.Helper()
	doc := map[string]any{
		"source_count": 2,
		"decisions": []any{
			map[string]any{"id": consumedFleetAlias, "decision": "drop", "reason": aliasReason},
			map[string]any{"id": liveSibling, "decision": "keep", "reason": "still live operator work"},
		},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal decisions: %v", err)
	}
	path := filepath.Join(t.TempDir(), "decisions.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write decisions: %v", err)
	}
	return path
}

func carryoverIDs(t *testing.T, statePath string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state %s: %v", statePath, err)
	}
	var doc struct {
		CarryoverTodos []struct {
			ID string `json:"id"`
		} `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("state.json is not valid JSON after the apply: %v\n%s", err, raw)
	}
	ids := make(map[string]bool, len(doc.CarryoverTodos))
	for _, e := range doc.CarryoverTodos {
		ids[e.ID] = true
	}
	return ids
}

func applyRetirement(t *testing.T, statePath, decisionsPath string) (stdout, stderr string, code int) {
	t.Helper()
	bin := requireBinary(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(bin,
		"carryover", "apply-decisions", "--apply", "--state", statePath, "--decisions", decisionsPath)
	if err != nil && code == 0 {
		t.Fatalf("running %s carryover apply-decisions: %v (stderr=%s)", bin, err, stderr)
	}
	return stdout, stderr, code
}

func TestC1498_001_AliasDropRemovesOnlyTheNamedAlias(t *testing.T) {
	statePath := writeAliasFixture(t)
	decisionsPath := writeDecisions(t, "consumed fleet alias; residual shipped")

	stdout, stderr, code := applyRetirement(t, statePath, decisionsPath)
	if code != 0 {
		t.Fatalf("apply exited %d, want 0 (stdout=%s stderr=%s)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "2→1") && !strings.Contains(stdout, "2->1") {
		t.Errorf("apply did not report a 2→1 convergence; stdout=%q", stdout)
	}

	ids := carryoverIDs(t, statePath)
	if ids[consumedFleetAlias] {
		t.Errorf("consumed alias %q survived a reviewed drop decision", consumedFleetAlias)
	}
	if !ids[liveSibling] {
		t.Errorf("unrelated live item %q was removed — retirement over-reached beyond the named id", liveSibling)
	}
	if len(ids) != 1 {
		t.Errorf("resident carryover ids = %d (%v), want exactly 1 (the live sibling)", len(ids), ids)
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if !strings.Contains(string(raw), `"someOtherKey"`) {
		t.Errorf("unrelated state key someOtherKey was dropped by the apply")
	}
}

func TestC1498_002_EmptyReasonDropIsRejectedBeforeAnyWrite(t *testing.T) {
	statePath := writeAliasFixture(t)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	decisionsPath := writeDecisions(t, "   ")

	stdout, stderr, code := applyRetirement(t, statePath, decisionsPath)
	if code == 0 {
		t.Fatalf("apply exited 0 for an empty-reason drop; want non-zero (stdout=%s)", stdout)
	}
	if !strings.Contains(stderr, "empty reason") {
		t.Errorf("rejection did not name the empty-reason cause; stderr=%q", stderr)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after rejection: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("state.json was mutated by a REJECTED decisions file (want byte-identical)")
	}
	ids := carryoverIDs(t, statePath)
	if !ids[consumedFleetAlias] || !ids[liveSibling] {
		t.Errorf("carryover entries changed on a rejected apply: %v", ids)
	}
}

func TestC1498_003_RepeatedAliasDropIsIdempotent(t *testing.T) {
	statePath := writeAliasFixture(t)
	decisionsPath := writeDecisions(t, "consumed fleet alias; residual shipped")

	if _, stderr, code := applyRetirement(t, statePath, decisionsPath); code != 0 {
		t.Fatalf("first apply exited %d, want 0 (stderr=%s)", code, stderr)
	}
	first := carryoverIDs(t, statePath)

	stdout, stderr, code := applyRetirement(t, statePath, decisionsPath)
	if code != 0 {
		t.Fatalf("second (idempotent) apply exited %d, want 0 (stderr=%s)", code, stderr)
	}
	second := carryoverIDs(t, statePath)

	if len(first) != len(second) {
		t.Errorf("repeat apply changed the carryover count %d→%d (not idempotent)", len(first), len(second))
	}
	if second[consumedFleetAlias] {
		t.Errorf("consumed alias reappeared after the repeat apply")
	}
	if !second[liveSibling] {
		t.Errorf("live sibling %q was removed by the REPEAT apply", liveSibling)
	}
	if !strings.Contains(stdout, "dropped 0") {
		t.Errorf("repeat apply reported a non-zero drop count; stdout=%q", stdout)
	}
}

func TestC1498_004_NamedRegressionTestExistsAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	testFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_carryover_test.go")
	if !acsassert.FileExists(t, testFile) {
		t.Fatalf("RED: %s missing", testFile)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch",
		"go/cmd/evolve/cmd_carryover_test.go"); code != 0 {
		t.Errorf("RED: go/cmd/evolve/cmd_carryover_test.go is untracked — it would be dropped at ship")
	}

	cmd := exec.Command("go", "test", "./cmd/evolve", "-run", "^"+namedRegressionTest+"$", "-v", "-count=1")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	text := string(out)

	if !strings.Contains(text, "--- PASS: "+namedRegressionTest) {
		t.Fatalf("RED: %s did not run and PASS in ./cmd/evolve (go test err=%v)\n%s", namedRegressionTest, err, text)
	}
	if strings.Contains(text, "no tests to run") {
		t.Errorf("RED: `go test -run ^%s$` matched no test (vacuous green)", namedRegressionTest)
	}
}
