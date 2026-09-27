package core

import (
	"encoding/json"
	"reflect"
	"testing"
)

func everyFieldSet(t *testing.T) LedgerEntry {
	t.Helper()
	var e LedgerEntry
	v := reflect.ValueOf(&e).Elem()
	for i := 0; i < v.NumField(); i++ {
		f, name := v.Field(i), v.Type().Field(i).Name
		switch f.Kind() {
		case reflect.String:
			f.SetString("v-" + name)
		case reflect.Int:
			f.SetInt(int64(i + 1))
		case reflect.Slice:
			f.Set(reflect.ValueOf([]string{"w-" + name}))
		default:
			t.Fatalf("LedgerEntry.%s has kind %s, which this round trip does not fill", name, f.Kind())
		}
	}
	return e
}

func TestLedgerEntry_EveryFieldSurvivesTheLedgerRoundTrip(t *testing.T) {
	want := everyFieldSet(t)
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got LedgerEntry
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a ledger row lost fields between write and read:\n got %+v\nwant %+v\n(from %s)", got, want, raw)
	}
}
