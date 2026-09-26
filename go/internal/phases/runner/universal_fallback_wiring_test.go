package runner

import "testing"

// resetUniversalFallbackDefaults restores the package defaults, so a test that sets them cannot leak into the next.
func resetUniversalFallbackDefaults(t *testing.T) {
	t.Helper()
	origFn, origEnabled := DefaultDiscoverCLIsFn, DefaultUniversalFallback
	t.Cleanup(func() { DefaultDiscoverCLIsFn, DefaultUniversalFallback = origFn, origEnabled })
}

func TestNew_UniversalFallbackDefaults_PackageVarFallthrough(t *testing.T) {
	resetUniversalFallbackDefaults(t)
	sentinel := []string{"agy-tmux"}
	DefaultUniversalFallback = true
	DefaultDiscoverCLIsFn = func() []string { return sentinel }

	b := New(Options{Hooks: &fakeHooks{phase: "audit"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-auditor", "x")})
	if !b.universalFallback {
		t.Error("New() must inherit DefaultUniversalFallback when Options.UniversalFallback is unset")
	}
	if b.discoverCLIsFn == nil {
		t.Fatal("New() must inherit DefaultDiscoverCLIsFn when Options.DiscoverCLIsFn is nil")
	}
	if got := b.discoverCLIsFn(); len(got) != 1 || got[0] != "agy-tmux" {
		t.Errorf("inherited discovery fn returned %v, want [agy-tmux]", got)
	}
}

func TestNew_UniversalFallbackOptions_OverridePackageVar(t *testing.T) {
	resetUniversalFallbackDefaults(t)
	DefaultDiscoverCLIsFn = func() []string { return []string{"package-var"} }

	injected := []string{"codex-tmux"}
	b := New(Options{
		Hooks: &fakeHooks{phase: "build"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-builder", "x"),
		UniversalFallback: true,
		DiscoverCLIsFn:    func() []string { return injected },
	})
	if got := b.discoverCLIsFn(); len(got) != 1 || got[0] != "codex-tmux" {
		t.Errorf("Options.DiscoverCLIsFn must override the package var; got %v", got)
	}
}

func TestNew_UniversalFallbackDefault_OffIsInert(t *testing.T) {
	resetUniversalFallbackDefaults(t)
	DefaultUniversalFallback = false
	DefaultDiscoverCLIsFn = nil

	b := New(Options{Hooks: &fakeHooks{phase: "scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x")})
	if b.universalFallback || b.discoverCLIsFn != nil {
		t.Errorf("zero Options + zero package vars must be inert; got enabled=%v fn=%v", b.universalFallback, b.discoverCLIsFn != nil)
	}
}
