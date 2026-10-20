package inboxbatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadItemFile(t *testing.T, body string) (Item, []string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return LoadFile(path)
}

func TestLoadFile_ANumericPriorityIsReadAsTheWeight(t *testing.T) {
	t.Parallel()
	it, notes, err := loadItemFile(t, `{"id":"persona-tail-curation-fleet","priority":0.86}`)
	if err != nil {
		t.Fatalf("a numeric priority must not drop the item: %v", err)
	}
	if it.Weight != 0.86 || it.Priority != "" {
		t.Errorf("weight %v priority %q, want the number as the weight", it.Weight, it.Priority)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "item.json: priority 0.86 is a number; it is read as the weight") {
		t.Errorf("notes %q, want one note that names the move", notes)
	}
}

func TestLoadFile_ANumericPriorityNeverOverridesAWeight(t *testing.T) {
	t.Parallel()
	it, notes, err := loadItemFile(t, `{"id":"a","weight":0.4,"priority":0.86}`)
	if err != nil || it.Weight != 0.4 || len(notes) != 1 || !strings.Contains(notes[0], "the weight 0.4 stays") {
		t.Errorf("weight %v notes %q err %v, want the declared weight kept and the number noted", it.Weight, notes, err)
	}
}

func TestLoadFile_AStringPriorityIsUnchanged(t *testing.T) {
	t.Parallel()
	it, notes, err := loadItemFile(t, `{"id":"a","weight":0.4,"priority":"P2"}`)
	if err != nil || it.Priority != "P2" || it.Weight != 0.4 || len(notes) != 0 {
		t.Errorf("priority %q weight %v notes %q err %v", it.Priority, it.Weight, notes, err)
	}
}

func TestLoadFile_APriorityOfAnotherTypeIsAnError(t *testing.T) {
	t.Parallel()
	if _, _, err := loadItemFile(t, `{"id":"a","priority":{"level":2}}`); err == nil || !strings.Contains(err.Error(), "priority") {
		t.Errorf("err %v, want an error that names the priority field", err)
	}
	if _, _, err := loadItemFile(t, `{"id":"a","weight":"high"}`); err == nil {
		t.Error("a malformed field outside priority must still fail the load")
	}
}

func TestLoadWarning_StringSaysSkippedOnlyForAnUnreadableFile(t *testing.T) {
	t.Parallel()
	if got := (LoadWarning{Text: "a.json: not JSON", Unreadable: true}).String(); got != "WARN skipped a.json: not JSON" {
		t.Errorf("unreadable = %q", got)
	}
	if got := (LoadWarning{Text: "b.json: sanitized title"}).String(); got != "WARN b.json: sanitized title" {
		t.Errorf("notice = %q", got)
	}
}
