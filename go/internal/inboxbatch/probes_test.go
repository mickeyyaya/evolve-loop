package inboxbatch_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestItem_DecodesTypedProbesAsKindAndArgument(t *testing.T) {
	raw := `{"id":"a","probes":[{"kind":"test-exists","arg":"TestDone"},{"kind":"path-absent","arg":"go/internal/old/old.go"}]}`

	var it inboxbatch.Item
	if err := json.Unmarshal([]byte(raw), &it); err != nil {
		t.Fatalf("Unmarshal = %v", err)
	}

	want := []inboxbatch.Probe{
		{Kind: inboxbatch.ProbeTestExists, Arg: "TestDone"},
		{Kind: inboxbatch.ProbePathAbsent, Arg: "go/internal/old/old.go"},
	}
	if !slices.Equal(it.Probes, want) {
		t.Errorf("Probes = %+v, want %+v", it.Probes, want)
	}
	out, err := json.Marshal(inboxbatch.Item{ID: "b"})
	if err != nil || containsKey(out, "probes") {
		t.Errorf("an item with no probes marshals %s (%v); want no probes key", out, err)
	}
}

func containsKey(raw []byte, key string) bool {
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil {
		return false
	}
	_, present := doc[key]
	return present
}

func TestProbeKinds_ListsTheThreeTypedKinds(t *testing.T) {
	got := inboxbatch.ProbeKinds()

	want := []string{inboxbatch.ProbeTestExists, inboxbatch.ProbePathExists, inboxbatch.ProbePathAbsent}
	if len(got) != len(want) {
		t.Fatalf("ProbeKinds = %v, want %v", got, want)
	}
	for _, kind := range want {
		if !slices.Contains(got, kind) {
			t.Errorf("ProbeKinds = %v, lacks %q", got, kind)
		}
	}
	if inboxbatch.ProbeTestExists != "test-exists" || inboxbatch.ProbePathExists != "path-exists" || inboxbatch.ProbePathAbsent != "path-absent" {
		t.Errorf("the kind spellings are %q %q %q; the items on disk spell test-exists, path-exists and path-absent",
			inboxbatch.ProbeTestExists, inboxbatch.ProbePathExists, inboxbatch.ProbePathAbsent)
	}
}

func TestProbeKinds_ReturnsACopyCallersCannotMutate(t *testing.T) {
	first := inboxbatch.ProbeKinds()
	first[0] = "tampered"

	if slices.Contains(inboxbatch.ProbeKinds(), "tampered") {
		t.Errorf("ProbeKinds shares its backing array with the caller")
	}
}
