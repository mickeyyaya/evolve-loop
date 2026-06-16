package profiles

import (
	"reflect"
	"testing"
	"testing/fstest"
)

// TestLoader_TypedBindingAndGet names the Loader type via an explicit typed
// binding (NewFromFS returns *Loader) and pins the round-trip contract: a
// Loader built over an fs.FS resolves a profile by name. Mirrors how the
// composition root holds a *profiles.Loader and calls Get.
func TestLoader_TypedBindingAndGet(t *testing.T) {
	var l *Loader = NewFromFS(fixtureFS())
	p, err := l.Get("scout")
	if err != nil {
		t.Fatalf("Loader.Get(scout): %v", err)
	}
	if p.Name != "scout" {
		t.Errorf("Name = %q, want scout", p.Name)
	}
}

// TestSandboxConfig_FullStructEquality binds a SandboxConfig literal and asserts
// it round-trips through the JSON the Loader parses. This pins the typed shape
// of profile.sandbox that the sandbox adapter (and phaseconfig/phaseregistrar
// consumers) depend on — e.g. read_only_repo:true must survive so an auditor
// profile cannot mutate the repo.
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
