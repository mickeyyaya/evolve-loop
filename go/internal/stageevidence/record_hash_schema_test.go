package stageevidence

import (
	"reflect"
	"slices"
	"testing"
)

func fieldNames(rt reflect.Type) []string {
	names := make([]string, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		names = append(names, rt.Field(i).Name)
	}
	return names
}

func undecidedFields(record reflect.Type, hashed reflect.Type, unhashed []string) []string {
	hashedNames := fieldNames(hashed)
	var undecided []string
	for _, name := range fieldNames(record) {
		if !slices.Contains(hashedNames, name) && !slices.Contains(unhashed, name) {
			undecided = append(undecided, name)
		}
	}
	return undecided
}

func contradictoryFields(hashed reflect.Type, unhashed []string) []string {
	hashedNames := fieldNames(hashed)
	var both []string
	for _, name := range unhashed {
		if slices.Contains(hashedNames, name) {
			both = append(both, name)
		}
	}
	return both
}

func staleDecisions(record reflect.Type, hashed reflect.Type, unhashed []string) []string {
	recordNames := fieldNames(record)
	var stale []string
	for _, name := range append(slices.Clone(fieldNames(hashed)), unhashed...) {
		if !slices.Contains(recordNames, name) {
			stale = append(stale, name)
		}
	}
	return stale
}

func TestRecordHashSchema_EveryRecordFieldIsHashedOrDeclaredUnhashed(t *testing.T) {
	record, hashed := reflect.TypeOf(Record{}), reflect.TypeOf(hashedRecord{})
	if got := undecidedFields(record, hashed, unhashedRecordFields); len(got) != 0 {
		t.Errorf("Record fields with no hashing decision (add to hashedRecord or unhashedRecordFields): %v", got)
	}
	if got := contradictoryFields(hashed, unhashedRecordFields); len(got) != 0 {
		t.Errorf("fields both hashed and declared unhashed: %v", got)
	}
	if got := staleDecisions(record, hashed, unhashedRecordFields); len(got) != 0 {
		t.Errorf("hashing decisions naming no Record field: %v", got)
	}
}

func TestRecordHashSchema_GuardFlagsAFieldAddedWithoutDecision(t *testing.T) {
	type grownDirect struct {
		ID         int
		Hash       string
		Annotation string
	}
	type grownHashed struct {
		ID int
	}
	grown := reflect.TypeOf(grownDirect{})
	hashed := reflect.TypeOf(grownHashed{})
	if got := undecidedFields(grown, hashed, []string{"Hash"}); !slices.Equal(got, []string{"Annotation"}) {
		t.Fatalf("undecided = %v, want [Annotation]", got)
	}
	if got := undecidedFields(grown, hashed, []string{"Hash", "Annotation"}); len(got) != 0 {
		t.Errorf("declared field still undecided: %v", got)
	}
	if got := contradictoryFields(hashed, []string{"ID"}); !slices.Equal(got, []string{"ID"}) {
		t.Errorf("contradiction = %v, want [ID]", got)
	}
	if got := staleDecisions(grown, hashed, []string{"Gone"}); !slices.Equal(got, []string{"Gone"}) {
		t.Errorf("stale = %v, want [Gone]", got)
	}
}

func mutated(t *testing.T, base Record, name string) Record {
	t.Helper()
	field := reflect.ValueOf(&base).Elem().FieldByName(name)
	switch field.Kind() {
	case reflect.String:
		field.SetString("changed")
	case reflect.Int:
		field.SetInt(7)
	case reflect.Map:
		field.Set(reflect.ValueOf(map[string]string{"changed": "yes"}))
	default:
		t.Fatalf("Record.%s has kind %s: teach mutated() to set it", name, field.Kind())
	}
	return base
}

func TestRecordHashSchema_HashChangesExactlyWhenAHashedFieldChanges(t *testing.T) {
	base := Record{Scope: ScopeCycle, ID: 3, Session: "s", Key: "k", Version: "v", Kind: KindDecision, Seq: 1, Fields: map[string]string{"a": "b"}, Prev: "p", Hash: "h"}
	baseline := computeRecordHash(base)
	for _, name := range fieldNames(reflect.TypeOf(Record{})) {
		changed := computeRecordHash(mutated(t, base, name)) != baseline
		wantChanged := !slices.Contains(unhashedRecordFields, name)
		if changed != wantChanged {
			t.Errorf("Record.%s: hash changed=%v, want %v", name, changed, wantChanged)
		}
	}
}
