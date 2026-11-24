package lifecycle

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const repairItem = `{"id":"repair-inc-1","kind":"pipeline-repair","priority_class":"debuggability","weight":0.4,` +
	`"title":"repair incident inc-1","files":["go/internal/gc/sweep.go"],` +
	`"summary":"The incident inc-1 repeats.","fix":"Repair it.","acceptance":["a red test reproduces inc-1"]}`

func repairFiler(t *testing.T) (*Mover, string, *recordingAppender) {
	t.Helper()
	inbox := newInbox(t)
	rec := &recordingAppender{}
	return newFiler(inbox, rec), inbox, rec
}

func TestMover_FileRepair_StampsTheTierTheProtectedFilesAndAConsoleRoute(t *testing.T) {
	t.Parallel()
	m, _, rec := repairFiler(t)

	res, err := m.FileRepair([]byte(repairItem), RepairStamp{Tier: "T1", ProtectedFiles: []string{"go/internal/guards/role.go"}})

	if err != nil {
		t.Fatalf("FileRepair: %v", err)
	}
	doc := readItem(t, res.Path)
	protected, _ := doc["protected_files"].([]any)
	if doc["tier"] != "T1" || len(protected) != 1 || protected[0] != "go/internal/guards/role.go" || doc["route"] != "console" {
		t.Errorf("stamps = tier %v protected %v route %v", doc["tier"], protected, doc["route"])
	}
	item, _, err := inboxbatch.LoadFile(res.Path)
	if err != nil || item.RepairTier() != "T0" {
		t.Errorf("the protected file binds T0 on read: %q %v", item.RepairTier(), err)
	}
	if routed, _ := inboxbatch.ConsoleRouted(item, nil); !routed {
		t.Error("a filed repair item is console-routed")
	}
	if len(rec.records) != 1 || rec.records[0].Action != "file" {
		t.Errorf("ledger = %+v", rec.records)
	}
}

func TestMover_FileRepair_WritesNoProtectedFilesStampWhenThereIsNone(t *testing.T) {
	t.Parallel()
	m, _, _ := repairFiler(t)

	res, err := m.FileRepair([]byte(repairItem), RepairStamp{Tier: "T2"})

	if err != nil {
		t.Fatal(err)
	}
	if _, has := readItem(t, res.Path)["protected_files"]; has {
		t.Error("an empty stamp list must not write protected_files")
	}
}

func TestMover_FileRepair_RefusesAnAuthoredStampAndKeepsTheAuthorCheck(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"an authored tier":            strings.Replace(repairItem, `"weight":0.4,`, `"weight":0.4,"tier":"T1",`, 1),
		"an authored protected_files": strings.Replace(repairItem, `"weight":0.4,`, `"weight":0.4,"protected_files":["a.go"],`, 1),
		"an authored lane route":      strings.Replace(repairItem, `"weight":0.4,`, `"weight":0.4,"route":"lane",`, 1),
		"a malformed priority class":  strings.Replace(repairItem, `"debuggability"`, `"urgent"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m, inbox, _ := repairFiler(t)
			if _, err := m.FileRepair([]byte(body), RepairStamp{Tier: "T1"}); !errors.Is(err, ErrInvalidItem) {
				t.Fatalf("err = %v, want ErrInvalidItem", err)
			}
			if got := jsonFiles(t, inbox); len(got) != 0 {
				t.Errorf("a refused item leaves no file: %v", got)
			}
		})
	}
}

func TestMover_File_StillRefusesAnAuthoredTier(t *testing.T) {
	t.Parallel()
	m, _, _ := repairFiler(t)
	body := strings.Replace(validItem, `"weight":0.5,`, `"weight":0.5,"tier":"T1",`, 1)
	if _, err := m.File([]byte(body)); !errors.Is(err, ErrInvalidItem) {
		t.Fatalf("err = %v, want ErrInvalidItem", err)
	}
}

func TestMover_FileRepair_RefusesAnIdAlreadyFiled(t *testing.T) {
	t.Parallel()
	m, _, _ := repairFiler(t)
	if _, err := m.FileRepair([]byte(repairItem), RepairStamp{Tier: "T1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.FileRepair([]byte(repairItem), RepairStamp{Tier: "T1"}); !errors.Is(err, ErrAlreadyFiled) {
		t.Fatalf("err = %v, want ErrAlreadyFiled", err)
	}
}

func TestMover_FileRepair_RefusesAStampThatIsEmptyOrT0(t *testing.T) {
	t.Parallel()
	for name, stamp := range map[string]RepairStamp{
		"an empty stamp":   {},
		"a stamped T0":     {Tier: "T0", ProtectedFiles: []string{"go/internal/guards/role.go"}},
		"a mis-cased tier": {Tier: "t1"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m, inbox, _ := repairFiler(t)
			if _, err := m.FileRepair([]byte(repairItem), stamp); !errors.Is(err, ErrInvalidItem) || len(jsonFiles(t, inbox)) != 0 {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestMover_FileRepair_AKernelItemCarriesOnlyItsProtectedFiles(t *testing.T) {
	t.Parallel()
	m, _, _ := repairFiler(t)
	res, err := m.FileRepair([]byte(repairItem), RepairStamp{ProtectedFiles: []string{"go/internal/guards/role.go"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, has := readItem(t, res.Path)["tier"]; has {
		t.Error("T0 is derived from protected_files and is never stamped")
	}
}
