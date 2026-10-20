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

func TestLoadFile_ANumericPriorityOutsideTheWeightRangeIsNotAWeight(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"2", "-0.5", "1.5"} {
		it, notes, err := loadItemFile(t, `{"id":"p2-meant","priority":`+raw+`}`)
		if err != nil {
			t.Fatalf("priority %s: a number out of range must not drop the item: %v", raw, err)
		}
		if it.Weight != 0 || it.Priority != "" {
			t.Errorf("priority %s: weight %v priority %q, want no weight read from an out-of-range number", raw, it.Weight, it.Priority)
		}
		want := "item.json: priority " + raw + ` of item "p2-meant" is outside the weight range 0..1, so it is not read as the weight`
		if len(notes) != 1 || notes[0] != want {
			t.Errorf("priority %s: notes %q, want %q", raw, notes, want)
		}
	}
	if it, notes, err := loadItemFile(t, `{"id":"edge","priority":1}`); err != nil || it.Weight != 1 || len(notes) != 1 || !strings.Contains(notes[0], "read as the weight") {
		t.Errorf("priority 1 is inside the range: weight %v notes %q err %v, want weight 1 and the read note", it.Weight, notes, err)
	}
}
