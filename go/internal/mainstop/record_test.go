package mainstop

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

func TestRead_AMissingRecordIsNoStopAndABrokenRecordIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if stop, found, err := Read(dir); err != nil || found || stop.Holds() {
		t.Errorf("Read of no record = %+v, %v, %v, want no stop and no error", stop, found, err)
	}
	if err := os.WriteFile(Path(dir), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, found, err := Read(dir); err == nil || found || !strings.Contains(err.Error(), "parse") {
		t.Errorf("Read of a broken record = %v, %v, want a parse error", found, err)
	}
	if _, err := Update(dir, func(s Stop, _ bool) (Stop, error) { return s, nil }); err == nil {
		t.Error("Update over a broken record = nil error, want the parse error")
	}
	if err := os.Remove(Path(dir)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(Path(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(dir); err == nil || !strings.Contains(err.Error(), "read") {
		t.Errorf("Read of a directory = %v, want a read error", err)
	}
}

func TestUpdate_AFailedWriteIsAnErrorAndGivesTheStopOnDisk(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores the read-only directory")
	}
	dir := t.TempDir()
	if _, err := Update(dir, func(Stop, bool) (Stop, error) { return openStop(dupKey), nil }); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("refused")
	stop, err := Update(dir, func(Stop, bool) (Stop, error) { return Stop{}, cause })
	if !errors.Is(err, cause) || !slices.Equal(stop.Keys, []baseattr.Key{dupKey}) {
		t.Errorf("Update with a failed transition = %+v, %v, want the error and the stop on disk", stop, err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	stop, err = Update(dir, func(s Stop, _ bool) (Stop, error) {
		return s.Released([]baseattr.Key{dupKey}, "cycle 7", opened), nil
	})
	if err == nil || !strings.Contains(err.Error(), "write") || !slices.Equal(stop.Keys, []baseattr.Key{dupKey}) {
		t.Errorf("Update in a read-only directory = %+v, %v, want a write error and the stop on disk", stop, err)
	}
}

func TestRead_RefusesARecordOfAnotherSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	record := `{"schema":"main-red-stop/v2","keys":[{"package":"internal/dup","test":"TestCount"}]}`
	if err := os.WriteFile(Path(dir), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	if stop, found, err := Read(dir); err == nil || found || stop.Holds() || !strings.Contains(err.Error(), "main-red-stop/v2") {
		t.Errorf("Read of a v2 record = %+v, %v, %v, want an error that names the schema", stop, found, err)
	}
}

func TestUpdate_StampsTheSchemaAndWritesTheKeysInSnakeCase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	unstamped := Stop{Keys: []baseattr.Key{dupKey}, MainSHA: mainSHA, OpenedAt: opened}
	if _, err := Update(dir, func(Stop, bool) (Stop, error) { return unstamped, nil }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path(dir))
	if err != nil || !strings.Contains(string(data), `"package": "internal/dup"`) || !strings.Contains(string(data), `"test": "TestCount"`) {
		t.Errorf("record = %s, %v, want the key in snake case", data, err)
	}
	if stop, found, err := Read(dir); err != nil || !found || stop.Schema != Schema || !slices.Equal(stop.Keys, []baseattr.Key{dupKey}) {
		t.Errorf("Read = %+v, %v, %v, want the stamped stop with its key", stop, found, err)
	}
}

func TestUpdate_WritesTheNextStopAndReadGivesItBack(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".evolve")
	wrote, err := Update(dir, func(s Stop, found bool) (Stop, error) {
		if found || s.Holds() {
			t.Errorf("the first update saw %+v, %v, want no stop", s, found)
		}
		return openStop(dupKey).WithParked(Parked{Cycle: 2249, Scope: []string{"go/internal/dup"}, Workspace: "ws"}), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	read, found, err := Read(dir)
	if err != nil || !found || !read.Holds() || !slices.Equal(read.Keys, wrote.Keys) || !read.OpenedAt.Equal(opened) || read.Parked[0].Workspace != "ws" {
		t.Errorf("Read = %+v, %v, %v, want the stop that Update wrote", read, found, err)
	}
	data, err := os.ReadFile(Path(dir))
	if err != nil || !strings.Contains(string(data), `"main_sha": "`+mainSHA+`"`) || strings.Contains(string(data), "released_at") {
		t.Errorf("record = %s, %v, want the main commit and no release field", data, err)
	}
}

func TestUpdate_AFailedTransitionWritesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cause := errors.New("refused")
	if _, err := Update(dir, func(Stop, bool) (Stop, error) { return openStop(dupKey), cause }); !errors.Is(err, cause) {
		t.Errorf("Update = %v, want the error of the transition", err)
	}
	if _, found, _ := Read(dir); found {
		t.Error("a failed transition wrote a record")
	}
	blocked := filepath.Join(dir, "file")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(blocked, func(s Stop, _ bool) (Stop, error) { return s, nil }); err == nil {
		t.Error("Update under a file = nil error, want a lock error")
	}
}

func TestUpdate_ConcurrentUpdatesLoseNoKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keys := make([]baseattr.Key, 16)
	var wg sync.WaitGroup
	for i := range keys {
		keys[i] = baseattr.Key{Package: "internal/p", Test: "Test" + string(rune('A'+i))}
		wg.Add(1)
		go func(k baseattr.Key) {
			defer wg.Done()
			if _, err := Update(dir, func(s Stop, _ bool) (Stop, error) {
				return s.Opened(baseattr.Verdict{MainRed: []baseattr.Key{k}, Main: mainSHA}, "item", opened), nil
			}); err != nil {
				t.Error(err)
			}
		}(keys[i])
	}
	wg.Wait()
	stop, _, err := Read(dir)
	if err != nil || len(stop.Keys) != len(keys) {
		t.Errorf("after %d concurrent updates the stop has %d keys, %v", len(keys), len(stop.Keys), err)
	}
}
