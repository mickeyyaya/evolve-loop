package continuation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func regFixture(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func sampleContinuation(cycle int, sha string) Continuation {
	return Continuation{
		Worktree:     "/wt/cycle-" + sha,
		Branch:       "evolve/cycle-1078",
		SnapshotSHA:  sha,
		BaseSHA:      "base0000000000000000000000000000000000000",
		FindingsPath: "/ws/failure-digest.json",
		Cycle:        cycle,
	}
}

func TestRegistry_RoundTripByScopeID(t *testing.T) {
	root := regFixture(t)
	want := sampleContinuation(1078, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	if err := WriteRegistryEntry(root, "chain-boundary-loop", want); err != nil {
		t.Fatalf("WriteRegistryEntry: %v", err)
	}
	got, ok, err := ReadRegistryEntry(root, "chain-boundary-loop")
	if err != nil || !ok {
		t.Fatalf("ReadRegistryEntry: ok=%v err=%v", ok, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("registry round-trip lost fields:\n got %+v\nwant %+v", got, want)
	}
	if _, err := os.Stat(RegistryPath(root)); err != nil {
		t.Errorf("registry must exist at RegistryPath(%q) = %q: %v", root, RegistryPath(root), err)
	}
}

func TestRegistry_SecondScopeDoesNotClobberFirst(t *testing.T) {
	root := regFixture(t)
	first := sampleContinuation(1078, "1111111111111111111111111111111111111111")
	second := sampleContinuation(1079, "2222222222222222222222222222222222222222")

	if err := WriteRegistryEntry(root, "scope-a", first); err != nil {
		t.Fatalf("write scope-a: %v", err)
	}
	if err := WriteRegistryEntry(root, "scope-b", second); err != nil {
		t.Fatalf("write scope-b: %v", err)
	}

	a, ok, err := ReadRegistryEntry(root, "scope-a")
	if err != nil || !ok {
		t.Fatalf("scope-a lost after writing scope-b: ok=%v err=%v", ok, err)
	}
	if a.SnapshotSHA != first.SnapshotSHA {
		t.Errorf("scope-a snapshot = %q, want %q (a second lane's write clobbered it)", a.SnapshotSHA, first.SnapshotSHA)
	}
	b, ok, err := ReadRegistryEntry(root, "scope-b")
	if err != nil || !ok || b.SnapshotSHA != second.SnapshotSHA {
		t.Errorf("scope-b = %+v ok=%v err=%v, want snapshot %q", b, ok, err, second.SnapshotSHA)
	}

	third := sampleContinuation(1090, "3333333333333333333333333333333333333333")
	if err := WriteRegistryEntry(root, "scope-a", third); err != nil {
		t.Fatalf("rewrite scope-a: %v", err)
	}
	a2, _, _ := ReadRegistryEntry(root, "scope-a")
	if a2.SnapshotSHA != third.SnapshotSHA || a2.Cycle != 1090 {
		t.Errorf("re-stamp must replace the scope's own entry, got %+v", a2)
	}
	if b2, ok, _ := ReadRegistryEntry(root, "scope-b"); !ok || b2.SnapshotSHA != second.SnapshotSHA {
		t.Errorf("re-stamping scope-a must not disturb scope-b, got %+v ok=%v", b2, ok)
	}
}

func TestRegistry_MissingFileAndUnknownScopeAreCleanMiss(t *testing.T) {
	root := regFixture(t)

	got, ok, err := ReadRegistryEntry(root, "never-stamped")
	if err != nil || ok || got != (Continuation{}) {
		t.Errorf("absent registry must be a clean miss, got %+v ok=%v err=%v", got, ok, err)
	}

	if err := WriteRegistryEntry(root, "scope-a", sampleContinuation(1078, "abc")); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, ok, err = ReadRegistryEntry(root, "some-other-scope")
	if err != nil || ok || got != (Continuation{}) {
		t.Errorf("unknown scope in a present registry must be a clean miss, got %+v ok=%v err=%v", got, ok, err)
	}
	if _, ok, _ := ReadRegistryEntry(root, ""); ok {
		t.Error("empty scope id must never resolve a binding")
	}
}

func TestRegistry_CorruptFileIsLoudError(t *testing.T) {
	root := regFixture(t)
	if err := os.MkdirAll(filepath.Dir(RegistryPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RegistryPath(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := ReadRegistryEntry(root, "scope-a"); err == nil {
		t.Errorf("corrupt registry must be a LOUD error, got ok=%v err=nil", ok)
	}
	err := WriteRegistryEntry(root, "scope-a", sampleContinuation(1078, "abc"))
	if err == nil {
		if _, ok, rerr := ReadRegistryEntry(root, "scope-a"); rerr != nil || !ok {
			t.Errorf("WriteRegistryEntry reported success over a corrupt registry but the "+
				"entry is not readable afterwards: ok=%v err=%v", ok, rerr)
		}
	}
}

func TestRegistry_StoresTheContinuationSchemaVerbatim(t *testing.T) {
	root := regFixture(t)
	want := sampleContinuation(1078, "4444444444444444444444444444444444444444")
	if err := WriteRegistryEntry(root, "scope-a", want); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, err := os.ReadFile(RegistryPath(root))
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	var byScope map[string]Continuation
	if err := json.Unmarshal(raw, &byScope); err != nil {
		t.Fatalf("registry must be a scope-id → Continuation map: %v\n%s", err, raw)
	}
	if !reflect.DeepEqual(byScope["scope-a"], want) {
		t.Errorf("registry entry != Continuation verbatim:\n got %+v\nwant %+v", byScope["scope-a"], want)
	}

	ws := t.TempDir()
	if err := WriteManifest(ws, want); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	manifestRaw, err := os.ReadFile(filepath.Join(ws, "continuation-manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifestKeys, entryKeys map[string]json.RawMessage
	if err := json.Unmarshal(manifestRaw, &manifestKeys); err != nil {
		t.Fatal(err)
	}
	var rawByScope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawByScope); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rawByScope["scope-a"], &entryKeys); err != nil {
		t.Fatalf("registry entry is not a JSON object: %v", err)
	}
	for k := range manifestKeys {
		if _, ok := entryKeys[k]; !ok {
			t.Errorf("registry entry is missing manifest field %q — schema fork", k)
		}
	}
	for k := range entryKeys {
		if _, ok := manifestKeys[k]; !ok {
			t.Errorf("registry entry carries field %q the manifest does not — schema fork", k)
		}
	}
}

func TestRegistry_ConcurrentLaneWritesDoNotLoseBindings(t *testing.T) {
	root := regFixture(t)
	const lanes = 8

	var wg sync.WaitGroup
	errs := make([]error, lanes)
	for i := 0; i < lanes; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sha := fmt.Sprintf("%040d", i)
			errs[i] = WriteRegistryEntry(root, fmt.Sprintf("lane-%d", i), sampleContinuation(1100+i, sha))
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("lane %d WriteRegistryEntry: %v", i, err)
		}
	}
	for i := 0; i < lanes; i++ {
		c, ok, err := ReadRegistryEntry(root, fmt.Sprintf("lane-%d", i))
		if err != nil {
			t.Fatalf("lane %d ReadRegistryEntry: %v", i, err)
		}
		if !ok {
			t.Errorf("lane %d binding was lost — concurrent writes clobbered it", i)
			continue
		}
		if want := fmt.Sprintf("%040d", i); c.SnapshotSHA != want {
			t.Errorf("lane %d snapshot = %q, want %q", i, c.SnapshotSHA, want)
		}
	}
}
