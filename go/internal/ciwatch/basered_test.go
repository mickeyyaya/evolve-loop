package ciwatch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func baseRedReport() ClassifyReport {
	failure := func(pkg, test string, baseRed, rerunGreen Fact) ClassifiedFailure {
		return ClassifiedFailure{
			FailingTest: FailingTest{Package: pkg, Test: test, Jobs: []string{"test (ubuntu)"}},
			Evidence:    Evidence{Touched: FactNo, BaseRed: baseRed, RecurredOnMain: FactUnknown, RerunGreen: rerunGreen},
		}
	}
	return ClassifyReport{
		Target: "pr:12", RunID: 501, RunURL: "https://ci/runs/501", Conclusion: "failure",
		HeadSHA: "0123456789abcdef0123", BaseSHA: "fedcba9876543210fedc",
		Failures: []ClassifiedFailure{
			failure("example.com/m/internal/old", "TestOld_IsRedOnMain", FactYes, FactNo),
			failure("example.com/m/internal/new", "TestNew_IsThisChange", FactNo, FactNo),
			failure("example.com/m/internal/maybe", "TestMaybe", FactUnknown, FactNo),
			failure("example.com/m/internal/flaky", "TestFlaky_GreenOnRerun", FactYes, FactYes),
			failure("example.com/m/internal/unrun", "TestUnrun_NoRerunYet", FactYes, FactUnknown),
			{FailingTest: FailingTest{Step: "make apicover-check", Jobs: []string{"build"}}, Evidence: Evidence{BaseRed: FactYes, RerunGreen: FactNo}},
		},
	}
}

func readInboxItems(t *testing.T, dir string) []escalationItem {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	items := make([]escalationItem, 0, len(paths))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var it escalationItem
		if err := json.Unmarshal(raw, &it); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		items = append(items, it)
	}
	return items
}

func TestFileBaseRedItems_OneFixFirstItemPerBaseRedFailureAndNoneOnASecondRun(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	filed, err := FileBaseRedItems(inbox, baseRedReport(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(filed) != 2 {
		t.Fatalf("filed %v, want one item for the base-red test and one for the base-red step", filed)
	}
	items := readInboxItems(t, inbox)
	if len(items) != 2 {
		t.Fatalf("inbox holds %d items, want 2", len(items))
	}
	var test escalationItem
	for _, it := range items {
		if strings.Contains(it.Title, "TestOld_IsRedOnMain") {
			test = it
		}
		if strings.Contains(it.Title, "TestNew") || strings.Contains(it.Title, "TestMaybe") {
			t.Errorf("a failure that is not red on the base was filed: %+v", it)
		}
		if strings.Contains(it.Title, "TestFlaky") || strings.Contains(it.Title, "TestUnrun") {
			t.Errorf("a failure that is not proven red on the head after a rerun was filed: %+v", it)
		}
	}
	if test.ID == "" || !strings.HasPrefix(test.ID, BaseRedItemPrefix) || test.Source != BaseRedSource || test.Evidence != "https://ci/runs/501" {
		t.Errorf("base-red item = %+v, want a %s item from %s that cites the run", test, BaseRedItemPrefix, BaseRedSource)
	}
	if test.Kind != "fix" || test.PriorityClass != inboxbatch.ClassCorrectness || !strings.Contains(test.Summary, "example.com/m/internal/old") || !strings.Contains(test.Summary, "fedcba987654") {
		t.Errorf("base-red item = %+v, want a correctness fix that names the package and the base", test)
	}

	again, err := FileBaseRedItems(inbox, baseRedReport(), now.Add(time.Hour))
	if err != nil || len(again) != 0 {
		t.Fatalf("second run filed %v (err %v), want none for the same reds", again, err)
	}
	if n := len(readInboxItems(t, inbox)); n != 2 {
		t.Errorf("inbox holds %d items after the second run, want 2", n)
	}
}

func TestFileBaseRedItems_AnItemInAnyPendingFolderStillDeduplicates(t *testing.T) {
	for name, folder := range map[string]func(inbox string) string{
		"claimed by a cycle": func(inbox string) string { return inboxbatch.ProcessingCycleDir(inbox, 7) },
		"waiting for retry":  func(inbox string) string { return filepath.Join(inbox, "retry") },
		"quarantined":        func(inbox string) string { return filepath.Join(inbox, "quarantine") },
	} {
		t.Run(name, func(t *testing.T) {
			inbox := filepath.Join(t.TempDir(), "inbox")
			now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
			if _, err := FileBaseRedItems(inbox, baseRedReport(), now); err != nil {
				t.Fatal(err)
			}
			dest := folder(inbox)
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatal(err)
			}
			paths, _ := filepath.Glob(filepath.Join(inbox, "*.json"))
			for _, p := range paths {
				if err := os.Rename(p, filepath.Join(dest, filepath.Base(p))); err != nil {
					t.Fatal(err)
				}
			}
			again, err := FileBaseRedItems(inbox, baseRedReport(), now.Add(time.Hour))
			if err != nil || len(again) != 0 {
				t.Errorf("filed %v (err %v) while the items are %s, want none", again, err, name)
			}
		})
	}
}

func TestFileBaseRedItems_DifferentTestsGetDifferentIDsAndAGreenReportFilesNothing(t *testing.T) {
	a := baseRedItemID(FailingTest{Package: "p/a", Test: "TestX"})
	b := baseRedItemID(FailingTest{Package: "p/b", Test: "TestX"})
	if a == b || !strings.Contains(a, "testx") {
		t.Errorf("ids %q and %q: two packages with one test name must not collide, and the id must name the test", a, b)
	}
	if got := baseRedItemID(FailingTest{Jobs: []string{"lint"}}); !strings.HasPrefix(got, BaseRedItemPrefix+"lint-") {
		t.Errorf("job-only failure id = %q, want it to name the job", got)
	}
	long := baseRedItemID(FailingTest{Package: "p", Test: "TestAVeryLongName_" + strings.Repeat("Ab_", 30)})
	slug := strings.TrimPrefix(long[:len(long)-9], BaseRedItemPrefix)
	if len(slug) > baseRedSlugMax || strings.HasSuffix(slug, "-") || !strings.HasPrefix(slug, "testaverylongname-ab-") {
		t.Errorf("long-name id = %q: the slug must stop at %d characters and never end in a dash", long, baseRedSlugMax)
	}
	inbox := filepath.Join(t.TempDir(), "inbox")
	filed, err := FileBaseRedItems(inbox, ClassifyReport{Conclusion: ConclusionSuccess}, time.Now())
	if err != nil || len(filed) != 0 {
		t.Errorf("green report filed %v (err %v)", filed, err)
	}
	if _, err := os.Stat(inbox); !os.IsNotExist(err) {
		t.Errorf("green report created the inbox dir (stat err %v)", err)
	}
}

func TestFileBaseRedItems_AnUnreadableOrUnwritableInboxIsAnError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "inbox")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FileBaseRedItems(file, baseRedReport(), time.Now()); err == nil || !strings.Contains(err.Error(), "ciwatch: base-red dedup") {
		t.Errorf("err = %v, want the dedup scan error", err)
	}

	inbox := filepath.Join(t.TempDir(), "inbox")
	if err := os.MkdirAll(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
	if _, err := FileBaseRedItems(inbox, baseRedReport(), time.Now()); err == nil || !strings.Contains(err.Error(), "ciwatch: write escalation:") {
		t.Errorf("err = %v, want the write error", err)
	}
}

func TestFileBaseRedItems_AnUnreadableProcessingDirIsAnError(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	claimed := inboxbatch.ProcessingCycleDir(inbox, 3)
	if err := os.MkdirAll(claimed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(claimed, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(claimed, 0o755) })
	if _, err := FileBaseRedItems(inbox, baseRedReport(), time.Now()); err == nil || !strings.Contains(err.Error(), "ciwatch: base-red dedup") {
		t.Errorf("err = %v, want the dedup scan error of the claimed dir", err)
	}
}

func TestFileBaseRedItems_MarshalFailureReturnsError(t *testing.T) {
	orig := marshalIndent
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("encode boom") }
	t.Cleanup(func() { marshalIndent = orig })

	inbox := t.TempDir()
	_, err := FileBaseRedItems(inbox, baseRedReport(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "ciwatch: marshal base-red item: encode boom") {
		t.Fatalf("err = %v, want marshal error", err)
	}
}

func TestFileBaseRedItems_ALongOddTestNameIsStillFiled(t *testing.T) {
	inbox := t.TempDir()
	rep := baseRedReport()
	rep.Failures = []ClassifiedFailure{{
		FailingTest: FailingTest{Package: "example.com/m/internal/old", Test: "TestX_" + strings.Repeat("a_very_long_descriptive_name_", 8) + "/case\twith\ttabs"},
		Evidence:    Evidence{BaseRed: FactYes, RerunGreen: FactNo},
	}}

	filed, err := FileBaseRedItems(inbox, rep, time.Now())

	if err != nil || len(filed) != 1 {
		t.Fatalf("filed=%v err=%v; a long or odd test name must still file the fix-first item", filed, err)
	}
	items := readInboxItems(t, inbox)
	if len(items) != 1 || len(items[0].Title) > inboxbatch.MaxFieldLen || strings.ContainsAny(items[0].Title, "\t\n") {
		t.Errorf("items = %+v, want one item with a bounded, control-free title", items)
	}
}

func TestFileBaseRedItems_ARefusedItemDoesNotStopTheOthers(t *testing.T) {
	orig := marshalIndent
	calls := 0
	marshalIndent = func(v any, prefix, indent string) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("encode boom")
		}
		return orig(v, prefix, indent)
	}
	t.Cleanup(func() { marshalIndent = orig })
	inbox := t.TempDir()
	rep := baseRedReport()
	rep.Failures = append(rep.Failures, ClassifiedFailure{
		FailingTest: FailingTest{Package: "example.com/m/internal/second", Test: "TestSecond_IsRedOnMain"},
		Evidence:    Evidence{BaseRed: FactYes, RerunGreen: FactNo},
	})

	filed, err := FileBaseRedItems(inbox, rep, time.Now())

	if err == nil || !strings.Contains(err.Error(), "encode boom") {
		t.Fatalf("err = %v, want the refusal of the first item reported", err)
	}
	if len(filed) < 2 {
		t.Errorf("filed = %v, want every other red still filed", filed)
	}
}

func TestFileBaseRedItems_AFailureWithNoNameStillFilesAKebabID(t *testing.T) {
	inbox := t.TempDir()
	rep := baseRedReport()
	rep.Failures = []ClassifiedFailure{{
		FailingTest: FailingTest{Jobs: []string{}},
		Evidence:    Evidence{BaseRed: FactYes, RerunGreen: FactNo},
	}}

	filed, err := FileBaseRedItems(inbox, rep, time.Now())

	if err != nil || len(filed) != 1 || strings.Contains(filed[0], "--") {
		t.Fatalf("filed=%v err=%v; a nameless red must still file a kebab id", filed, err)
	}
}
