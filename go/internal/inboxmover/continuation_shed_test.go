package inboxmover

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func shedFixture(t *testing.T) (Options, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	claim := filepath.Join(inbox, "processing", "cycle-2020")
	if err := os.MkdirAll(claim, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"retro.json": `{"id":"retro","continuation":{"snapshot_sha":"d2f06f24","cycle":2004}}`,
		"other.json": `{"id":"other","continuation":{"snapshot_sha":"feedface","cycle":2011}}`,
		"plain.json": `{"id":"plain"}`,
		"bare.json":  `{"id":"bare","continuation":{"cycle":3}}`,
	} {
		if err := os.WriteFile(filepath.Join(claim, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Options{ProjectRoot: root, Stderr: io.Discard}, claim
}

func itemFields(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestShedDeclinedContinuation_ClearsThePointerOfTheDeclinedSnapshotOnly(t *testing.T) {
	opts, claim := shedFixture(t)

	shed, err := ShedDeclinedContinuation(opts, 2020, continuation.Continuation{SnapshotSHA: "d2f06f24", Cycle: 2004})

	if err != nil || len(shed) != 1 || shed[0] != "retro" {
		t.Fatalf("shed = %v, err = %v; want [retro]", shed, err)
	}
	retro := itemFields(t, filepath.Join(claim, "retro.json"))
	if _, kept := retro["continuation"]; kept {
		t.Errorf("a rejected adoption must leave no continuation block on the item: %s", retro["continuation"])
	}
	if released := string(retro["released_continuations"]); !strings.Contains(released, "d2f06f24") || !strings.Contains(released, "declined in cycle 2020") {
		t.Errorf("the shed pointer is preserved with its reason: %s", released)
	}
	if other := itemFields(t, filepath.Join(claim, "other.json")); !strings.Contains(string(other["continuation"]), "feedface") {
		t.Errorf("a pointer to another snapshot must stay: %s", other["continuation"])
	}
}

func TestShedDeclinedContinuation_AnEmptyOrMissingClaimShedsNothing(t *testing.T) {
	opts, claim := shedFixture(t)

	if shed, err := ShedDeclinedContinuation(opts, 2021, continuation.Continuation{SnapshotSHA: "d2f06f24"}); err != nil || len(shed) != 0 {
		t.Errorf("a cycle with no claim dir: shed = %v, err = %v", shed, err)
	}
	if shed, err := ShedDeclinedContinuation(opts, 2020, continuation.Continuation{}); err != nil || len(shed) != 0 {
		t.Errorf("an empty snapshot names no pointer: shed = %v, err = %v", shed, err)
	}
	if bare := itemFields(t, filepath.Join(claim, "bare.json")); !strings.Contains(string(bare["continuation"]), `"cycle":3`) {
		t.Errorf("a stamp with no snapshot is kept by an empty declined snapshot: %s", bare["continuation"])
	}
}

func TestShedDeclinedContinuation_ARewriteFaultIsReturned(t *testing.T) {
	opts, claim := shedFixture(t)
	if err := os.Mkdir(filepath.Join(claim, "retro.json.tmp."+strconv.Itoa(os.Getpid())), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := ShedDeclinedContinuation(opts, 2020, continuation.Continuation{SnapshotSHA: "d2f06f24", Cycle: 2004})

	if err == nil || !strings.Contains(err.Error(), "retro.json") {
		t.Errorf("err = %v; a pointer that cannot be shed must be an error naming the item", err)
	}
}

func TestShedDeclinedContinuation_AnUnreadableClaimOrItemIsAnError(t *testing.T) {
	opts, claim := shedFixture(t)
	if err := os.WriteFile(filepath.Join(opts.ProjectRoot, ".evolve", "inbox", "processing", "cycle-2022"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ShedDeclinedContinuation(opts, 2022, continuation.Continuation{SnapshotSHA: "d2f06f24"}); err == nil || !strings.Contains(err.Error(), "cycle-2022") {
		t.Errorf("a claim path that is not a directory: err = %v", err)
	}
	locked := filepath.Join(claim, "retro.json")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	if _, err := ShedDeclinedContinuation(opts, 2020, continuation.Continuation{SnapshotSHA: "d2f06f24"}); err == nil || !strings.Contains(err.Error(), "retro.json") {
		t.Errorf("an unreadable item: err = %v", err)
	}
}

func TestShedDeclinedContinuation_AMarshalFaultShedsNothing(t *testing.T) {
	opts, claim := shedFixture(t)
	orig := marshalReleasedContinuation
	t.Cleanup(func() { marshalReleasedContinuation = orig })
	marshalReleasedContinuation = func(releasedContinuation) ([]byte, error) { return nil, os.ErrInvalid }

	shed, err := ShedDeclinedContinuation(opts, 2020, continuation.Continuation{SnapshotSHA: "d2f06f24"})

	if err == nil || len(shed) != 0 {
		t.Errorf("shed = %v, err = %v; a pointer that cannot be preserved is not shed", shed, err)
	}
	if body, _ := os.ReadFile(filepath.Join(claim, "retro.json")); !strings.Contains(string(body), "d2f06f24") {
		t.Errorf("the item keeps its pointer: %s", body)
	}
}

func TestReleaseContinuationBinding_ClearsTheMatchingPointerOnTheItem(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	item := filepath.Join(inbox, "old-item.json")
	if err := os.WriteFile(item, []byte(`{"id":"old-item","continuation":{"snapshot_sha":"0ld","cycle":1127}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteRegistryEntry(root, "old-item", continuation.Continuation{SnapshotSHA: "0ld", Cycle: 1127}); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: io.Discard}

	if _, released, err := ReleaseContinuationBinding(opts, "old-item", "older than 100 cycles", "test"); err != nil || !released {
		t.Fatalf("released = %v, err = %v", released, err)
	}

	fields := itemFields(t, item)
	if _, kept := fields["continuation"]; kept {
		t.Errorf("a released binding must not stay adoptable through the item: %s", fields["continuation"])
	}
	if got := ResolveContinuationForScope(opts, 3000, []string{"old-item"}); got != nil {
		t.Errorf("the next claim resolves %+v; a released binding must not resume", got)
	}
}

func TestReleaseContinuationBinding_KeepsAPointerToAnotherSnapshot(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	item := filepath.Join(inbox, "rebound.json")
	if err := os.WriteFile(item, []byte(`{"id":"rebound","continuation":{"snapshot_sha":"n3w","cycle":2050}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteRegistryEntry(root, "rebound", continuation.Continuation{SnapshotSHA: "0ld", Cycle: 1127}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := ReleaseContinuationBinding(Options{ProjectRoot: root, Stderr: io.Discard}, "rebound", "r", "test"); err != nil {
		t.Fatal(err)
	}

	if fields := itemFields(t, item); !strings.Contains(string(fields["continuation"]), "n3w") {
		t.Errorf("a newer pointer on the item stays: %s", fields["continuation"])
	}
}

func TestShedDeclinedContinuation_RedactsTheHomeInThePreservedPointer(t *testing.T) {
	opts, claim := shedFixture(t)
	t.Setenv("HOME", opts.ProjectRoot)
	wt := filepath.Join(opts.ProjectRoot, "worktrees", "cycle-2004")
	body := `{"id":"homed","continuation":{"snapshot_sha":"h0me","worktree":"` + wt + `","cycle":2004}}`
	if err := os.WriteFile(filepath.Join(claim, "homed.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ShedDeclinedContinuation(opts, 2020, continuation.Continuation{SnapshotSHA: "h0me"}); err != nil {
		t.Fatal(err)
	}

	if released := string(itemFields(t, filepath.Join(claim, "homed.json"))["released_continuations"]); !strings.Contains(released, `"worktree":"~/worktrees/cycle-2004"`) {
		t.Errorf("the preserved pointer must carry ~, never the host home: %s", released)
	}
}

func TestReleaseContinuationBinding_AnEmptyReleasedSnapshotKeepsTheLiveBlock(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	item := filepath.Join(inbox, "bare.json")
	if err := os.WriteFile(item, []byte(`{"id":"bare","continuation":{"cycle":7}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteRegistryEntry(root, "bare", continuation.Continuation{Cycle: 7}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := ReleaseContinuationBinding(Options{ProjectRoot: root, Stderr: io.Discard}, "bare", "r", "test"); err != nil {
		t.Fatal(err)
	}

	if fields := itemFields(t, item); !strings.Contains(string(fields["continuation"]), `"cycle":7`) {
		t.Errorf("a release with no snapshot names no live block: %s", fields["continuation"])
	}
}
