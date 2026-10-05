package lifecycle

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const curatedItem = `{"id":"curated","kind":"feature","title":"a curated item","summary":"s","fix":"f","weight":0.5,` +
	`"priority_class":"hygiene","files":["go/a.go","go/b.go"],"deps":["base"],"connects_to":["peer"],` +
	`"acceptance":["one"],"route":"console-manual","routed_reason":"why","routed_cycle":7,"failure_count":2,"source":"console"}`

func curationInbox(t *testing.T) (inbox, path string) {
	t.Helper()
	inbox = newInbox(t)
	path = filepath.Join(inbox, "2026-09-30T00-00-00Z-curated.json")
	writeItem(t, path, curatedItem)
	writeItem(t, filepath.Join(inbox, "base.json"), `{"id":"base"}`)
	writeItem(t, filepath.Join(inbox, "consumed", "done.json"), `{"id":"done"}`)
	return inbox, path
}

func withoutKeys(item map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for k, v := range item {
		out[k] = v
	}
	for _, k := range keys {
		delete(out, k)
	}
	return out
}

func TestMover_Edit_SetsOnlyTheNamedFieldAndRecordsIt(t *testing.T) {
	inbox, path := curationInbox(t)
	before := readItem(t, path)
	rec := &recordingAppender{}

	got, err := New(inbox, rec).Edit("curated", []FieldEdit{{Op: EditSet, Field: "weight", Value: "0.4"}})

	if err != nil || got != path {
		t.Fatalf("Edit = (%q, %v), want (%q, nil)", got, err, path)
	}
	after := readItem(t, path)
	if after["weight"] != 0.4 {
		t.Errorf("weight = %v, want 0.4", after["weight"])
	}
	if !reflect.DeepEqual(withoutKeys(before, "weight"), withoutKeys(after, "weight")) {
		t.Errorf("an edit of weight changed another field:\nbefore %v\nafter  %v", before, after)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "edit" || rec.records[0].TaskID != "curated" ||
		rec.records[0].Message != "set weight=0.4" {
		t.Errorf("ledger = %+v, want one edit line naming the change", rec.records)
	}
}

func TestMover_Edit_AppliesListEditsInOrder(t *testing.T) {
	inbox, path := curationInbox(t)
	rec := &recordingAppender{}

	_, err := New(inbox, rec).Edit("curated", []FieldEdit{
		{Op: EditAdd, Field: "deps", Value: "done"},
		{Op: EditRemove, Field: "files", Value: "go/a.go"},
		{Op: EditAdd, Field: "files", Value: "go/c.go"},
		{Op: EditSet, Field: "acceptance", Value: `["first","second"]`},
		{Op: EditSet, Field: "connects_to", Value: `[]`},
		{Op: EditSet, Field: "title", Value: "a sharper title"},
	})

	if err != nil {
		t.Fatal(err)
	}
	item := readItem(t, path)
	for field, want := range map[string]any{
		"deps":        []any{"base", "done"},
		"files":       []any{"go/b.go", "go/c.go"},
		"acceptance":  []any{"first", "second"},
		"connects_to": []any{},
		"title":       "a sharper title",
	} {
		if !reflect.DeepEqual(item[field], want) {
			t.Errorf("%s = %#v, want %#v", field, item[field], want)
		}
	}
	if want := `add deps=done; remove files=go/a.go; add files=go/c.go; set acceptance=["first","second"]; set connects_to=[]; set title=a sharper title`; len(rec.records) != 1 || rec.records[0].Message != want {
		t.Errorf("ledger = %+v, want the message %q", rec.records, want)
	}
}

func TestMover_Edit_RefusesAFieldOutsideTheCurationSet(t *testing.T) {
	for _, edit := range []FieldEdit{
		{Op: EditSet, Field: "route", Value: "lane"},
		{Op: EditSet, Field: "routed_reason", Value: "x"},
		{Op: EditSet, Field: "failure_count", Value: "0"},
		{Op: EditSet, Field: "premise_verified_at", Value: "2026-09-30T00:00:00Z"},
		{Op: EditSet, Field: "kind", Value: "feature"},
		{Op: EditSet, Field: "created_at", Value: "2026-09-30"},
		{Op: EditSet, Field: "injected_by", Value: ""},
		{Op: EditAdd, Field: "weight", Value: "0.1"},
		{Op: EditRemove, Field: "title", Value: "a curated item"},
		{Op: "replace", Field: "title", Value: "x"},
	} {
		t.Run(string(edit.Op)+" "+edit.Field, func(t *testing.T) {
			inbox, path := curationInbox(t)
			before, _ := os.ReadFile(path)
			rec := &recordingAppender{}

			_, err := New(inbox, rec).Edit("curated", []FieldEdit{edit})

			if !errors.Is(err, ErrBadArgs) {
				t.Errorf("err = %v, want ErrBadArgs", err)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) || len(rec.records) != 0 {
				t.Errorf("a refused edit changed the item or the ledger:\n%s\n%+v", after, rec.records)
			}
		})
	}
	if _, err := New(newInbox(t), nil).Edit("curated", nil); !errors.Is(err, ErrBadArgs) {
		t.Errorf("no edits: err = %v, want ErrBadArgs", err)
	}
}

func TestMover_Edit_RefusesAResultInboxAddWouldRefuse(t *testing.T) {
	for name, tc := range map[string]struct {
		edit FieldEdit
		why  string
	}{
		"a dependency naming no item":  {FieldEdit{Op: EditAdd, Field: "deps", Value: "no-such-item"}, "no-such-item"},
		"a dependency on itself":       {FieldEdit{Op: EditSet, Field: "deps", Value: `["curated"]`}, "itself"},
		"a weight of zero":             {FieldEdit{Op: EditSet, Field: "weight", Value: "0"}, "weight"},
		"a weight above one":           {FieldEdit{Op: EditSet, Field: "weight", Value: "1.5"}, "weight"},
		"a weight that is no number":   {FieldEdit{Op: EditSet, Field: "weight", Value: "heavy"}, "weight"},
		"a blank title":                {FieldEdit{Op: EditSet, Field: "title", Value: "  "}, "title"},
		"a title with a control char":  {FieldEdit{Op: EditSet, Field: "title", Value: "a\nb"}, "title"},
		"no acceptance criterion":      {FieldEdit{Op: EditSet, Field: "acceptance", Value: `[]`}, "acceptance"},
		"a list that is no JSON array": {FieldEdit{Op: EditSet, Field: "files", Value: "go/a.go"}, "JSON array"},
		"a dependency already listed":  {FieldEdit{Op: EditAdd, Field: "deps", Value: "base"}, "already"},
		"an entry the list lacks":      {FieldEdit{Op: EditRemove, Field: "files", Value: "go/z.go"}, "does not list"},
		"a rename of a filed id":       {FieldEdit{Op: EditSet, Field: "id", Value: "renamed"}, "never renamed"},
		"a blank list entry":           {FieldEdit{Op: EditAdd, Field: "files", Value: "  "}, "blank entry"},
	} {
		t.Run(name, func(t *testing.T) {
			inbox, path := curationInbox(t)
			before, _ := os.ReadFile(path)
			rec := &recordingAppender{}

			_, err := New(inbox, rec).Edit("curated", []FieldEdit{tc.edit})

			if !errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), tc.why) {
				t.Errorf("err = %v, want ErrInvalidItem naming %q", err, tc.why)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) || len(rec.records) != 0 {
				t.Errorf("a refused edit changed the item or the ledger:\n%s\n%+v", after, rec.records)
			}
		})
	}
}

func TestMover_Edit_JudgesOnlyTheFieldsItWrites(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "legacy.json")
	writeItem(t, path, `{"id":"legacy","title":"`+strings.Repeat("t", 200)+`","weight":0.2}`)
	m := New(inbox, nil)

	if _, err := m.Edit("legacy", []FieldEdit{{Op: EditSet, Field: "weight", Value: "0.3"}}); err != nil {
		t.Errorf("a weight edit on an item with a long title and no summary: %v, want it written", err)
	}
	if _, err := m.Edit("legacy", []FieldEdit{{Op: EditAdd, Field: "acceptance", Value: strings.Repeat("a", 700)}}); !errors.Is(err, ErrInvalidItem) {
		t.Errorf("an overlong criterion: err = %v, want ErrInvalidItem", err)
	}
	if item := readItem(t, path); item["weight"] != 0.3 {
		t.Errorf("item = %v", item)
	}
}

func TestMover_Edit_StampsTheIDOfAnIDlessItemNamedByItsPath(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "2026-09-27T00-00-00Z-nameless.json")
	writeItem(t, path, `{"title":"an item filed by hand without an id","weight":0.3}`)
	writeItem(t, filepath.Join(inbox, "consumed", "taken.json"), `{"id":"taken"}`)
	rec := &recordingAppender{}
	m := New(inbox, rec)

	if _, err := m.Edit(path, []FieldEdit{{Op: EditSet, Field: "id", Value: "taken"}}); !errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), "already") {
		t.Errorf("an id the inbox holds: err = %v, want ErrInvalidItem", err)
	}
	if _, err := m.Edit(path, []FieldEdit{{Op: EditSet, Field: "id", Value: "Not Kebab"}}); !errors.Is(err, ErrInvalidItem) {
		t.Errorf("a non-kebab id: err = %v, want ErrInvalidItem", err)
	}
	got, err := m.Edit(path, []FieldEdit{{Op: EditSet, Field: "id", Value: "nameless"}})

	if err != nil || got != path {
		t.Fatalf("Edit = (%q, %v)", got, err)
	}
	if item := readItem(t, path); item["id"] != "nameless" {
		t.Errorf("item = %v, want the id stamped", item)
	}
	if loc, err := Locate(inbox, "nameless"); err != nil || loc.Path != path {
		t.Errorf("Locate after the stamp = (%+v, %v), want the item found by its new id", loc, err)
	}
	if len(rec.records) != 1 || rec.records[0].TaskID != "nameless" {
		t.Errorf("ledger = %+v, want one line under the new id", rec.records)
	}
}

func TestMover_Edit_LeavesAClaimedMissingOrOutsideItemAlone(t *testing.T) {
	inbox := newInbox(t)
	claimed := procPath(inbox, 1780, "held.json")
	writeItem(t, claimed, `{"id":"held","weight":0.5}`)
	outside := filepath.Join(t.TempDir(), "elsewhere.json")
	writeItem(t, outside, `{"weight":0.5}`)
	retired := filepath.Join(inbox, "consumed", "retired.json")
	writeItem(t, retired, `{"weight":0.5}`)
	m := New(inbox, nil)
	weight := []FieldEdit{{Op: EditSet, Field: "weight", Value: "0.4"}}

	for name, target := range map[string]string{
		"a claimed item":           "held",
		"a missing id":             "absent",
		"a claimed item's path":    claimed,
		"a file outside the inbox": outside,
		"a path to no file":        filepath.Join(inbox, "absent.json"),
		"an empty target":          "",
		"a retired item's path":    retired,
	} {
		if _, err := m.Edit(target, weight); !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrBadArgs) {
			t.Errorf("%s: err = %v, want ErrNotFound or ErrBadArgs", name, err)
		}
	}
	if item := readItem(t, claimed); item["weight"] != 0.5 {
		t.Errorf("claimed item = %v, want it untouched", item)
	}
}

func TestMover_Edit_ARewriteFaultIsNeitherARefusalNorAMissingItem(t *testing.T) {
	inbox, path := curationInbox(t)
	before, _ := os.ReadFile(path)
	rec := &recordingAppender{}
	if err := os.Chmod(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })

	_, err := New(inbox, rec).Edit("curated", []FieldEdit{{Op: EditSet, Field: "weight", Value: "0.4"}})

	if err == nil || errors.Is(err, ErrInvalidItem) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrBadArgs) {
		t.Errorf("err = %v, want the rewrite fault itself", err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) || len(rec.records) != 0 {
		t.Errorf("a failed rewrite changed the item or recorded an edit")
	}
}

func TestMover_Edit_RefusesAnItemItsReadersCannotLoad(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	writeItem(t, path, `{"id":"x","weight":0.2,"files":"go/a.go","acceptance":"one"}`)
	m := New(inbox, nil)

	if _, err := m.Edit("x", []FieldEdit{{Op: EditAdd, Field: "files", Value: "go/b.go"}}); !errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), "not a list") {
		t.Errorf("an add to a field that is no list: err = %v, want ErrInvalidItem", err)
	}
	if _, err := m.Edit("x", []FieldEdit{{Op: EditSet, Field: "weight", Value: "0.3"}}); !errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), "does not load") {
		t.Errorf("an edit that leaves the item unloadable: err = %v, want ErrInvalidItem", err)
	}
	if _, err := m.Edit("x", []FieldEdit{{Op: EditSet, Field: "files", Value: `["go/a.go"]`}, {Op: EditSet, Field: "acceptance", Value: `["one"]`}}); err != nil {
		t.Errorf("an edit that repairs the item: %v, want it written", err)
	}
}

func TestMover_Edit_AScanOrStatFaultIsNotARefusal(t *testing.T) {
	inbox, _ := curationInbox(t)
	locked := filepath.Join(inbox, "consumed")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	m := New(inbox, nil)

	if _, err := m.Edit("curated", []FieldEdit{{Op: EditAdd, Field: "deps", Value: "done"}}); err == nil || errors.Is(err, ErrInvalidItem) || errors.Is(err, ErrNotFound) {
		t.Errorf("a deps edit over an unreadable tree: err = %v, want the scan fault", err)
	}
	if _, err := m.Edit(filepath.Join(locked, "x.json"), []FieldEdit{{Op: EditSet, Field: "weight", Value: "0.4"}}); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("a path the edit cannot stat: err = %v, want the stat fault", err)
	}
	if _, err := m.Edit("curated", []FieldEdit{{Op: EditSet, Field: "weight", Value: "0.4"}}); err != nil {
		t.Errorf("a weight edit needs no identity scan: %v", err)
	}
}

func TestMover_Edit_NeverOpensAConsoleOwnedItemToLanes(t *testing.T) {
	isProtected := func(p string) bool { return p == "go/internal/guards/phase.go" }
	for name, tc := range map[string]struct {
		body string
		edit FieldEdit
		open bool
	}{
		"dropping the protected file it declares":         {`{"id":"x","weight":0.5,"files":["go/internal/guards/phase.go"]}`, FieldEdit{Op: EditRemove, Field: "files", Value: "go/internal/guards/phase.go"}, true},
		"declaring an unprotected surface over a mention": {`{"id":"x","weight":0.5,"summary":"breaks in go/internal/guards/phase.go"}`, FieldEdit{Op: EditAdd, Field: "files", Value: "go/a.go"}, true},
		"reweighting a pipeline-repair item":              {`{"id":"x","weight":0.5,"kind":"pipeline-repair"}`, FieldEdit{Op: EditSet, Field: "weight", Value: "0.4"}, false},
		"declaring a protected file on a lane item":       {`{"id":"x","weight":0.5,"files":["go/a.go"]}`, FieldEdit{Op: EditAdd, Field: "files", Value: "go/internal/guards/phase.go"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			inbox := newInbox(t)
			path := filepath.Join(inbox, "x.json")
			writeItem(t, path, tc.body)
			before, _ := os.ReadFile(path)
			rec := &recordingAppender{}

			_, err := New(inbox, rec, WithProtectedPath(isProtected)).Edit("x", []FieldEdit{tc.edit})

			if !tc.open {
				if err != nil {
					t.Errorf("err = %v, want the edit written", err)
				}
				return
			}
			if !errors.Is(err, ErrConsoleRouted) || !strings.Contains(err.Error(), "evolve inbox route-lane") {
				t.Errorf("err = %v, want ErrConsoleRouted pointing to route-lane", err)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) || len(rec.records) != 0 {
				t.Errorf("a refused edit changed the item or the ledger:\n%s", after)
			}
		})
	}
}
