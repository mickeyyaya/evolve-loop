package failurelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPruneExpiredCarryoverTodos_ZeroClockUsesTheRealNowAndNonObjectEntriesSurvive(t *testing.T) {
	dir := t.TempDir()
	past := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	future := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	state := filepath.Join(dir, "state.json")
	body := `{"carryoverTodos":["a bare string",7,null,{"id":"old","expiresAt":"` + past + `"},{"id":"fresh","expiresAt":"` + future + `"},{"id":"no-expiry"}],"other":1}`
	if err := os.WriteFile(state, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := PruneExpiredCarryoverTodos(state, time.Time{})

	if err != nil || res.Before != 6 || res.After != 5 || res.Removed != 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	after, _ := os.ReadFile(state)
	for _, want := range []string{`"a bare string"`, `"fresh"`, `"no-expiry"`, `"other"`} {
		if !strings.Contains(string(after), want) {
			t.Errorf("state lost %s: %s", want, after)
		}
	}
	if strings.Contains(string(after), `"old"`) {
		t.Errorf("the expired entry survived: %s", after)
	}
}

func TestPruneExpiredCarryoverTodos_MalformedStateIsReportedNotPruned(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(state, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := PruneExpiredCarryoverTodos(state, time.Now())

	if err == nil || !strings.Contains(err.Error(), "failurelog: parse state") || res != (PruneResult{}) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if got, _ := os.ReadFile(state); string(got) != "{broken" {
		t.Errorf("a malformed state file must be left as found: %s", got)
	}
}
