package failurelearning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// floorFixture is the remediation fixture (retro_remediation_filter_test.go):
// a project root whose .evolve/inbox can be inspected and the cycle workspace
// under it; the clock is Unix 1754000000.
func floorFixture(t *testing.T) (root, ws string, f Failure) {
	t.Helper()
	root = t.TempDir()
	ws = filepath.Join(root, ".evolve", "runs", "cycle-1279")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, ws, Failure{Cycle: 1279, Phase: cyclestate.PhaseAudit, ProjectRoot: root, Workspace: ws}
}

func floorClock() time.Time { return time.Unix(1754000000, 0).UTC() }

func inboxFiles(t *testing.T, root string) []string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(root, ".evolve", "inbox"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

func readGolden(t *testing.T, name, ws string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "{{WS}}", ws)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return string(b)
}

func warnsOf(events []signalcenter.Event, code signalcenter.Code) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

// Test 17 — the pure projection of a Failure + Learned onto the lesson event.
func TestFloorEvent_ProjectsTheFailureAndPrefersTheStructuredBlock(t *testing.T) {
	f := Failure{Cycle: 9, Phase: cyclestate.PhaseBuild, ProjectRoot: "root", Workspace: "ws"}
	ev := floorEvent(f, Learned{Summary: "s"}, fixedNow)
	if ev.Cycle != 9 || ev.FailedPhase != "build" || ev.Scope != faillearn.ScopePhase || ev.Classification != cyclestate.ClassificationMidExecutionFail ||
		ev.Verdict != cyclestate.VerdictFAIL || ev.Summary != "s" || len(ev.Defects) != 1 || ev.Defects[0] != "s" ||
		len(ev.EvidencePaths) != 1 || ev.EvidencePaths[0] != "ws" || !ev.Now.Equal(fixedNow) {
		t.Fatalf("defaults: %+v", ev)
	}
	fb := &phasecontract.FailureBlock{Class: "code-build-fail", Defects: []string{"d1"}, EvidencePaths: []string{"e1", "e2"}}
	ev = floorEvent(f, Learned{Summary: "s", Structured: fb}, fixedNow)
	if ev.Classification != "code-build-fail" || len(ev.Defects) != 1 || ev.Defects[0] != "d1" ||
		strings.Join(ev.EvidencePaths, ",") != "e1,e2,ws" {
		t.Fatalf("the block's class/defects; its evidence with the workspace appended LAST: %+v", ev)
	}
	ev = floorEvent(f, Learned{Summary: "s", Structured: &phasecontract.FailureBlock{Class: "code-build-fail"}}, fixedNow)
	if ev.Defects[0] != "s" || ev.EvidencePaths[0] != "ws" {
		t.Fatalf("a block without defects/evidence keeps the defaults: %+v", ev)
	}
}

// Test 18 — the floor's artifacts are byte-identical to the goldens captured
// on 97825125: the report, the lesson, the two inbox items, the ledger.
func TestWriteFloor_WritesLessonAndReportByteIdenticalToTheGolden(t *testing.T) {
	root, ws, f := floorFixture(t)
	e := New(floorClock, carryover.New())
	e.WriteFloor(f, Learned{Summary: "audit phase exited 1 after 3 attempts", Structured: &phasecontract.FailureBlock{
		Class: "deliverable-rejected", Defects: []string{"the ledger row lacks evidence", "the manifest row lacks a path"}, EvidencePaths: []string{"docs/x.md"}}})
	if got, want := readFile(t, filepath.Join(ws, "retrospective-report.md")), readGolden(t, "floor-report.golden.md", ws); got != want {
		t.Fatalf("retrospective-report.md drifted:\n%s\n--- want\n%s", got, want)
	}
	if got, want := readFile(t, filepath.Join(root, ".evolve", "instincts", "lessons", "cycle-1279-phase-audit.yaml")), readGolden(t, "floor-lesson.golden.yaml", ws); got != want {
		t.Fatalf("the lesson drifted:\n%s\n--- want\n%s", got, want)
	}
	for _, item := range []string{"retro-1279-the-ledger-row-lacks-evidence-915304b4", "retro-1279-the-manifest-row-lacks-a-path-4f0ecc16"} {
		if got, want := readFile(t, filepath.Join(root, ".evolve", "inbox", item+".json")), readGolden(t, "inbox-"+item[len(item)-8:]+".golden.json", ws); got != want {
			t.Fatalf("inbox item %s drifted:\n%s", item, got)
		}
	}
	if got, want := readFile(t, filepath.Join(root, ".evolve", "recurrence-ledger.json")), readGolden(t, "recurrence-ledger.golden.json", ws); got != want {
		t.Fatalf("the recurrence ledger drifted:\n%s", got)
	}
}

func TestWriteFloor_FilesOnlyStructuredDefectsAsInbox(t *testing.T) {
	summary := "audit phase exited 1 after 3 attempts"
	cases := []struct {
		name string
		fb   *phasecontract.FailureBlock
		want int
	}{
		{"classed but defectless", &phasecontract.FailureBlock{Class: "deliverable-rejected"}, 0},
		{"the summary echo", &phasecontract.FailureBlock{Class: "deliverable-rejected", Defects: []string{summary}}, 0},
		{"no block", nil, 0},
		{"self-reported defects", &phasecontract.FailureBlock{Class: "deliverable-rejected", Defects: []string{"a real defect"}}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, ws, f := floorFixture(t)
			New(floorClock, carryover.New()).WriteFloor(f, Learned{Summary: summary, Structured: tc.fb})
			if got := inboxFiles(t, root); len(got) != tc.want {
				t.Fatalf("inbox: %v, want %d item(s)", got, tc.want)
			}
			if _, err := os.Stat(filepath.Join(ws, "retrospective-report.md")); err != nil {
				t.Fatalf("the retrospective is always written: %v", err)
			}
		})
	}
}

// Test 19 — ONE policy read: a present-but-unreadable policy.json is exactly
// one FAILURELEARNING_POLICY_LOAD_FAILED; the floor proceeds on the compiled
// defaults; an absent file is silent.
func TestWriteFloor_PolicyLoadFailsOnceAndFallsBackToCompiledDefaults(t *testing.T) {
	root, _, f := floorFixture(t)
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "policy.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	e, got := observed(t)
	e.WriteFloor(f, Learned{Summary: "s", Structured: &phasecontract.FailureBlock{Class: "deliverable-rejected", Defects: []string{"a real defect"}}})
	w := warnsOf(*got, CodePolicyLoadFailed)
	if len(w) != 1 {
		t.Fatalf("exactly ONE policy WARN for the one read (two reads fired two stderr lines before): %+v", *got)
	}
	ev := w[0]
	if ev.Module != signalcenter.ModuleFailureLearning || ev.Kind != signalcenter.KindFailureLearningWarning || ev.Severity != signalcenter.SeverityWarn ||
		ev.Origin != "Engine.WriteFloor" || ev.Cycle != 1279 || ev.Phase != "audit" || ev.Fields["step"] != "floor" ||
		ev.Fields["path"] != filepath.Join(root, ".evolve", "policy.json") || !strings.HasPrefix(ev.Reason, "policy load failed (using compiled defaults): ") {
		t.Fatalf("the WARN's module/kind/severity/origin/cycle/phase/fields: %+v", ev)
	}
	items := inboxFiles(t, root)
	if len(items) != 1 || !strings.Contains(readFile(t, filepath.Join(root, ".evolve", "inbox", items[0])), `"weight": 0.75`) {
		t.Fatalf("the remediation is still filed at the compiled weight: %v", items)
	}
	root2, _, f2 := floorFixture(t)
	e2, got2 := observed(t)
	e2.WriteFloor(f2, Learned{Summary: "s"})
	if len(*got2) != 0 {
		t.Fatalf("an absent policy.json is silent (%s): %+v", root2, *got2)
	}
}

// Test 20 — a floor write failure is one WARN with the wrapped error, and the
// recurrence closure STILL lands (the closure is unconditional).
func TestWriteFloor_WriteFailureIsAWarnSignalAndTheClosureStillLands(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(ws, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, got := observed(t)
	f := Failure{Cycle: 1279, Phase: cyclestate.PhaseAudit, ProjectRoot: root, Workspace: ws}
	e.WriteFloor(f, Learned{Summary: "s", Structured: &phasecontract.FailureBlock{Class: "deliverable-rejected"}})
	w := warnsOf(*got, CodeFloorWriteFailed)
	if len(w) != 1 || w[0].Fields["step"] != "floor" || w[0].Fields["workspace"] != ws ||
		w[0].Fields["lessons_dir"] != filepath.Join(root, ".evolve", "instincts", "lessons") || !strings.HasPrefix(w[0].Reason, "deterministic fallback write: ") || w[0].Origin != "Engine.WriteFloor" {
		t.Fatalf("one FLOOR_WRITE_FAILED with step/workspace/lessons_dir and the wrapped error: %+v", *got)
	}
	led, err := recurrence.Load(filepath.Join(root, ".evolve", "recurrence-ledger.json"))
	if err != nil || led.Count("deliverable-rejected") != 1 {
		t.Fatalf("the closure still lands after a write failure: err=%v", err)
	}
}

// Test 21 — the closure is keyed by the class and records the cycle; the
// guard (empty root / blank class) writes nothing.
func TestWriteFloor_RecordsTheRecurrenceClosureKeyedByCycleAndLast(t *testing.T) {
	root, _, f := floorFixture(t)
	e, _ := observed(t)
	e.WriteFloor(f, Learned{Summary: "s"})
	led, err := recurrence.Load(filepath.Join(root, ".evolve", "recurrence-ledger.json"))
	if err != nil || led.Count(cyclestate.ClassificationMidExecutionFail) != 1 {
		t.Fatalf("the default class is the closure's pattern: err=%v", err)
	}
	e.recurrenceClosure(Failure{Cycle: 1, ProjectRoot: "", Workspace: ""}, "x")
	e.recurrenceClosure(Failure{Cycle: 1, ProjectRoot: root, Workspace: ""}, "   ")
	if _, err := os.Stat(filepath.Join(root, ".evolve", "recurrence-ledger.json")); err != nil {
		t.Fatal(err)
	}
	if led2, _ := recurrence.Load(filepath.Join(root, ".evolve", "recurrence-ledger.json")); led2.Count("x") != 0 || led2.Count("   ") != 0 || len(led2.Entries) != 1 {
		t.Fatalf("the guard: no root or a blank pattern records nothing: %+v", led2.Entries)
	}
}

// Test 22 — the ledger WARN names the failing call: op=load for a directory
// at the ledger path; op=save under a read-only .evolve whose lock file
// already exists (a prior Save created it — flock keeps it), so the load
// succeeds and only the save fails.
func TestRecurrenceClosure_OpNamesTheFailingCall(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "recurrence-ledger.json"), 0o755); err != nil {
			t.Fatal(err)
		}
		e, got := observed(t)
		e.recurrenceClosure(Failure{Cycle: 3, Phase: cyclestate.PhaseAudit, ProjectRoot: root}, "p")
		w := warnsOf(*got, CodeRecurrenceLedgerFailed)
		if len(w) != 1 || w[0].Fields["op"] != "load" || w[0].Fields["step"] != "recurrence" || w[0].Fields["path"] != filepath.Join(root, ".evolve", "recurrence-ledger.json") ||
			!strings.HasPrefix(w[0].Reason, "recurrence ledger load failed: ") {
			t.Fatalf("op=load: %+v", *got)
		}
	})
	t.Run("save", func(t *testing.T) {
		root := t.TempDir()
		evolveDir := filepath.Join(root, ".evolve")
		path := filepath.Join(evolveDir, "recurrence-ledger.json")
		if err := os.MkdirAll(evolveDir, 0o755); err != nil {
			t.Fatal(err)
		}
		e, got := observed(t)
		e.recurrenceClosure(Failure{Cycle: 3, ProjectRoot: root}, "p") // a prior Save: the ledger AND its lock file now exist
		if len(*got) != 0 {
			t.Fatalf("the seeding save must succeed: %+v", *got)
		}
		if err := os.Chmod(evolveDir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })
		e.recurrenceClosure(Failure{Cycle: 4, ProjectRoot: root}, "p")
		w := warnsOf(*got, CodeRecurrenceLedgerFailed)
		if len(w) != 1 || w[0].Fields["op"] != "save" || !strings.HasPrefix(w[0].Reason, "recurrence ledger save failed: ") || w[0].Fields["path"] != path {
			t.Fatalf("op=save (the load succeeded through the existing lock file; the save actually failed): %+v", *got)
		}
		if led, err := recurrence.Load(path); err == nil && led.Count("p") != 1 {
			t.Fatalf("the failed save left the ledger at the seeded state: count=%d", led.Count("p"))
		}
	})
}

// Test 23 — the remediation cap is recorded, never silent.
func TestRemediationItems_CapsAtThirtyTwoAndSignalsTheTruncation(t *testing.T) {
	e, got := observed(t)
	defects := make([]string, 33)
	for i := range defects {
		defects[i] = "defect number " + strings.Repeat("x", i+1)
	}
	items := e.remediationItems(Failure{Cycle: 5, Phase: cyclestate.PhaseAudit}, defects, 0.5)
	w := warnsOf(*got, CodeRemediationTruncated)
	if len(items) != remediationMaxItems || len(w) != 1 || w[0].Fields["reported"] != "33" || w[0].Fields["filed"] != "32" || w[0].Fields["step"] != "floor" || w[0].Origin != "Engine.WriteFloor" || w[0].Cycle != 5 {
		t.Fatalf("32 filed + one TRUNCATED WARN {reported 33, filed 32}: %d items, %+v", len(items), w)
	}
	e2, got2 := observed(t)
	if items := e2.remediationItems(Failure{Cycle: 5}, defects[:32], 0.5); len(items) != 32 || len(*got2) != 0 {
		t.Fatalf("exactly 32: no truncation, no WARN: %d, %+v", len(items), *got2)
	}
}

// Test 24 — the item shape and the unnameable-defect rule.
func TestRemediationItems_ShapeAndUnnameableDefectsSkipped(t *testing.T) {
	e := New(floorClock, carryover.New())
	long := strings.Repeat("d", 600)
	items := e.remediationItems(Failure{Cycle: 7}, []string{"  " + long + "  ", "!!!", "   ", "Fix the thing"}, 0.42)
	if len(items) != 2 {
		t.Fatalf("unnameable defects yield no item: %+v", items)
	}
	if items[0].Title != carryover.TruncateRunes(long, remediationTitleMaxRunes) || items[0].Weight != 0.42 || items[0].Kind != "bug" || items[0].Priority != "H" || items[0].InjectedBy != "faillearn-failure-floor" {
		t.Fatalf("the item: title capped through carryover.TruncateRunes, weight from the caller, bug/H, the floor's provenance: %+v", items[0])
	}
	if items[1].ID != "retro-7-fix-the-thing-"+remediationFingerprint("Fix the thing") {
		t.Fatalf("id = retro-<cycle>-<slug>-<8hex>: %s", items[1].ID)
	}
}

// Test 25 — the cycle-1285 premise: ids are injective over the FULL title.
func TestRemediationItems_IDsAreInjectiveOverTheFullTitle(t *testing.T) {
	if remediationSlugMaxRunes != 60 {
		t.Fatalf("the fixture below assumes a 60-rune slug bound, got %d", remediationSlugMaxRunes)
	}
	const prefix = "evidenceResolves accepts an unrelated in-repo file as closure evidence"
	items := New(floorClock, carryover.New()).remediationItems(Failure{Cycle: 1285}, []string{prefix + " for the ledger row", prefix + " for the manifest row"}, 0.5)
	if len(items) != 2 || items[0].ID == items[1].ID {
		t.Fatalf("two defect lines diverging only after rune 60 must mint TWO ids: %+v", items)
	}
}

// Test 26 — the pure id helpers.
func TestRemediationSlug_LowercasesHyphenatesAndStopsAtSixty(t *testing.T) {
	if got := remediationSlug("  Hello, World!!  x"); got != "hello-world-x" {
		t.Fatalf("lowercase, runs of non-alphanumerics to one hyphen, trimmed: %q", got)
	}
	if got := remediationSlug(strings.Repeat("ab", 40)); len(got) != remediationSlugMaxRunes {
		t.Fatalf("stops at %d: %d", remediationSlugMaxRunes, len(got))
	}
	if remediationSlug("!!!") != "" {
		t.Fatal("nothing nameable ⇒ empty")
	}
}

func TestRemediationFingerprint_IsEightHexOfSHA256(t *testing.T) {
	got := remediationFingerprint("the ledger row lacks evidence")
	if got != "915304b4" || remediationFingerprint("x") == remediationFingerprint("y") {
		t.Fatalf("the first 8 hex of sha256 (the golden id's tail): %q", got)
	}
}
