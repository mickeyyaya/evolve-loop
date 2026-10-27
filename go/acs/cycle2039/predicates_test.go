//go:build acs

package cycle2039

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/fixability"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	consoleItemAC5     = "retro-minting-files-through-inbox-add-lane-ac-5-console"
	consoleItemUnify   = "retro-minting-files-through-inbox-add-console"
	salvageBranch      = "fix/c2035-retro-minting"
	failurelearningPkg = "go/internal/core/failurelearning"
)

const cycle2035Round1 = `<!-- evolve-verdict: {"phase":"audit","verdict":"WARN","schema_version":2,"failure":{"class":"code-audit-warn","defects":["M1: go/internal/core/failurelearning floor tests fail (3) with this diff, because faillearn now mints timestamped files under .bump.lock; protected surface, fix is console item retro-minting-files-through-inbox-add-lane-ac-5-console","M2: go/internal/inboxfile/inboxfile.go duplicates the filing rules of go/internal/inboxmover/lifecycle/file.go (protected); unification queued as retro-minting-files-through-inbox-add-console"],"evidence_paths":[".evolve/runs/cycle-2035/acs-verdict.json",".evolve/runs/cycle-2035/audit-report.md"],"prescription":["Land console item retro-minting-files-through-inbox-add-lane-ac-5-console (core/failurelearning floor_test + goldens) in the same PR, or CI goes red on 3 TestWriteFloor_* tests"]}} -->`

const cycle2035Round2 = `<!-- evolve-verdict: {"phase":"audit","verdict":"WARN","schema_version":2,"failure":{"class":"code-audit-warn","defects":["M1: go/internal/core/failurelearning floor tests fail (3) with this diff, because faillearn now mints timestamped files under .bump.lock; protected surface, fix is console item retro-minting-files-through-inbox-add-lane-ac-5-console","M2: go/internal/inboxfile/inboxfile.go duplicates the filing rules of go/internal/inboxmover/lifecycle/file.go (protected); unification queued as retro-minting-files-through-inbox-add-console","M3: go/cmd/evolve/cmd_wave_pid_integration_test.go:19-26 adds an out-of-scope sandbox skip in a document-only correction round; the root cause is the phase sandbox denying /bin/ps"],"evidence_paths":[".evolve/runs/cycle-2035/acs-verdict.json",".evolve/runs/cycle-2035/audit-report.md"],"prescription":["Land console item retro-minting-files-through-inbox-add-lane-ac-5-console (core/failurelearning floor_test + goldens) in the same PR, or CI goes red on 3 TestWriteFloor_* tests","Drop the ps skip from go/cmd/evolve/cmd_wave_pid_integration_test.go and allow /bin/ps in the phase sandbox profile instead"]}} -->`

const cycle2035Round3 = `<!-- evolve-verdict: {"phase":"audit","verdict":"FAIL","schema_version":2,"failure":{"class":"code-audit-fail","defects":["H1: go/internal/core/retro_remediation_filter_test.go:29 inboxFiles counts .bump.lock and every entry, so TestWriteDeterministicLearning_StructuredDefectsAreFiled and TestRepro1285_F1_CollidingRemediationSlugSuppressesRetrospectiveAndLesson fail with this diff; unprotected, lane-fixable; ship REPO_CONTRACT_GATE refused","M1: go/internal/core/failurelearning floor tests fail (3) with this diff; protected surface, fix is console item retro-minting-files-through-inbox-add-lane-ac-5-console in the same PR","M2: go/internal/inboxfile/inboxfile.go duplicates the filing rules of go/internal/inboxmover/lifecycle/file.go (protected); unification queued as retro-minting-files-through-inbox-add-console","M3: go/cmd/evolve/cmd_wave_pid_integration_test.go:19-26 adds an out-of-scope ps sandbox skip"],"evidence_paths":[".evolve/runs/cycle-2035/ship-error.json",".evolve/runs/cycle-2035/audit-report.md"],"prescription":["Make inboxFiles in go/internal/core/retro_remediation_filter_test.go count only *.json items (ignore inboxfile.LockName) and rerun go test -count=1 ./internal/core","Land console item retro-minting-files-through-inbox-add-lane-ac-5-console (core/failurelearning floor_test + goldens) in the same PR, or the importer backstop stays red on 3 TestWriteFloor_* tests","Drop the ps skip from go/cmd/evolve/cmd_wave_pid_integration_test.go and allow /bin/ps in the phase sandbox profile instead"]}} -->`

const cycle2035Round4 = `<!-- evolve-verdict: {"phase":"audit","verdict":"WARN","schema_version":2,"failure":{"class":"code-audit-warn","defects":["M1: go/internal/core/failurelearning floor tests (3) fail with this diff; protected surface, fix is console item retro-minting-files-through-inbox-add-lane-ac-5-console in the same PR","M2: go/internal/inboxfile/inboxfile.go duplicates the filing rules of protected go/internal/inboxmover/lifecycle/file.go; unification queued as retro-minting-files-through-inbox-add-console"],"evidence_paths":[".evolve/runs/cycle-2035/ship-error.json",".evolve/runs/cycle-2035/acs-verdict.json"],"prescription":["Land console item retro-minting-files-through-inbox-add-lane-ac-5-console (core/failurelearning floor_test + goldens) in the same PR, or the importer backstop stays red on 3 TestWriteFloor_* tests"]}} -->`

const helperFixDefect = "H1: go/internal/core/retro_remediation_filter_test.go:29 inboxFiles counts .bump.lock and every entry, so TestWriteDeterministicLearning_StructuredDefectsAreFiled fails with this diff; unprotected, lane-fixable"

const duplicateRulesDefect = "M2: go/internal/inboxfile/inboxfile.go duplicates the filing rules of go/internal/inboxmover/lifecycle/file.go (protected); unification queued as " + consoleItemUnify

const widgetDefect = "M7: go/internal/widget/widget.go fails its test; the fix is console item widget-fix-console"

const protectedHighDefect = "H2: go/internal/guards/integrity_surface.go lists a fence path that the diff removes"

var bothConsoleItemsOpen = []string{consoleItemAC5, consoleItemUnify}

func defectsOf(t *testing.T, sentinel string) []string {
	t.Helper()
	parsed, ok := phasecontract.ParseVerdictSentinelFull(sentinel)
	if !ok || parsed.Failure == nil {
		t.Fatalf("the cycle 2035 fixture does not parse as a failure block: %.80s", sentinel)
	}
	return parsed.Failure.Defects
}

func idsOf(findings []fixability.Finding) []string {
	out := []string{}
	for _, f := range findings {
		out = append(out, f.ID)
	}
	return out
}

func requireIDs(t *testing.T, label string, findings []fixability.Finding, want ...string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if got := idsOf(findings); !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestC2039_001_RoundOneStopsWithHandoffBlockerNamingM1AndTheConsoleItem(t *testing.T) {
	decision := fixability.Classify(fixability.Input{
		Defects:          defectsOf(t, cycle2035Round1),
		RedPaths:         []string{failurelearningPkg},
		OpenConsoleItems: bothConsoleItemsOpen,
	})
	if !decision.Stop {
		t.Fatalf("round 1 of cycle 2035 did not stop: %+v", decision)
	}
	if decision.Reason != "handoff_blocker" || fixability.ReasonHandoffBlocker != "handoff_blocker" {
		t.Errorf("reason = %q (constant %q), want handoff_blocker", decision.Reason, fixability.ReasonHandoffBlocker)
	}
	requireIDs(t, "blockers", decision.Blockers, "M1")
	if !reflect.DeepEqual(decision.ConsoleItems, []string{consoleItemAC5}) {
		t.Errorf("console items = %v, want only %s", decision.ConsoleItems, consoleItemAC5)
	}
}

func TestC2039_002_EveryCycle2035RoundStopsAndKeepsLaneFindingsListed(t *testing.T) {
	rounds := []struct {
		name     string
		sentinel string
		red      []string
		lane     []string
		handoff  []string
	}{
		{"round 1", cycle2035Round1, []string{failurelearningPkg}, nil, []string{"M1", "M2"}},
		{"round 2", cycle2035Round2, []string{failurelearningPkg}, []string{"M3"}, []string{"M1", "M2"}},
		{"round 3", cycle2035Round3, []string{failurelearningPkg, "go/internal/core"}, []string{"H1", "M3"}, []string{"M1", "M2"}},
		{"round 4", cycle2035Round4, []string{failurelearningPkg + "/floor_test.go"}, nil, []string{"M1", "M2"}},
	}
	for _, r := range rounds {
		t.Run(r.name, func(t *testing.T) {
			decision := fixability.Classify(fixability.Input{
				Defects:          defectsOf(t, r.sentinel),
				RedPaths:         r.red,
				OpenConsoleItems: bothConsoleItemsOpen,
			})
			if !decision.Stop || decision.Reason != fixability.ReasonHandoffBlocker {
				t.Fatalf("stop = %v reason = %q, want a handoff_blocker stop", decision.Stop, decision.Reason)
			}
			requireIDs(t, "lane", decision.Lane, r.lane...)
			requireIDs(t, "handoff", decision.Handoff, r.handoff...)
			requireIDs(t, "blockers", decision.Blockers, "M1")
		})
	}
}

func TestC2039_003_TheHelperFixAloneReturnsNoStopAndListsTheLaneFinding(t *testing.T) {
	decision := fixability.Classify(fixability.Input{
		Defects:          []string{helperFixDefect},
		RedPaths:         []string{"go/internal/core"},
		OpenConsoleItems: bothConsoleItemsOpen,
	})
	if decision.Stop || decision.Reason != "" {
		t.Errorf("a lane finding stopped the cycle: stop = %v reason = %q", decision.Stop, decision.Reason)
	}
	requireIDs(t, "lane", decision.Lane, "H1")
	requireIDs(t, "handoff", decision.Handoff)
	requireIDs(t, "blockers", decision.Blockers)
	for _, lane := range decision.Lane {
		if lane.Severity != "HIGH" {
			t.Errorf("%s severity = %q, want HIGH", lane.ID, lane.Severity)
		}
	}
}

func TestC2039_004_ANonBlockingHandoffFindingIsListedAndDoesNotStop(t *testing.T) {
	for name, defects := range map[string][]string{
		"M2 alone":      {duplicateRulesDefect},
		"M2 beside H1":  {helperFixDefect, duplicateRulesDefect},
		"red elsewhere": {duplicateRulesDefect},
	} {
		t.Run(name, func(t *testing.T) {
			red := []string(nil)
			if name == "red elsewhere" {
				red = []string{"go/internal/unrelated"}
			}
			decision := fixability.Classify(fixability.Input{Defects: defects, RedPaths: red, OpenConsoleItems: bothConsoleItemsOpen})
			if decision.Stop {
				t.Fatalf("a MEDIUM hand-off finding with no red proof stopped the cycle: %+v", decision)
			}
			requireIDs(t, "handoff", decision.Handoff, "M2")
			requireIDs(t, "blockers", decision.Blockers)
			if len(decision.ConsoleItems) != 0 {
				t.Errorf("a non-blocking finding put console items on the decision: %v", decision.ConsoleItems)
			}
		})
	}
}

func TestC2039_005_TheStopCarriesTheSalvagePointerTheFindingsAndTheConsoleItem(t *testing.T) {
	decision := fixability.Classify(fixability.Input{
		Defects:          defectsOf(t, cycle2035Round3),
		RedPaths:         []string{failurelearningPkg},
		OpenConsoleItems: bothConsoleItemsOpen,
		Salvage:          salvageBranch,
	})
	if !decision.Stop {
		t.Fatalf("round 3 did not stop: %+v", decision)
	}
	if decision.Salvage != salvageBranch {
		t.Errorf("salvage = %q, want %q", decision.Salvage, salvageBranch)
	}
	requireIDs(t, "blockers", decision.Blockers, "M1")
	if len(decision.Blockers) == 0 {
		t.FailNow()
	}
	blocker := decision.Blockers[0]
	if blocker.Severity != "MEDIUM" || !strings.Contains(blocker.Text, "floor tests fail") {
		t.Errorf("blocker lost its severity or text: %+v", blocker)
	}
	if !reflect.DeepEqual(blocker.ConsoleItems, []string{consoleItemAC5}) {
		t.Errorf("blocker console items = %v, want %s", blocker.ConsoleItems, consoleItemAC5)
	}
	if !reflect.DeepEqual(decision.ConsoleItems, []string{consoleItemAC5}) {
		t.Errorf("decision console items = %v, want %s", decision.ConsoleItems, consoleItemAC5)
	}
}

func TestC2039_006_AHighHandoffFindingOnAFencePathBlocksWithoutRedProof(t *testing.T) {
	high := fixability.Classify(fixability.Input{Defects: []string{protectedHighDefect}})
	if !high.Stop || high.Reason != fixability.ReasonHandoffBlocker {
		t.Fatalf("a HIGH finding on a protected surface did not stop: %+v", high)
	}
	requireIDs(t, "blockers", high.Blockers, "H2")
	medium := fixability.Classify(fixability.Input{Defects: []string{strings.Replace(protectedHighDefect, "H2:", "M2:", 1)}})
	if medium.Stop {
		t.Fatalf("the same finding at MEDIUM with no red proof stopped the cycle: %+v", medium)
	}
	requireIDs(t, "handoff", medium.Handoff, "M2")
}

func TestC2039_007_OnlyAnOpenConsoleItemMakesAFindingHandoff(t *testing.T) {
	closedItem := fixability.Classify(fixability.Input{
		Defects:  []string{widgetDefect},
		RedPaths: []string{"go/internal/widget"},
	})
	if closedItem.Stop {
		t.Fatalf("a finding that names a console item that is not open stopped the cycle: %+v", closedItem)
	}
	requireIDs(t, "lane", closedItem.Lane, "M7")
	requireIDs(t, "handoff", closedItem.Handoff)

	openItem := fixability.Classify(fixability.Input{
		Defects:          []string{widgetDefect},
		RedPaths:         []string{"go/internal/widget"},
		OpenConsoleItems: []string{"widget-fix-console"},
	})
	if !openItem.Stop {
		t.Fatalf("the same finding with its console item open and red did not stop: %+v", openItem)
	}
	requireIDs(t, "blockers", openItem.Blockers, "M7")
	if !reflect.DeepEqual(openItem.ConsoleItems, []string{"widget-fix-console"}) {
		t.Errorf("console items = %v, want widget-fix-console", openItem.ConsoleItems)
	}
}

func TestC2039_008_RedProofMatchesOnAPathSegmentBoundary(t *testing.T) {
	notASegmentPrefix := fixability.Classify(fixability.Input{
		Defects:          []string{duplicateRulesDefect},
		RedPaths:         []string{"go/internal/inbox"},
		OpenConsoleItems: bothConsoleItemsOpen,
	})
	if notASegmentPrefix.Stop {
		t.Fatalf("go/internal/inbox is not a parent of go/internal/inboxfile, yet it blocked: %+v", notASegmentPrefix)
	}
	citedFileIsRed := fixability.Classify(fixability.Input{
		Defects:          []string{duplicateRulesDefect},
		RedPaths:         []string{"go/internal/inboxfile/inboxfile.go"},
		OpenConsoleItems: bothConsoleItemsOpen,
	})
	if !citedFileIsRed.Stop {
		t.Fatalf("a red path that the finding cites did not block: %+v", citedFileIsRed)
	}
	requireIDs(t, "blockers", citedFileIsRed.Blockers, "M2")
}

func TestC2039_009_TheNewPackageIsEnrolledInTheApicoverGateAndNamesEveryExport(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	// acs-predicate: config-check
	if !acsassert.FileContains(t, filepath.Join(goDir, ".apicover-enforce"), "./internal/fixability\n") {
		t.Fatal("go/.apicover-enforce does not enroll ./internal/fixability")
	}
	pkgDir := filepath.Join(goDir, "internal", "fixability")
	symbols, err := apicover.Enumerate(context.Background(), pkgDir)
	if err != nil {
		t.Fatalf("enumerate %s: %v", pkgDir, err)
	}
	named, err := apicover.NamesReferencedInTests(context.Background(), pkgDir)
	if err != nil {
		t.Fatalf("read the tests of %s: %v", pkgDir, err)
	}
	if len(symbols) == 0 {
		t.Fatal("the fixability package exports nothing")
	}
	for _, s := range symbols {
		if !s.Ignored && !named[s.Name] {
			t.Errorf("exported %s %s is named in no test of the package", s.Kind, s.Name)
		}
	}
}

func parseWorkflow(t *testing.T, workflowJSON string) (policy.Policy, error) {
	t.Helper()
	return policy.Parse([]byte(`{"workflow": ` + workflowJSON + `}`))
}

func TestC2039_010_TheFixabilityStageDefaultsToShadow(t *testing.T) {
	for name, p := range map[string]policy.Policy{
		"no workflow block":   {},
		"no repair block":     {Workflow: &policy.WorkflowPolicy{}},
		"no fixability block": {Workflow: &policy.WorkflowPolicy{Repair: &policy.RepairPolicy{}}},
		"an empty fixability": {Workflow: &policy.WorkflowPolicy{Repair: &policy.RepairPolicy{Fixability: &policy.FixabilityPolicy{}}}},
	} {
		got := p.WorkflowConfig().Repair.Fixability
		if got.Stage != policy.FixabilityShadow || got.Enforced() {
			t.Errorf("%s: fixability = %+v, want shadow and not enforced", name, got)
		}
	}
	fromEmptyBlocks, err := parseWorkflow(t, `{"repair": {"fixability": {}}}`)
	if err != nil {
		t.Fatalf("parse empty fixability block: %v", err)
	}
	if got := fromEmptyBlocks.WorkflowConfig().Repair.Fixability.Stage; got != policy.FixabilityShadow {
		t.Errorf("parsed empty block stage = %q, want shadow", got)
	}
}

func TestC2039_011_TheThreeStageWordsParseAndOnlyEnforceEnforces(t *testing.T) {
	for stage, enforced := range map[string]bool{"off": false, "shadow": false, "enforce": true} {
		p, err := parseWorkflow(t, `{"repair": {"fixability": {"stage": "`+stage+`"}}}`)
		if err != nil {
			t.Fatalf("stage %q refused: %v", stage, err)
		}
		got := p.WorkflowConfig().Repair.Fixability
		if got.Stage != stage || got.Enforced() != enforced {
			t.Errorf("stage %q: config = %+v, Enforced() = %v, want stage kept and Enforced() = %v", stage, got, got.Enforced(), enforced)
		}
	}
	if policy.FixabilityOff != "off" || policy.FixabilityShadow != "shadow" || policy.FixabilityEnforce != "enforce" {
		t.Errorf("stage constants = %q %q %q", policy.FixabilityOff, policy.FixabilityShadow, policy.FixabilityEnforce)
	}
}

func TestC2039_012_AnUnknownStageWordRefusesTheLoadAndNamesTheKey(t *testing.T) {
	for _, stage := range []string{"on", "disabled", "ENFORCE", "advisory"} {
		_, err := parseWorkflow(t, `{"repair": {"fixability": {"stage": "`+stage+`"}}}`)
		if err == nil {
			t.Errorf("stage %q was accepted", stage)
			continue
		}
		if errors.Is(err, policy.ErrUnknownKey) || !strings.Contains(err.Error(), "workflow.repair.fixability.stage") {
			t.Errorf("stage %q: error %q must name workflow.repair.fixability.stage and not be ErrUnknownKey", stage, err)
		}
	}
}

func TestC2039_013_AMisspelledKeyUnderRepairFailsWithErrUnknownKey(t *testing.T) {
	if _, err := parseWorkflow(t, `{"repair": {"fixability": {"stage": "off"}}}`); err != nil {
		t.Fatalf("the spelled key is refused, so the negative cases prove nothing: %v", err)
	}
	for name, workflowJSON := range map[string]string{
		"block under repair": `{"repair": {"fixabilty": {"stage": "off"}}}`,
		"key under block":    `{"repair": {"fixability": {"stages": "off"}}}`,
		"sibling of stage":   `{"repair": {"fixability": {"stage": "off", "mode": "shadow"}}}`,
	} {
		if _, err := parseWorkflow(t, workflowJSON); !errors.Is(err, policy.ErrUnknownKey) {
			t.Errorf("%s: error = %v, want ErrUnknownKey", name, err)
		}
	}
}
