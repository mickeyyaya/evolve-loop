package profiles

import (
	"reflect"
	"testing"
	"testing/fstest"
)

func TestLoader_TypedBindingAndGet(t *testing.T) {
	// The explicit *Loader binding names the type for apicover's named-in-test check.
	var l *Loader = NewFromFS(fixtureFS())
	p, err := l.Get("scout")
	if err != nil {
		t.Fatalf("Loader.Get(scout): %v", err)
	}
	if p.Name != "scout" {
		t.Errorf("Name = %q, want scout", p.Name)
	}
}

func TestSandboxConfig_FullStructEquality(t *testing.T) {
	const sandboxProfile = `{
	  "name": "sb", "role": "sb", "cli": "claude", "model_tier_default": "haiku",
	  "sandbox": {
	    "enabled": true,
	    "read_only_repo": true,
	    "write_subpaths": [".evolve/runs/cycle-*"],
	    "deny_subpaths": [".git"],
	    "allow_network": false
	  }
	}`
	want := &SandboxConfig{
		Enabled:       true,
		ReadOnlyRepo:  true,
		WriteSubpaths: []string{".evolve/runs/cycle-*"},
		DenySubpaths:  []string{".git"},
		AllowNetwork:  false,
	}
	p, err := NewFromFS(fstest.MapFS{
		"sb.json": &fstest.MapFile{Data: []byte(sandboxProfile)},
	}).Get("sb")
	if err != nil {
		t.Fatalf("Get(sb): %v", err)
	}
	if !reflect.DeepEqual(p.Sandbox, want) {
		t.Errorf("Sandbox = %+v, want %+v", p.Sandbox, want)
	}
}
