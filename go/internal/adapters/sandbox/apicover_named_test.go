package sandbox

import (
	"testing"
)

// TestBwrapPrefix_PrefixOnlyAndComposesGenerate pins BwrapPrefix's contract:
// it returns ONLY the bwrap flag prefix (no inner argv), and GenerateBwrapArgv
// is exactly append(BwrapPrefix(cfg), inner...). The first assertion proves the
// prefix carries the namespace flags and excludes any inner command; the second
// proves the two entry points stay in lockstep (the backcompat invariant in the
// source doc comment).
func TestBwrapPrefix_PrefixOnlyAndComposesGenerate(t *testing.T) {
	cfg := canonicalConfig()
	prefix := BwrapPrefix(cfg)

	// Prefix must contain the namespace flags...
	mustContainArg(t, prefix, "--unshare-user")
	mustContainArg(t, prefix, "--bind", "/repo", "/repo") // canonicalConfig is rw repo
	// ...and must NOT contain any inner command token.
	for _, a := range prefix {
		if a == "claude" || a == "true" {
			t.Errorf("BwrapPrefix leaked an inner-argv token %q: %v", a, prefix)
		}
	}

	// GenerateBwrapArgv(cfg, inner) == append(BwrapPrefix(cfg), inner...).
	inner := []string{"claude", "-p", "go"}
	full := GenerateBwrapArgv(cfg, inner)
	want := append(BwrapPrefix(cfg), inner...)
	if len(full) != len(want) {
		t.Fatalf("len mismatch: GenerateBwrapArgv=%d want=%d", len(full), len(want))
	}
	for i := range want {
		if full[i] != want[i] {
			t.Fatalf("arg %d: GenerateBwrapArgv=%q want=%q", i, full[i], want[i])
		}
	}
}

// TestLookPathFunc_DrivesProbeForBranch pins LookPathFunc as the injectable
// seam: a value of the named type, when it resolves "bwrap", makes probeFor
// report Available with that BinaryPath. This names the type AND exercises it
// through the production probe branch (rather than an anonymous closure).
func TestLookPathFunc_DrivesProbeForBranch(t *testing.T) {
	var look LookPathFunc = func(name string) (string, error) {
		if name == "bwrap" {
			return "/opt/bin/bwrap", nil
		}
		return "", fakeNotFound(name)
	}
	pr := probeFor("linux", look)
	if !pr.Available {
		t.Fatalf("probeFor with resolving LookPathFunc: Available=false, %+v", pr)
	}
	if pr.BinaryPath != "/opt/bin/bwrap" {
		t.Errorf("BinaryPath=%q, want /opt/bin/bwrap", pr.BinaryPath)
	}
}
