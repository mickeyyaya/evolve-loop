//go:build acs

package cycle1932

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const reweightedItemID = "recurring-reweight-item"

type boundaryProject struct {
	evolveDir string
	inboxDir  string
	itemPath  string
}

func newBoundaryProject(t *testing.T, itemWeight, intentWeight float64, recurrenceCount int) boundaryProject {
	t.Helper()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	inboxDir := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ledger.New(evolveDir).Append(context.Background(), core.LedgerEntry{
		TS: "2026-10-17T00:00:00Z", Role: "orchestrator", Kind: "cycle-start",
	}); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
	body, err := json.MarshalIndent(map[string]any{
		"id":             reweightedItemID,
		"title":          "a recurring failure the boundary reweights",
		"kind":           "bug",
		"summary":        "the same failure pattern recurred across cycles",
		"fix":            "remove the recurring failure",
		"priority_class": "correctness",
		"weight":         itemWeight,
		"acceptance":     []string{"the failure stops recurring"},
		"created_at":     "2026-10-01T00:00:00Z",
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	itemPath := filepath.Join(inboxDir, reweightedItemID+".json")
	if err := os.WriteFile(itemPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := dispositionrouter.StageIntent(filepath.Join(evolveDir, "escalations"), dispositionrouter.Intent{
		Cycle:         1931,
		Pattern:       "pattern:recurring-reweight",
		ItemID:        reweightedItemID,
		Action:        dispositionrouter.ActionEscalate,
		Route:         dispositionrouter.RouteQueue,
		Recurrence:    recurrenceCount,
		Weight:        intentWeight,
		PriorityClass: "correctness",
	}); err != nil {
		t.Fatalf("StageIntent: %v", err)
	}
	return boundaryProject{evolveDir: evolveDir, inboxDir: inboxDir, itemPath: itemPath}
}

func (p boundaryProject) options(pol recurrence.EscalationPolicy, reweight func(id string, weight float64) error) recurrence.ApplyOptions {
	return recurrence.ApplyOptions{
		InboxDir:        p.inboxDir,
		EscalationsPath: dispositionrouter.PendingActionsPath(filepath.Join(p.evolveDir, "escalations")),
		ReportPath:      filepath.Join(p.evolveDir, "escalation-apply-report.json"),
		Cycle:           1932,
		Policy:          pol,
		Now:             time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC),
		Reweight:        reweight,
	}
}

func (p boundaryProject) moverReweight(id string, weight float64) error {
	_, err := inboxmover.Edit(inboxmover.Options{
		ProjectRoot: filepath.Dir(p.evolveDir),
		InboxDir:    p.inboxDir,
		LedgerPath:  filepath.Join(p.evolveDir, "ledger.jsonl"),
	}, id, []inboxmover.FieldEdit{{Op: inboxmover.EditSet, Field: "weight", Value: strconv.FormatFloat(weight, 'f', -1, 64)}})
	return err
}

func (p boundaryProject) apply(t *testing.T, pol recurrence.EscalationPolicy) (recurrence.ApplyResult, error) {
	t.Helper()
	return recurrence.ApplyBoundary(p.options(pol, p.moverReweight))
}

func (p boundaryProject) itemBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(p.itemPath)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	return raw
}

func (p boundaryProject) itemWeight(t *testing.T) float64 {
	t.Helper()
	raw, err := os.ReadFile(p.itemPath)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	var item struct {
		Weight float64 `json:"weight"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("parse item: %v", err)
	}
	return item.Weight
}

func (p boundaryProject) lifecycleEdits(t *testing.T) []core.LedgerEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(p.evolveDir, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var edits []core.LedgerEntry
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var e core.LedgerEntry
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("ledger line does not parse: %v: %s", err, line)
		}
		if e.Kind == "inbox-lifecycle" && e.Action == "edit" && e.TaskID == reweightedItemID {
			edits = append(edits, e)
		}
	}
	return edits
}

func TestC1932_001_RecurrenceReweightAppendsAnInboxLifecycleEditLine(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	p := newBoundaryProject(t, 0.5, 0.5, 4)
	pol := recurrence.DefaultEscalationPolicy()
	res, err := p.apply(t, pol)
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if !slices.Equal(res.Bumped, []string{reweightedItemID}) {
		t.Fatalf("Bumped = %v, want [%s]", res.Bumped, reweightedItemID)
	}
	if got, want := p.itemWeight(t), pol.Target(0.5, 4); got != want {
		t.Errorf("item weight = %v, want the policy target %v", got, want)
	}
	edits := p.lifecycleEdits(t)
	if len(edits) != 1 {
		t.Fatalf("RED: %d inbox-lifecycle edit lines for %s in <project>/.evolve/ledger.jsonl, want 1: the reweight bypasses inboxmover", len(edits), reweightedItemID)
	}
	if !strings.Contains(edits[0].Message, "weight") {
		t.Errorf("edit line message %q does not name the weight change", edits[0].Message)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".evolve")); err == nil {
		t.Errorf("the reweight wrote a .evolve tree under the process cwd %s: the mover's ledger path must derive from the inbox's project, not the cwd", cwd)
	}
}

func TestC1932_002_RecurrenceReweightRefusesAWeightTheCuratorRuleRejects(t *testing.T) {
	p := newBoundaryProject(t, 0.5, 0.5, 4)
	capAboveOne := recurrence.EscalationPolicy{Threshold: 2, Step: 0.5, Cap: 1.5}
	res, _ := p.apply(t, capAboveOne)
	if slices.Contains(res.Bumped, reweightedItemID) {
		t.Errorf("RED: the boundary reported %s bumped to a weight above one: Bumped = %v", reweightedItemID, res.Bumped)
	}
	if got := p.itemWeight(t); got != 0.5 {
		t.Errorf("RED: item weight = %v after an out-of-range target, want it left at 0.5 (weight must be in (0, 1])", got)
	}
	if edits := p.lifecycleEdits(t); len(edits) != 0 {
		t.Errorf("a refused reweight recorded %d edit lines, want 0", len(edits))
	}
}

func TestC1932_003_RecurrenceReweightNeverLowersAndLeavesTheItemUntouched(t *testing.T) {
	p := newBoundaryProject(t, 0.9, 0.5, 2)
	before, err := os.ReadFile(p.itemPath)
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.apply(t, recurrence.DefaultEscalationPolicy())
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if !slices.Contains(res.Skipped, reweightedItemID) || len(res.Bumped) != 0 {
		t.Errorf("result = %+v, want %s skipped and nothing bumped", res, reweightedItemID)
	}
	after, err := os.ReadFile(p.itemPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("a skipped reweight rewrote the item:\nbefore %s\nafter  %s", before, after)
	}
	if edits := p.lifecycleEdits(t); len(edits) != 0 {
		t.Errorf("a skipped reweight recorded %d edit lines, want 0", len(edits))
	}
}

func TestC1932_004_InboxWriteScanEnrollsNoRecurrenceWriter(t *testing.T) {
	root := acsassert.RepoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1",
		"-run", "TestInboxWriters_EveryAutofiledItemCarriesAKnownClass/every_production_write_into_the_inbox_is_enrolled",
		"./cmd/evolve")
	cmd.Dir = filepath.Join(root, "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the inbox write-site scan fails: %v\n%s", err, out)
	}
	contract := filepath.Join(root, "go", "cmd", "evolve", "inbox_writers_contract_test.go")
	if !acsassert.FileNotContains(t, contract, `"internal/recurrence`) {
		t.Errorf("RED: inboxWritersThatFileNothing still enrolls a recurrence writer: every inbox item write must go through internal/inboxmover")
	}
}

type reweightCall struct {
	id     string
	weight float64
}

func TestC1932_005_RecurrenceDelegatesTheItemWriteToTheInjectedReweightSeam(t *testing.T) {
	p := newBoundaryProject(t, 0.5, 0.5, 4)
	before := p.itemBytes(t)
	pol := recurrence.DefaultEscalationPolicy()
	var calls []reweightCall
	recordingSeam := func(id string, weight float64) error {
		calls = append(calls, reweightCall{id: id, weight: weight})
		return nil
	}
	res, err := recurrence.ApplyBoundary(p.options(pol, recordingSeam))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if want := []reweightCall{{id: reweightedItemID, weight: pol.Target(0.5, 4)}}; !slices.Equal(calls, want) {
		t.Fatalf("RED: Reweight seam calls = %+v, want %+v: the reweight must go through the injected writer", calls, want)
	}
	if !slices.Equal(res.Bumped, []string{reweightedItemID}) {
		t.Errorf("Bumped = %v, want [%s] once the seam accepted the weight", res.Bumped, reweightedItemID)
	}
	if after := p.itemBytes(t); !bytes.Equal(before, after) {
		t.Errorf("RED: internal/recurrence rewrote the item itself beside the seam:\nbefore %s\nafter  %s", before, after)
	}
	if edits := p.lifecycleEdits(t); len(edits) != 0 {
		t.Errorf("RED: internal/recurrence appended %d inbox-lifecycle lines itself; the ledger line is the mover's", len(edits))
	}
}

func TestC1932_006_ARefusedSeamWriteIsNotReportedBumped(t *testing.T) {
	p := newBoundaryProject(t, 0.5, 0.5, 4)
	curatorRefusal := errors.New("curator refused the weight edit")
	refusingSeam := func(string, float64) error { return curatorRefusal }
	res, err := recurrence.ApplyBoundary(p.options(recurrence.DefaultEscalationPolicy(), refusingSeam))
	if slices.Contains(res.Bumped, reweightedItemID) {
		t.Errorf("RED: a refused seam write still reported %s bumped: %+v", reweightedItemID, res)
	}
	if err == nil && !slices.Contains(res.Refused, reweightedItemID) && !slices.Contains(res.Skipped, reweightedItemID) {
		t.Errorf("RED: the seam refusal vanished: no error and %s neither refused nor skipped: %+v", reweightedItemID, res)
	}
	if err != nil && !errors.Is(err, curatorRefusal) {
		t.Errorf("ApplyBoundary error %v does not wrap the seam's refusal", err)
	}
}

func TestC1932_007_WithoutAReweightSeamRecurrenceWritesNoItem(t *testing.T) {
	p := newBoundaryProject(t, 0.5, 0.5, 4)
	before := p.itemBytes(t)
	res, _ := recurrence.ApplyBoundary(p.options(recurrence.DefaultEscalationPolicy(), nil))
	if slices.Contains(res.Bumped, reweightedItemID) {
		t.Errorf("RED: with no Reweight seam the boundary still bumped %s: internal/recurrence keeps its own item writer", reweightedItemID)
	}
	if after := p.itemBytes(t); !bytes.Equal(before, after) {
		t.Errorf("RED: with no Reweight seam internal/recurrence rewrote the item itself:\nbefore %s\nafter  %s", before, after)
	}
	if edits := p.lifecycleEdits(t); len(edits) != 0 {
		t.Errorf("RED: with no Reweight seam internal/recurrence appended %d inbox-lifecycle lines itself", len(edits))
	}
}

func TestC1932_008_TheLoopBoundaryCallerReweightsThroughInboxmover(t *testing.T) {
	root := acsassert.RepoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1",
		"-run", "^TestApplyEscalationBoundary_ReweightsAnOpenItemThroughInboxmover$", "-v",
		"./cmd/evolve")
	cmd.Dir = filepath.Join(root, "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("RED: the production caller applyEscalationBoundary does not reweight through inboxmover: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "--- PASS: TestApplyEscalationBoundary_ReweightsAnOpenItemThroughInboxmover") {
		t.Errorf("the caller-proof test did not run:\n%s", out)
	}
}
