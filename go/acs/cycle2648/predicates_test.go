//go:build acs

// Package cycle2648 holds the predicates of the typed not-ready reason in evolve stage verify and evolve stage report.
package cycle2648

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	conflictKey      = "held_proposals.stage"
	plainKey         = "inbox.curator.stage"
	holdWithoutItem  = "gc.sentinel.hold.stage"
	reason           = "conflicts_unmeasured"
	heldPolicy       = `{"held_proposals":{"stage":"shadow"}}`
	exitReady        = 0
	exitNotReadyCode = 1
)

var (
	buildOnce sync.Once
	binary    string
	buildErr  string
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cycle2648-bin-")
		if err != nil {
			buildErr = err.Error()
			return
		}
		binary = filepath.Join(dir, "evolve")
		_, stderr, code, runErr := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "build", "-o", binary, "./cmd/evolve")
		if code != 0 {
			buildErr = "go build exited " + stderr
			if runErr != nil {
				buildErr += " " + runErr.Error()
			}
		}
	})
	if buildErr != "" {
		t.Fatalf("RED: %s", buildErr)
	}
	return binary
}

func heldProject(t *testing.T) string {
	t.Helper()
	root := acsassert.SetupTempProject(t)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(heldPolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(acsassert.GoModuleDir(t), filepath.Join(root, "go")); err != nil {
		t.Fatal(err)
	}
	return root
}

func runStage(t *testing.T, args ...string) (string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(evolveBinary(t), append([]string{"stage"}, args...)...)
	if code == -1 {
		t.Fatalf("evolve stage %v did not launch: %s", args, stderr)
	}
	return stdout, code
}

func decisionVersion(t *testing.T, root string) string {
	t.Helper()
	out, _ := runStage(t, "verify", conflictKey, "--json", "--project-root", root)
	var res struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Version == "" {
		t.Fatalf("RED: evolve stage verify prints no decision version: %q (%v)", out, err)
	}
	return res.Version
}

func chainJSON(key, version string, fields map[string]string, routesMeasured bool) []byte {
	rec := stageevidence.Link("", stageevidence.Record{Key: key, Version: version, Kind: stageevidence.KindDecision, ID: 1, Seq: 1, Fields: fields})
	outcome := map[string]any{"id": 1, "shipped": true}
	if routesMeasured {
		outcome["routes_measured"] = true
	}
	raw, err := json.Marshal(map[string]any{"records": []stageevidence.Record{rec}, "outcomes": []map[string]any{outcome}})
	if err != nil {
		panic(err)
	}
	return raw
}

func chainFile(t *testing.T, raw []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chain.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func heldChain(t *testing.T, root string, routesMeasured bool) string {
	t.Helper()
	fields := map[string]string{"would_hold": "true", "item": "item-1"}
	return chainFile(t, chainJSON(conflictKey, decisionVersion(t, root), fields, routesMeasured))
}

func notReadyOf(t *testing.T, raw []byte) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("not a JSON object: %s (%v)", raw, err)
	}
	got, _ := m["not_ready"].(string)
	return got
}

func scoreKey(t *testing.T, key string, fields map[string]string) (stageevidence.Result, string) {
	t.Helper()
	var chain stageevidence.Chain
	if err := json.Unmarshal(chainJSON(key, "v1", fields, false), &chain); err != nil {
		t.Fatal(err)
	}
	res := stageevidence.Score(chain, "v1", gatecontract.Promotion{MinDecisions: 1}, stageevidence.ScorerFor(key))
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return res, notReadyOf(t, raw)
}

func TestC2648_001_VerifyPrintsTheReasonInJSONAndText(t *testing.T) {
	root := heldProject(t)
	chain := heldChain(t, root, false)

	out, code := runStage(t, "verify", conflictKey, "--json", "--project-root", root, "--chain", chain)
	if code != exitNotReadyCode {
		t.Errorf("RED: verify of an unmeasured conflict class exited %d, want %d", code, exitNotReadyCode)
	}
	if got := notReadyOf(t, []byte(out)); got != reason {
		t.Errorf("RED: verify --json not_ready = %q, want %q:\n%s", got, reason, out)
	}

	text, _ := runStage(t, "verify", conflictKey, "--project-root", root, "--chain", chain)
	if !strings.Contains(text, reason) {
		t.Errorf("RED: verify text does not name %q:\n%s", reason, text)
	}
}

func TestC2648_002_AMeasuredConflictClassIsReadyAndPrintsNoReason(t *testing.T) {
	root := heldProject(t)
	chain := heldChain(t, root, true)

	out, code := runStage(t, "verify", conflictKey, "--json", "--project-root", root, "--chain", chain)
	if code != exitReady {
		t.Errorf("RED: verify of a measured conflict class exited %d, want %d:\n%s", code, exitReady, out)
	}
	if got := notReadyOf(t, []byte(out)); got != "" {
		t.Errorf("not_ready = %q for a measured conflict class, want none", got)
	}
	text, _ := runStage(t, "verify", conflictKey, "--project-root", root, "--chain", chain)
	if strings.Contains(text, reason) {
		t.Errorf("verify text names %q for a measured conflict class:\n%s", reason, text)
	}
}

func TestC2648_003_ReportPrintsTheReasonOnlyForTheKeyThatNeedsIt(t *testing.T) {
	root := heldProject(t)
	chain := heldChain(t, root, false)

	out, _ := runStage(t, "report", "--json", "--project-root", root, "--chain", chain)
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) == 0 {
		t.Fatalf("RED: stage report --json printed %q: %v", out, err)
	}
	named := 0
	for _, row := range rows {
		got, _ := row["not_ready"].(string)
		switch {
		case row["key"] == conflictKey && got != reason:
			t.Errorf("RED: report --json row of %s has not_ready %q, want %q", conflictKey, got, reason)
		case row["key"] != conflictKey && got != "":
			t.Errorf("report --json row of %v has not_ready %q, want none", row["key"], got)
		}
		if got != "" {
			named++
		}
	}
	if named != 1 {
		t.Errorf("RED: %d report rows name a not-ready reason, want exactly 1", named)
	}

	text, _ := runStage(t, "report", "--project-root", root, "--chain", chain)
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		isHeld := strings.HasPrefix(line, conflictKey+"\t")
		if isHeld != strings.Contains(line, reason) {
			t.Errorf("RED: report text line must name %q exactly on the %s row (isHeld=%v): %s", reason, conflictKey, isHeld, line)
		}
	}
}

func TestC2648_004_TheReasonFollowsTheKeyNotTheRecordFields(t *testing.T) {
	_, plain := scoreKey(t, plainKey, map[string]string{"would_hold": "true", "item": "item-1"})
	if plain != "" {
		t.Errorf("RED: %s carries would_hold in a record and got not_ready %q; the key does not depend on the conflict class", plainKey, plain)
	}
	_, sentinel := scoreKey(t, holdWithoutItem, map[string]string{"would_hold": "true"})
	if sentinel != "" {
		t.Errorf("RED: %s has no item to route and got not_ready %q", holdWithoutItem, sentinel)
	}
	res, bare := scoreKey(t, conflictKey, map[string]string{"cycle": "1"})
	if bare != reason || res.Ready {
		t.Errorf("RED: %s record without would_hold: not_ready %q ready %v, want %q and not ready", conflictKey, bare, res.Ready, reason)
	}
}

func TestC2648_005_OneSharedPredicateDecidesEveryStageKey(t *testing.T) {
	if !stageevidence.DependsOnConflict(conflictKey) {
		t.Fatalf("RED: DependsOnConflict(%q) = false, want true", conflictKey)
	}
	for _, key := range []string{plainKey, holdWithoutItem, "not.a.stage.key"} {
		if stageevidence.DependsOnConflict(key) {
			t.Errorf("DependsOnConflict(%q) = true, want false", key)
		}
	}
	for _, entry := range gateregistry.StageEntries() {
		key := entry.Shadow.PolicyKey
		_, got := scoreKey(t, key, map[string]string{"cycle": "1"})
		if want := stageevidence.DependsOnConflict(key); (got == reason) != want {
			t.Errorf("RED: %s scores not_ready %q while DependsOnConflict is %v: the scorer must use the shared predicate", key, got, want)
		}
	}
}

func TestC2648_006_AMeasuredChainStillCountsTheRoutedConflict(t *testing.T) {
	fields := map[string]string{"would_hold": "true", "item": "item-1"}
	var chain stageevidence.Chain
	if err := json.Unmarshal(chainJSON(conflictKey, "v1", fields, true), &chain); err != nil {
		t.Fatal(err)
	}
	chain.Outcomes[0].Routed = []string{"item-1"}
	res := stageevidence.Score(chain, "v1", gatecontract.Promotion{MinDecisions: 1}, stageevidence.ScorerFor(conflictKey))
	out, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if res.Conflicts != 1 || res.Ready || notReadyOf(t, out) != "" {
		t.Errorf("RED: measured routed item: conflicts %d ready %v not_ready %q, want 1 conflict, not ready, no reason", res.Conflicts, res.Ready, notReadyOf(t, out))
	}
}
