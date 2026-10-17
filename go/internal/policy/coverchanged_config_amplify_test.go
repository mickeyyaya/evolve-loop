package policy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCoverChangedConfigAmplify_PolicyJSONStageTable(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"an empty object enforces":          {`{}`, "enforce"},
		"an empty cover_changed enforces":   {`{"cover_changed":{}}`, "enforce"},
		"an unrelated block still enforces": {`{"gc":{"mode":"enforce"}}`, "enforce"},
		"explicit off wins":                 {`{"cover_changed":{"stage":"off"}}`, "off"},
		"explicit enforce stays":            {`{"cover_changed":{"stage":"enforce"}}`, "enforce"},
		"explicit shadow wins":              {`{"cover_changed":{"stage":"shadow"}}`, "shadow"},
	} {
		path := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := policy.Load(path)
		if err != nil {
			t.Fatalf("%s: load: %v", name, err)
		}
		if got := p.CoverChangedConfig().Stage; got != tc.want {
			t.Errorf("%s: stage = %q, want %q", name, got, tc.want)
		}
	}
}

func TestCoverChangedConfigAmplify_ReadingTheConfigDoesNotMutateThePolicy(t *testing.T) {
	p := policy.Policy{CoverChanged: &policy.CoverChangedPolicy{}}
	_ = p.CoverChangedConfig()
	if p.CoverChanged.Stage != "" {
		t.Errorf("policy block mutated to %q", p.CoverChanged.Stage)
	}
	var zero policy.Policy
	_ = zero.CoverChangedConfig()
	if zero.CoverChanged != nil {
		t.Error("absent block was materialised")
	}
}
