//go:build acs

package cycle2667

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dashboard"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func writeItems(t *testing.T, root, state string, n int) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox", state)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%s-%d", state, i)
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(`{"id":"`+id+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func queueCounts(t *testing.T, root string) map[string]json.RawMessage {
	t.Helper()
	body, err := json.Marshal(dashboard.Collect(root, time.Now()).Queue)
	if err != nil {
		t.Fatal(err)
	}
	var counts map[string]json.RawMessage
	if err := json.Unmarshal(body, &counts); err != nil {
		t.Fatal(err)
	}
	return counts
}

func countOf(t *testing.T, counts map[string]json.RawMessage, state string) int {
	t.Helper()
	raw, ok := counts[state]
	if !ok {
		t.Errorf("queue view has no count for state %q", state)
		return -1
	}
	n, err := strconv.Atoi(string(raw))
	if err != nil {
		t.Errorf("queue count for state %q is not a number: %s", state, raw)
		return -1
	}
	return n
}

func TestC2667_001_one_delivered_item_shows_delivered_count_of_one(t *testing.T) {
	root := t.TempDir()
	writeItems(t, root, "delivered", 1)
	if got := countOf(t, queueCounts(t, root), inboxmover.StateDelivered); got != 1 {
		t.Errorf("delivered count = %d, want 1", got)
	}
}

func TestC2667_002_delivered_count_ignores_items_of_other_states(t *testing.T) {
	root := t.TempDir()
	writeItems(t, root, "processed", 2)
	writeItems(t, root, "consumed", 1)
	if got := countOf(t, queueCounts(t, root), inboxmover.StateDelivered); got != 0 {
		t.Errorf("delivered count = %d with no delivered item, want 0", got)
	}
}

func TestC2667_003_lifecycle_state_list_is_the_directory_backed_states_once(t *testing.T) {
	states := inboxmover.LifecycleStates()
	want := []string{
		inboxmover.StateProcessing, inboxmover.StateProcessed, inboxmover.StateConsumed,
		inboxmover.StateRetry, inboxmover.StateDelivered, inboxmover.StateRejected, inboxmover.StateQuarantine,
	}
	for _, s := range want {
		if !slices.Contains(states, s) {
			t.Errorf("LifecycleStates() = %v, missing %q", states, s)
		}
	}
	for _, s := range []string{inboxmover.StatePending, inboxmover.StateUnknown} {
		if slices.Contains(states, s) {
			t.Errorf("LifecycleStates() = %v carries %q, which has no state directory", states, s)
		}
	}
	if len(states) != len(want) {
		t.Errorf("LifecycleStates() = %v, want exactly %v", states, want)
	}
	states[0] = "mutated"
	if slices.Contains(inboxmover.LifecycleStates(), "mutated") {
		t.Errorf("LifecycleStates() returns its backing slice; a caller mutation leaks into the next call")
	}
}

func TestC2667_004_every_listed_state_has_its_own_queue_count(t *testing.T) {
	root := t.TempDir()
	states := inboxmover.LifecycleStates()
	if len(states) == 0 {
		t.Fatal("LifecycleStates() is empty")
	}
	for i, s := range states {
		writeItems(t, root, s, i+1)
	}
	counts := queueCounts(t, root)
	for i, s := range states {
		if got := countOf(t, counts, s); got != i+1 {
			t.Errorf("queue count for state %q = %d, want %d", s, got, i+1)
		}
	}
}

func TestC2667_005_non_json_files_in_a_state_directory_are_not_counted(t *testing.T) {
	root := t.TempDir()
	writeItems(t, root, "delivered", 1)
	notes := filepath.Join(root, ".evolve", "inbox", "delivered", "notes.md")
	if err := os.WriteFile(notes, []byte("not an item"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := countOf(t, queueCounts(t, root), inboxmover.StateDelivered); got != 1 {
		t.Errorf("delivered count = %d with one item and one note, want 1", got)
	}
}

func coverageByFunc(t *testing.T, pkg string) map[string]string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	goDir := filepath.Join(root, "go")
	if _, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-coverprofile="+profile, pkg); err != nil || code != 0 {
		t.Fatalf("go test %s exited %d (err=%v)\n%s", pkg, code, err, errOut)
	}
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover exited %d: %v", code, err)
	}
	byFunc := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		file := strings.SplitN(fields[0], ":", 2)[0]
		byFunc[filepath.Base(file)+" "+fields[1]] = fields[len(fields)-1]
	}
	return byFunc
}

func TestC2667_006_changed_functions_have_full_coverage(t *testing.T) {
	dash := coverageByFunc(t, "./internal/dashboard")
	for _, fn := range []string{"readQueue", "countJSON", "inboxDir"} {
		if got := dash["queue.go "+fn]; got != "100.0%" {
			t.Errorf("dashboard queue.go %s coverage = %q, want 100.0%%", fn, got)
		}
	}
	mover := coverageByFunc(t, "./internal/inboxmover")
	var found bool
	for key, got := range mover {
		if strings.HasSuffix(key, " LifecycleStates") {
			found = true
			if got != "100.0%" {
				t.Errorf("inboxmover %s coverage = %q, want 100.0%%", key, got)
			}
		}
	}
	if !found {
		t.Errorf("inboxmover.LifecycleStates is not in the coverage profile")
	}
}
