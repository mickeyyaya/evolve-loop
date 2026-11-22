package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
)

const originMainTip = "5c0ffee5deadbeef"

var originMainFailedJobs = []string{"build+test=failure", "lint=timed_out"}

func ciKeysOf(jobs ...string) []baseattr.Key {
	keys := make([]baseattr.Key, len(jobs))
	for i, job := range jobs {
		keys[i] = baseattr.Key{Package: CIPackage, Test: job}
	}
	return keys
}

func TestMainRedStop_CIOnlyRedNeedsNoLaneReproduction(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	opened, err := OpenMainCIRedStop(root, MainCIRed{Main: originMainTip, Failed: originMainFailedJobs})

	if !opened || err != nil {
		t.Fatalf("opened = %v err = %v, want a red origin/main CI to open the stop with no lane verdict", opened, err)
	}
	stop, found, err := mainstop.Read(filepath.Join(root, ".evolve"))
	if err != nil || !found || !stop.Holds() {
		t.Fatalf("stop = %+v found = %v err = %v, want a held stop", stop, found, err)
	}
	if !slices.Equal(stop.Keys, ciKeysOf(originMainFailedJobs...)) || stop.MainSHA != originMainTip {
		t.Errorf("keys = %v main = %q, want one CI key for each failed job at the origin/main tip", stop.Keys, stop.MainSHA)
	}
	if len(stop.Parked) != 0 {
		t.Errorf("parked = %+v, want no lane parked by a stop that no lane opened", stop.Parked)
	}
}

func TestMainRedStop_CIOnlyRedFilesAFixItemThatNamesTheFailedJobsNotALane(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if _, err := OpenMainCIRedStop(root, MainCIRed{Main: originMainTip, Failed: originMainFailedJobs}); err != nil {
		t.Fatal(err)
	}

	paths, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("pending items = %v err = %v, want one fix item", paths, err)
	}
	raw, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var item mainRedFixItem
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	if item.ID != mainstop.FixItemID(originMainTip) || item.Kind != "fix" || item.Priority != "critical" || item.Source != "main-red-stop" {
		t.Errorf("item = %+v, want the critical fix item of the stop", item)
	}
	text := item.Summary + " " + item.Fix + " " + strings.Join(item.Acceptance, " ")
	for _, job := range originMainFailedJobs {
		if !strings.Contains(text, job) {
			t.Errorf("fix item text %q does not name the failed job %q", text, job)
		}
	}
	if strings.Contains(item.Summary, "A lane found") {
		t.Errorf("summary = %q, want a CI cause because no lane found this red", item.Summary)
	}
}

func TestMainRedStop_CIOnlyRedOnAHeldStopOpensNothingAndFilesNoSecondItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := OpenMainCIRedStop(root, MainCIRed{Main: originMainTip, Failed: originMainFailedJobs[:1]}); err != nil {
		t.Fatal(err)
	}

	opened, err := OpenMainCIRedStop(root, MainCIRed{Main: "fedcba9876543210", Failed: originMainFailedJobs})

	if opened || err != nil {
		t.Fatalf("opened = %v err = %v, want the held stop to stay the one stop", opened, err)
	}
	stop, _, _ := mainstop.Read(filepath.Join(root, ".evolve"))
	if !slices.Equal(stop.Keys, ciKeysOf(originMainFailedJobs...)) {
		t.Errorf("keys = %v, want the held stop to gain the new failed job", stop.Keys)
	}
	if paths, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*.json")); len(paths) != 1 {
		t.Errorf("pending items = %v, want still one fix item", paths)
	}
}

func TestMainRedStop_AnEmptyFailedListOpensNoStop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	opened, err := OpenMainCIRedStop(root, MainCIRed{Main: originMainTip})

	if opened || err != nil {
		t.Fatalf("opened = %v err = %v, want a red with no failed job to open nothing", opened, err)
	}
	if _, found, _ := mainstop.Read(filepath.Join(root, ".evolve")); found {
		t.Error("a stop record exists for a CI verdict that names no failed job")
	}
}

func TestMainCIRedCode_NamesTheGateRepoIncident(t *testing.T) {
	t.Parallel()
	if string(CodeMainCIRed) != "GATE_REPO_MAIN_CI_RED" {
		t.Errorf("CodeMainCIRed = %q, want GATE_REPO_MAIN_CI_RED", CodeMainCIRed)
	}
}
