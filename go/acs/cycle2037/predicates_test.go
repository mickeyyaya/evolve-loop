//go:build acs

package cycle2037

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var decisionTime = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func deliveredAgo(id string, age time.Duration) integration.Deliverable {
	return integration.Deliverable{ID: id, DeliveredAt: decisionTime.Add(-age)}
}

func ids(batch []integration.Deliverable) []string {
	out := make([]string, 0, len(batch))
	for _, d := range batch {
		out = append(out, d.ID)
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestC2037_001_EachD7TriggerFlushesTheBatchOnce(t *testing.T) {
	unblocking := deliveredAgo("unblocks", time.Minute)
	unblocking.UnblocksQueuedWork = true
	scenarios := []struct {
		name      string
		batch     []integration.Deliverable
		waveEnded bool
		trigger   integration.FlushTrigger
	}{
		{"wave end", []integration.Deliverable{deliveredAgo("a", time.Minute)}, true, integration.TriggerWaveEnd},
		{"cap reached", []integration.Deliverable{deliveredAgo("a", time.Minute), deliveredAgo("b", time.Minute), deliveredAgo("c", time.Minute), deliveredAgo("d", time.Minute)}, false, integration.TriggerCapReached},
		{"max age", []integration.Deliverable{deliveredAgo("a", time.Minute), deliveredAgo("old", 45*time.Minute)}, false, integration.TriggerMaxAge},
		{"unblocks queued work", []integration.Deliverable{deliveredAgo("a", time.Minute), unblocking}, false, integration.TriggerUnblocksQueuedWork},
	}
	for _, s := range scenarios {
		decision := integration.DecideFlush(s.batch, s.waveEnded, integration.DefaultFlushPolicy(), decisionTime)
		if decision.Trigger != s.trigger {
			t.Errorf("%s: trigger = %q, want %q", s.name, decision.Trigger, s.trigger)
		}
		if !slices.Equal(ids(decision.Flush), ids(s.batch)) {
			t.Errorf("%s: flushed %v, want the whole batch %v", s.name, ids(decision.Flush), ids(s.batch))
		}
		if len(decision.Hold) != 0 {
			t.Errorf("%s: held %v, want nothing held after a flush", s.name, ids(decision.Hold))
		}
		again := integration.DecideFlush(decision.Hold, s.waveEnded, integration.DefaultFlushPolicy(), decisionTime)
		if again.Trigger != integration.TriggerNone || len(again.Flush) != 0 {
			t.Errorf("%s: a second decision on the held remainder flushed %v, want a single flush", s.name, ids(again.Flush))
		}
	}
}

func TestC2037_002_ARiskyDeliverableFlushesAlone(t *testing.T) {
	risky := deliveredAgo("risky", time.Minute)
	risky.Risky = true
	batch := []integration.Deliverable{deliveredAgo("a", 90*time.Minute), risky, deliveredAgo("c", time.Minute), deliveredAgo("d", time.Minute)}
	decision := integration.DecideFlush(batch, true, integration.DefaultFlushPolicy(), decisionTime)
	if decision.Trigger != integration.TriggerRiskyAlone {
		t.Errorf("trigger = %q, want %q", decision.Trigger, integration.TriggerRiskyAlone)
	}
	if !slices.Equal(ids(decision.Flush), []string{"risky"}) {
		t.Errorf("flushed %v, want only the risky deliverable", ids(decision.Flush))
	}
	if !slices.Equal(ids(decision.Hold), []string{"a", "c", "d"}) {
		t.Errorf("held %v, want the other three in order", ids(decision.Hold))
	}
}

func TestC2037_003_TheCapHalvesOnRedAndGrowsOnGreenInsideOneToFour(t *testing.T) {
	halved := map[int]int{4: 2, 3: 1, 2: 1, 1: 1}
	grown := map[int]int{1: 2, 2: 3, 3: 4, 4: 4}
	for from, want := range halved {
		if got := integration.HalveCap(from); got != want {
			t.Errorf("HalveCap(%d) = %d, want %d", from, got, want)
		}
	}
	for from, want := range grown {
		if got := integration.GrowCap(from); got != want {
			t.Errorf("GrowCap(%d) = %d, want %d", from, got, want)
		}
	}
	for _, outOfRange := range []int{-5, 0, 9, 100} {
		for name, got := range map[string]int{"HalveCap": integration.HalveCap(outOfRange), "GrowCap": integration.GrowCap(outOfRange)} {
			if got < 1 || got > 4 {
				t.Errorf("%s(%d) = %d, want a cap inside 1 to 4", name, outOfRange, got)
			}
		}
	}
}

func TestC2037_004_TheDefaultsAreACapOfFourAndMaxAgeMinutesOfFortyFive(t *testing.T) {
	policy := integration.DefaultFlushPolicy()
	if policy.Cap != 4 {
		t.Errorf("default cap = %d, want 4", policy.Cap)
	}
	if policy.MaxAgeMinutes != 45 {
		t.Errorf("default max_age_minutes = %d, want 45", policy.MaxAgeMinutes)
	}
	justUnderDefaultAge := []integration.Deliverable{deliveredAgo("a", 44*time.Minute+59*time.Second)}
	if got := integration.DecideFlush(justUnderDefaultAge, false, policy, decisionTime); got.Trigger != integration.TriggerNone {
		t.Errorf("44m59s old batch flushed with %q, want no flush under the 45 minute default", got.Trigger)
	}
	threeDeliverables := []integration.Deliverable{deliveredAgo("a", time.Minute), deliveredAgo("b", time.Minute), deliveredAgo("c", time.Minute)}
	if got := integration.DecideFlush(threeDeliverables, false, policy, decisionTime); got.Trigger != integration.TriggerNone {
		t.Errorf("3 deliverables flushed with %q, want no flush under the default cap of 4", got.Trigger)
	}
}

func TestC2037_005_ABatchWithNoTriggerAndAnEmptyBatchDoNotFlush(t *testing.T) {
	young := []integration.Deliverable{deliveredAgo("a", time.Minute), deliveredAgo("b", 2*time.Minute)}
	held := integration.DecideFlush(young, false, integration.DefaultFlushPolicy(), decisionTime)
	if held.Trigger != integration.TriggerNone || len(held.Flush) != 0 {
		t.Errorf("under-cap young batch decision = %+v, want no flush", held)
	}
	if !slices.Equal(ids(held.Hold), []string{"a", "b"}) {
		t.Errorf("held %v, want the whole batch held", ids(held.Hold))
	}
	empty := integration.DecideFlush(nil, true, integration.DefaultFlushPolicy(), decisionTime)
	if empty.Trigger != integration.TriggerNone || len(empty.Flush) != 0 {
		t.Errorf("empty batch at wave end decision = %+v, want no flush", empty)
	}
}

// acs-predicate: config-check
func TestC2037_006_TheNewPackageIsEnrolledInTheApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enrolment := filepath.Join(root, "go", ".apicover-enforce")
	lines := strings.Split(readFile(t, enrolment), "\n")
	if !slices.Contains(lines, "./internal/integration") {
		t.Errorf("RED: go/.apicover-enforce has no exact line ./internal/integration")
	}
	named := filepath.Join(root, "go", "internal", "integration", "apicover_named_test.go")
	if !acsassert.FileExists(t, named) {
		t.Errorf("RED: %s missing", named)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", "go/internal/integration/flush.go"); code != 0 {
		t.Errorf("RED: go/internal/integration/flush.go is not git-tracked")
	}
}
