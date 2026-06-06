// provenance_test.go — cycle-238 task `profile-provenance-field` (RED first).
//
// Pins the Invariant-1 foothold: every profile carries a `generated_from`
// provenance marker distinguishing hand-authored originals from generated
// projections (campaign retro §4, migration step 4; architecture-design R1,
// blueprint B1/B2). Contract for Builder:
//
//	Profile gains `GeneratedFrom string `json:"generated_from,omitempty"``
//
// Provenance vocabulary is free-form (architecture: "hand-authored" today,
// "phasespec:<name>@<sha>" later) — the validation logic lives at the CLI
// (`phases validate`, see cmd_phases_cycle238_test.go), not here.
package profiles

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

// stampedProfile mirrors a post-cycle-238 .evolve/profiles/*.json with the
// provenance stamp applied.
const stampedProfile = `{
  "name": "stamped",
  "role": "stamped",
  "cli": "claude-tmux",
  "model_tier_default": "sonnet",
  "generated_from": "hand-authored"
}`

func TestProvenanceField_ParsesGeneratedFrom(t *testing.T) {
	l := NewFromFS(fstest.MapFS{
		"stamped.json": &fstest.MapFile{Data: []byte(stampedProfile)},
	})
	p, err := l.Get("stamped")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.GeneratedFrom != "hand-authored" {
		t.Errorf("GeneratedFrom = %q, want %q", p.GeneratedFrom, "hand-authored")
	}
}

func TestProvenanceField_EmptyWhenAbsent(t *testing.T) {
	// B2(b): a pre-stamp profile (no generated_from key) round-trips with
	// the zero value — absence is the signal `phases validate` keys off.
	l := NewFromFS(fixtureFS())
	p, err := l.Get("scout")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.GeneratedFrom != "" {
		t.Errorf("GeneratedFrom = %q, want empty for pre-stamp profile", p.GeneratedFrom)
	}
}

func TestProvenanceField_MarshalsAsGeneratedFromKey(t *testing.T) {
	b, err := json.Marshal(Profile{Name: "x", GeneratedFrom: "hand-authored"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"generated_from":"hand-authored"`) {
		t.Errorf("marshal output missing generated_from key: %s", b)
	}
}

func TestProvenanceField_OmittedWhenEmpty(t *testing.T) {
	// omitempty: an unstamped Profile must not grow a phantom empty key on
	// marshal — the JSON round-trip of unstamped profiles is unchanged (B1).
	b, err := json.Marshal(Profile{Name: "naked"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(b), "generated_from") {
		t.Errorf("empty GeneratedFrom must be omitted, got: %s", b)
	}
}
