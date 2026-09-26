package profiles

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

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
	b, err := json.Marshal(Profile{Name: "naked"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(b), "generated_from") {
		t.Errorf("empty GeneratedFrom must be omitted, got: %s", b)
	}
}
