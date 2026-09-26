package router

import (
	"encoding/json"
	"testing"
)

func TestMintSpec_CarriesSelectMetadata(t *testing.T) {
	t.Parallel()
	var spec MintSpec
	raw := `{"prompt":"p","tier":"deep","cli":"claude","description":"what it produces","when_to_use":"when to select it"}`
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if spec.Description != "what it produces" {
		t.Errorf("Description=%q, want %q (check the json tag)", spec.Description, "what it produces")
	}
	if spec.WhenToUse != "when to select it" {
		t.Errorf("WhenToUse=%q, want %q (check the json tag)", spec.WhenToUse, "when to select it")
	}
}

func TestMintSpec_MetadataOmitEmpty(t *testing.T) {
	t.Parallel()
	got, err := json.Marshal(MintSpec{Prompt: "p", Tier: "deep", CLI: "claude"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"prompt":"p","tier":"deep","cli":"claude"}`; string(got) != want {
		t.Errorf("wire form changed:\n got %s\nwant %s (both new fields need omitempty)", got, want)
	}
}
