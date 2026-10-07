package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurelearning"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type autofiler struct {
	name  string
	sites []string
	class string
	build func(t *testing.T) inboxbatch.Item
}

var autofilers = []autofiler{
	{"post-push CI watch", []string{"internal/ciwatch.fileEscalation"}, inboxbatch.ClassCorrectness, ciRedItem},
	{"recurrence boundary applier, retro autofile", []string{"internal/retrofile.FileActions"}, inboxbatch.ClassCorrectness, contractGateDemotedItem},
	{"fleet starvation observer", []string{"internal/fleet.WriteTo"}, inboxbatch.ClassStability, starvationItem},
	{"fail-learning floor", []string{"internal/faillearn.writeInboxItems", "internal/faillearn.WriteArtifacts"}, inboxbatch.ClassCorrectness, remediationItem},
	{"triage-cap demotion", []string{"internal/triagecap.autoFileDemotionDefect"}, inboxbatch.ClassCorrectness, demotionItem},
	{"goal-stall escalation", []string{"cmd/evolve.writeTo"}, inboxbatch.ClassStability, goalStallInboxItem},
	{"ADR-0072 halt writer", []string{"cmd/evolve.writePipelineEscalation"}, inboxbatch.ClassStability, haltItem},
	{"unexplained-outcome classifier", []string{"cmd/evolve.fileUnexplainedOutcomeDefect"}, inboxbatch.ClassDebuggability, unexplainedOutcomeItem},
}

const movesOrRewritesAnExistingItem = "moves or rewrites an item that is already filed, so the item keeps the class it was filed with"

var inboxWritersThatFileNothing = map[string]string{
	"internal/inboxmover/lifecycle":             "the lifecycle writer: File refuses an item without a known class (ADR-0121 decision 6); every other write " + movesOrRewritesAnExistingItem,
	"internal/inboxmover.RecordRootTaskFailure": movesOrRewritesAnExistingItem + " (a failed root task's quarantine)",
	"internal/recurrence.applyIntent":           movesOrRewritesAnExistingItem + " (the escalate path's weight write, which plan P5 deletes)",
	"internal/phases/ship.stage":                movesOrRewritesAnExistingItem + " (the ship's consumption into consumed/)",
	"cmd/evolve.runInboxConsume":                movesOrRewritesAnExistingItem + " (evolve inbox consume)",
}

func TestInboxWriters_EveryAutofiledItemCarriesAKnownClass(t *testing.T) {
	order := checkedInClassOrder(t)
	for _, w := range autofilers {
		t.Run(w.name, func(t *testing.T) {
			item := w.build(t)
			if item.PriorityClass != w.class {
				t.Errorf("priority_class = %q, want %q", item.PriorityClass, w.class)
			}
			if err := inboxbatch.CheckPriorityClass(item.PriorityClass, order); err != nil {
				t.Errorf("the checked-in class_order refuses the class: %v", err)
			}
		})
	}
	t.Run("every production write into the inbox is enrolled", func(t *testing.T) {
		sites := inboxWriteSites(t, filepath.Join(findRepoRoot(t), "go"))
		for _, problem := range enrollmentProblems(sites) {
			t.Error(problem)
		}
	})
}

func enrollmentProblems(sites []string) []string {
	var problems []string
	enrolled := map[string]bool{}
	for _, w := range autofilers {
		for _, site := range w.sites {
			enrolled[site] = true
		}
	}
	matched := map[string]bool{}
	for _, site := range sites {
		pkg, _, _ := strings.Cut(site, ".")
		switch {
		case enrolled[site]:
			matched[site] = true
		case inboxWritersThatFileNothing[site] != "":
			matched[site] = true
		case inboxWritersThatFileNothing[pkg] != "":
			matched[pkg] = true
		default:
			problems = append(problems, site+" writes into .evolve/inbox/ but is not enrolled: an autofiler joins the autofilers table with the class it stamps; a writer that only moves or rewrites filed items joins inboxWritersThatFileNothing with its reason")
		}
	}
	for site := range enrolled {
		if !matched[site] {
			problems = append(problems, site+" is enrolled as an autofiler but the scan no longer finds it writing into the inbox: drop or rename the enrollment")
		}
	}
	for key := range inboxWritersThatFileNothing {
		if !matched[key] {
			problems = append(problems, key+" is enrolled as a non-filing inbox writer but the scan no longer finds it: drop the enrollment")
		}
	}
	slices.Sort(problems)
	return problems
}

func checkedInClassOrder(t *testing.T) []string {
	t.Helper()
	pol, err := policy.Load(filepath.Join(findRepoRoot(t), ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	return pol.InboxPriorityConfig().ClassOrder
}

func onlyItemIn(t *testing.T, dir string) inboxbatch.Item {
	t.Helper()
	items, warnings, err := inboxbatch.LoadDir(dir)
	if err != nil || len(warnings) > 0 || len(items) != 1 {
		t.Fatalf("want exactly one item in %s: %d items, warnings %v, err %v", dir, len(items), warnings, err)
	}
	return items[0]
}

func itemAt(t *testing.T, path string) inboxbatch.Item {
	t.Helper()
	item, warnings, err := inboxbatch.LoadFile(path)
	if err != nil || len(warnings) > 0 {
		t.Fatalf("decode %s: warnings %v, err %v", path, warnings, err)
	}
	return item
}

func autofilerClock() time.Time { return time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC) }

func ciRedItem(t *testing.T) inboxbatch.Item {
	t.Helper()
	inbox := t.TempDir()
	red := func(context.Context, string) (ciwatch.RunStatus, error) {
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: "failure", FailingTest: "TestX"}, nil
	}
	opts := ciwatch.Options{SHA: "deadbeefcafe0123", InboxDir: inbox, Fetch: red, Now: autofilerClock, Sleep: func(time.Duration) {}}
	if _, err := ciwatch.Watch(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	return onlyItemIn(t, inbox)
}

func contractGateDemotedItem(t *testing.T) inboxbatch.Item {
	t.Helper()
	root := t.TempDir()
	escalations := filepath.Join(root, "escalations")
	intent := core.ContractGateDemotion{Phase: core.PhaseBuild, CLI: "codex-tmux", Cycle: 41, Blocks: 2, Weight: 0.75, Reason: "the gate demoted itself"}.Intent()
	if _, err := dispositionrouter.StageIntent(escalations, intent); err != nil {
		t.Fatal(err)
	}
	_, err := recurrence.ApplyBoundary(recurrence.ApplyOptions{
		InboxDir: filepath.Join(root, "inbox"), EscalationsPath: dispositionrouter.PendingActionsPath(escalations),
		ReportPath: filepath.Join(root, "report.json"), Cycle: 42, Policy: recurrence.DefaultEscalationPolicy(), Now: autofilerClock(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return onlyItemIn(t, filepath.Join(root, "inbox"))
}

func starvationItem(t *testing.T) inboxbatch.Item {
	t.Helper()
	item := fleet.BuildStarvationItem(fleet.WaveObservation{DesiredLanes: 3, RealizedLanes: 1}, 3, 0.9, 1, autofilerClock().Format(time.RFC3339))
	path, err := item.WriteTo(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return itemAt(t, path)
}

func remediationItem(t *testing.T) inboxbatch.Item {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	failure := failurelearning.Failure{Cycle: 9, Phase: cyclestate.PhaseAudit, ProjectRoot: root, Workspace: ws}
	learned := failurelearning.Learned{Summary: "audit failed", Structured: &phasecontract.FailureBlock{Class: "deliverable-rejected", Defects: []string{"the ledger row lacks evidence"}}}
	failurelearning.New(autofilerClock, carryover.New()).WriteFloor(failure, learned)
	return onlyItemIn(t, filepath.Join(root, ".evolve", "inbox"))
}

func demotionItem(t *testing.T) inboxbatch.Item {
	t.Helper()
	raw, err := json.Marshal(triagecap.NewDemotionLedgerRecord(303, 301, 302, "identical reason", triagecap.RemedyPending))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "demotion.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return itemAt(t, path)
}

func goalStallInboxItem(t *testing.T) inboxbatch.Item {
	t.Helper()
	item := buildGoalStallItem(goalStallKind, "abcd1234ef", &goalStallEscalation{streak: 3}, 0.9, 1, autofilerClock().Format(time.RFC3339))
	path, err := item.writeTo(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return itemAt(t, path)
}

func haltItem(t *testing.T) inboxbatch.Item {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	sf := &cyclestate.SystemFailureSignal{Category: "verdict-incoherence", Level: "system", Evidence: "recorded FAIL, audit PASS", Halt: true}
	rec := writePipelineEscalation(evolveDir, root, 899, filepath.Join(evolveDir, "runs", "cycle-899"), sf, io.Discard)
	return itemAt(t, rec.InboxItemPath)
}

func unexplainedOutcomeItem(t *testing.T) inboxbatch.Item {
	t.Helper()
	root := t.TempDir()
	fileUnexplainedOutcomeDefect(root, 77, "no outcome recorded")
	return onlyItemIn(t, filepath.Join(root, ".evolve", "inbox"))
}

func TestInboxWriteSites_FindsEveryShapeOfAnInboxWriteAndNoRead(t *testing.T) {
	module := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(module, "internal", "filer", "filer.go"), `package filer

import (
	"os"
	"path/filepath"

	"example.com/m/internal/atomicwrite"
)

func Direct(evolveDir, id string) error {
	return os.WriteFile(filepath.Join(evolveDir, "inbox", id+".json"), nil, 0o644)
}

func ThroughAHelper(evolveDir string) error { return writeItem(itemPath(evolveDir)) }

func itemPath(evolveDir string) string { return filepath.Join(evolveDir, "inbox", "x.json") }

func writeItem(path string) error { return os.WriteFile(path+".tmp", nil, 0o644) }

func Atomic(inboxDir string) error { return atomicwrite.JSON(filepath.Join(inboxDir, "y.json"), 1) }

func ViaInboxPath(inboxPath string) error { return os.WriteFile(filepath.Join(inboxPath, "p.json"), nil, 0o644) }

func ViaInboxRoot(inboxRoot string) error { return os.WriteFile(filepath.Join(inboxRoot, "r.json"), nil, 0o644) }

func RenameIn(src, root string) error {
	dest := filepath.Join(root, ".evolve/inbox/z.json")
	return os.Rename(src, dest)
}

func ReadsOnly(evolveDir string) ([]byte, error) {
	return os.ReadFile(filepath.Join(evolveDir, "inbox", "x.json"))
}

func WritesElsewhere(runDir string) error {
	return os.WriteFile(filepath.Join(runDir, "report.json"), nil, 0o644)
}
`)
	fixtures.MustWrite(t, filepath.Join(module, "internal", "filer", "filer_test.go"), "package filer\n\nimport \"os\"\n\nfunc testOnly(inboxDir string) { _ = os.WriteFile(inboxDir, nil, 0o644) }\n")
	want := []string{"internal/filer.Atomic", "internal/filer.Direct", "internal/filer.RenameIn", "internal/filer.ThroughAHelper", "internal/filer.ViaInboxPath", "internal/filer.ViaInboxRoot"}
	if got := inboxWriteSites(t, module); !slices.Equal(got, want) {
		t.Errorf("inboxWriteSites = %v, want %v", got, want)
	}
}

func TestEnrollmentProblems_NameAnUnenrolledWriterAndAStaleEnrollment(t *testing.T) {
	var sites []string
	for _, w := range autofilers[1:] {
		sites = append(sites, w.sites...)
	}
	for key := range inboxWritersThatFileNothing {
		sites = append(sites, key)
	}
	sites = append(sites, "internal/ninth.fileItem")
	problems := strings.Join(enrollmentProblems(sites), "\n")
	if !strings.Contains(problems, "internal/ninth.fileItem writes into .evolve/inbox/ but is not enrolled") {
		t.Errorf("a ninth writer must be named: %s", problems)
	}
	if !strings.Contains(problems, "internal/ciwatch.fileEscalation is enrolled as an autofiler but the scan no longer finds it") {
		t.Errorf("a stale enrollment must be named: %s", problems)
	}
}
