package lifecycle

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

var classOrder = WithPriorityClasses([]string{"correctness", "stability", "hygiene"})

func weightEdit(value string) []FieldEdit {
	return []FieldEdit{{Op: EditSet, Field: "weight", Value: value}}
}

func TestMover_Edit_RefusesAWeightOrClassEditWithoutAReasonAndWritesNothing(t *testing.T) {
	cases := map[string]struct {
		reason string
		edits  []FieldEdit
	}{
		"weight":          {"", weightEdit("0.7")},
		"blank reason":    {"  ", weightEdit("0.7")},
		"class":           {"", []FieldEdit{{Op: EditSet, Field: "priority_class", Value: "stability"}}},
		"mixed with text": {"", []FieldEdit{{Op: EditSet, Field: "title", Value: "t"}, {Op: EditSet, Field: "weight", Value: "0.7"}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			inbox, path := curationInbox(t)
			before, _ := os.ReadFile(path)
			rec := &recordingAppender{}

			_, err := New(inbox, rec, classOrder).EditBecause("curated", tc.reason, tc.edits)

			if !errors.Is(err, ErrBadArgs) {
				t.Errorf("err = %v, want ErrBadArgs", err)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) || len(rec.records) != 0 {
				t.Errorf("a refused edit changed the item or recorded %d lines", len(rec.records))
			}
		})
	}
}

func TestMover_Edit_PlainEditAndEditHeldRefuseAWeightWithoutAReason(t *testing.T) {
	inbox, _ := curationInbox(t)
	m := New(inbox, &recordingAppender{}, classOrder)

	if _, err := m.Edit("curated", weightEdit("0.7")); !errors.Is(err, ErrBadArgs) {
		t.Errorf("Edit err = %v, want ErrBadArgs", err)
	}
	if _, err := m.EditHeld("curated", 3, weightEdit("0.7")); !errors.Is(err, ErrBadArgs) {
		t.Errorf("EditHeld err = %v, want ErrBadArgs", err)
	}
}

func TestMover_EditBecause_RecordsTheReasonInTheOneEditLine(t *testing.T) {
	inbox, path := curationInbox(t)
	rec := &recordingAppender{}

	_, err := New(inbox, rec, classOrder).EditBecause("curated", " operator review ", weightEdit("0.7"))

	if err != nil || readItem(t, path)["weight"] != 0.7 {
		t.Fatalf("err = %v weight = %v, want the edit applied", err, readItem(t, path)["weight"])
	}
	if len(rec.records) != 1 || rec.records[0].Action != "edit" || rec.records[0].Message != "set weight=0.7; reason: operator review" {
		t.Errorf("ledger = %+v, want one edit line carrying the reason", rec.records)
	}
}

func TestMover_EditBecause_APlainFieldNeedsNoReasonAndKeepsItsLine(t *testing.T) {
	inbox, _ := curationInbox(t)
	rec := &recordingAppender{}

	_, err := New(inbox, rec, classOrder).Edit("curated", []FieldEdit{{Op: EditSet, Field: "title", Value: "new"}})

	if err != nil || len(rec.records) != 1 || rec.records[0].Message != "set title=new" {
		t.Errorf("err = %v ledger = %+v, want the plain edit line", err, rec.records)
	}
}

func TestMover_EditBecause_ALedgerFailureFailsTheEditAndLeavesTheItem(t *testing.T) {
	inbox, path := curationInbox(t)
	before, _ := os.ReadFile(path)
	diskFull := errors.New("disk full")
	rec := &recordingAppender{fail: diskFull}

	_, err := New(inbox, rec, classOrder).EditBecause("curated", "operator review", weightEdit("0.7"))

	if !errors.Is(err, diskFull) {
		t.Errorf("err = %v, want the ledger failure", err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Errorf("the item changed although its reason was not recorded")
	}
}

func TestMover_EditBecause_AFailedRewriteAfterTheRecordLeavesAnAbortLine(t *testing.T) {
	inbox, path := curationInbox(t)
	before, _ := os.ReadFile(path)
	rec := &recordingAppender{}
	if err := os.Chmod(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })

	_, err := New(inbox, rec, classOrder).EditBecause("curated", "operator review", weightEdit("0.7"))

	if err == nil {
		t.Fatal("want the rewrite fault")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Errorf("the item changed")
	}
	if len(rec.records) != 2 || rec.records[1].Action != "edit-aborted" {
		t.Errorf("ledger = %+v, want the edit line then an edit-aborted line", rec.records)
	}
}

func TestMover_EditBecause_ARefusedValueRecordsNothing(t *testing.T) {
	inbox, _ := curationInbox(t)
	rec := &recordingAppender{}

	_, err := New(inbox, rec, classOrder).EditBecause("curated", "operator review", weightEdit("1.5"))

	if !errors.Is(err, ErrInvalidItem) || len(rec.records) != 0 {
		t.Errorf("err = %v records = %d, want ErrInvalidItem and no ledger line", err, len(rec.records))
	}
}

func TestMover_EditBecause_ChecksAPriorityClassAgainstTheWiredOrder(t *testing.T) {
	class := []FieldEdit{{Op: EditSet, Field: "priority_class", Value: "no-such-class"}}
	inbox, path := curationInbox(t)
	before, _ := os.ReadFile(path)
	rec := &recordingAppender{}

	_, err := New(inbox, rec, classOrder).EditBecause("curated", "typo", class)
	if !errors.Is(err, ErrInvalidItem) {
		t.Errorf("err = %v, want ErrInvalidItem for a class outside the order", err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) || len(rec.records) != 0 {
		t.Errorf("a refused class changed the item or recorded a line")
	}

	_, err = New(inbox, rec).EditBecause("curated", "no order", []FieldEdit{{Op: EditSet, Field: "priority_class", Value: "stability"}})
	if !errors.Is(err, errNoClassOrder) {
		t.Errorf("err = %v, want errNoClassOrder when no order is wired", err)
	}
}

func TestCheckEdit_NamesWhatEditBecauseWouldRefuseBeforeAnyItemIsRead(t *testing.T) {
	if err := CheckEdit(weightEdit("0.7"), ""); !errors.Is(err, ErrBadArgs) {
		t.Errorf("CheckEdit without reason = %v, want ErrBadArgs", err)
	}
	if err := CheckEdit(weightEdit("0.7"), "why"); err != nil {
		t.Errorf("CheckEdit with reason = %v, want nil", err)
	}
	if err := CheckEdit(nil, "why"); !errors.Is(err, ErrBadArgs) {
		t.Errorf("CheckEdit of no edits = %v, want ErrBadArgs", err)
	}
}
