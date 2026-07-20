package runner

import "testing"

// resetUniversalFallbackDefaults restores the composition-root package seams
// after a test mutates them, guarding against cross-test leakage within the
// single `go test` process (go-review MEDIUM: these vars, unlike
// PhaseBoundaryCheckpointer, are re-assigned per wireOrchestratorDeps call).
func resetUniversalFallbackDefaults(t *testing.T) {
	t.Helper()
	origFn, origEnabled := DefaultDiscoverCLIsFn, DefaultUniversalFallback
	t.Cleanup(func() { DefaultDiscoverCLIsFn, DefaultUniversalFallback = origFn, origEnabled })
}

// TestNew_UniversalFallbackDefaults_PackageVarFallthrough — the wiring
// precedence: when Options leaves the universal-fallback fields zero, New()
// falls through to the composition-root package seams (the ~10 phase
// constructors rely on this instead of each threading the discovery closure).
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

// TestNew_UniversalFallbackOptions_OverridePackageVar — per-instance Options
// win over the package defaults (test-injection precedence), so a test's
// injected discovery closure is authoritative regardless of composition state.
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

// TestNew_UniversalFallbackDefault_OffIsInert — with both Options and the
// package vars at their zero values, the runner carries no universal fallback:
// byte-identical to the pre-feature dispatch (the feature is opt-in/default-off
// at the seam level; production turns it on via the composition root).
func TestNew_UniversalFallbackDefault_OffIsInert(t *testing.T) {
	resetUniversalFallbackDefaults(t)
	DefaultUniversalFallback = false
	DefaultDiscoverCLIsFn = nil

	b := New(Options{Hooks: &fakeHooks{phase: "scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x")})
	if b.universalFallback || b.discoverCLIsFn != nil {
		t.Errorf("zero Options + zero package vars must be inert; got enabled=%v fn=%v", b.universalFallback, b.discoverCLIsFn != nil)
	}
}
