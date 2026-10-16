package sandbox

import (
	"testing"
)

func TestBwrapPrefix_PrefixOnlyAndComposesGenerate(t *testing.T) {
	cfg := canonicalConfig()
	prefix := BwrapPrefix(cfg)

	mustContainArg(t, prefix, "--unshare-user")
	mustContainArg(t, prefix, "--bind", "/repo", "/repo")
	for _, a := range prefix {
		if a == "claude" || a == "true" {
			t.Errorf("BwrapPrefix leaked an inner-argv token %q: %v", a, prefix)
		}
	}

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
