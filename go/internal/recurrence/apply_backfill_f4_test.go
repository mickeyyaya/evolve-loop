package recurrence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
)

func openItemF4(t *testing.T, inbox, id string, weight float64) string {
	t.Helper()
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inbox, id+".json")
	body, _ := json.Marshal(map[string]any{"id": id, "weight": weight, "action": "fix"})
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func optionsF4(inbox string) ApplyOptions {
	return ApplyOptions{InboxDir: inbox, Cycle: 5, Policy: DefaultEscalationPolicy(), Now: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)}
}

func TestApplyIntent_EscalationThatCannotBeWrittenIsAnErrorAndRecordsNoBump(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	path := openItemF4(t, inbox, "flaky", 0.1)
	if err := os.Chmod(inbox, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
	if f, err := os.CreateTemp(inbox, "probe"); err == nil {
		f.Close()
		t.Skip("directory permissions are not enforced for this user")
	}
	var res ApplyResult

	consumed, err := applyIntent(optionsF4(inbox), dispositionrouter.Intent{Action: dispositionrouter.ActionEscalate, ItemID: "flaky", Recurrence: 9, Weight: 1.0}, &res)

	if err == nil || consumed {
		t.Fatalf("consumed=%v err=%v", consumed, err)
	}
	if len(res.Bumped) != 0 {
		t.Errorf("an unwritten bump must not be reported: %+v", res)
	}
	if raw, _ := os.ReadFile(path); !strings.Contains(string(raw), `"weight":0.1`) {
		t.Errorf("the item must keep its weight: %s", raw)
	}
}

func TestAutofileIntent_AFilingFaultThatIsNotAMissingClassIsAnErrorNotARefusal(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "inbox-is-a-file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var res ApplyResult

	consumed, err := autofileIntent(optionsF4(blocker), dispositionrouter.Intent{
		Action: dispositionrouter.ActionAutofile, ItemID: "fix-x", Pattern: "p", Recurrence: 3, Weight: 2, PriorityClass: "correctness",
	}, &res)

	if err == nil || consumed {
		t.Fatalf("consumed=%v err=%v", consumed, err)
	}
	if len(res.Refused)+len(res.Filed)+len(res.Skipped) != 0 {
		t.Errorf("a filing fault is neither refused, filed nor skipped: %+v", res)
	}
}

func TestAutofileIntent_ClassifiesRefusedFiledAndAlreadyQueued(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	var res ApplyResult
	opts := optionsF4(inbox)

	if consumed, err := autofileIntent(opts, dispositionrouter.Intent{Action: dispositionrouter.ActionAutofile, ItemID: "no-class", Pattern: "p"}, &res); err != nil || !consumed {
		t.Fatalf("refusal: consumed=%v err=%v", consumed, err)
	}
	in := dispositionrouter.Intent{Action: dispositionrouter.ActionAutofile, ItemID: "good", Pattern: "p", Recurrence: 3, Weight: 2, PriorityClass: "correctness"}
	if consumed, err := autofileIntent(opts, in, &res); err != nil || !consumed {
		t.Fatalf("filed: consumed=%v err=%v", consumed, err)
	}
	if consumed, err := autofileIntent(opts, in, &res); err != nil || !consumed {
		t.Fatalf("already queued: consumed=%v err=%v", consumed, err)
	}

	if len(res.Refused) != 1 || res.Refused[0] != "no-class" || len(res.Filed) != 1 || res.Filed[0] != "good" || len(res.Skipped) != 1 || res.Skipped[0] != "good" {
		t.Fatalf("result = %+v", res)
	}
}
