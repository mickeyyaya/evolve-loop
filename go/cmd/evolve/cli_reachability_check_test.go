// cli_reachability_check_test.go pins the `evolve reachability check-pin`
// wiring gap: reachabilityprobe.BuildImportGraph/CheckCallSite (cycle-1226)
// has zero non-test callers repo-wide, so cycle-644's failure mode (freezing
// a doNotModifyTests:true structural test pin that is an unbuildable import
// cycle) remains fully reproducible today. This test drives the real
// production caller — runReachability, the function wired into the
// dispatcher `commands` table (registry_test asserts the wiring separately)
// — against fixture Go modules built from the REAL toolchain, not a
// hand-built literal ImportGraph (that round-trip is already covered by
// go/acs/cycle1226; this cycle's job is proving the CLI seam exists at all).
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReachabilitySubcommandRegistered — `reachability` must be wired into
// the dispatcher table (an unregistered command is dead code the CLI can
// never route to; cf. TestCarryoverSubcommandRegistered).
func TestReachabilitySubcommandRegistered(t *testing.T) {
	found := false
	for _, c := range commands {
		if c.Name == "reachability" {
			found = true
			if c.Run == nil {
				t.Fatalf("reachability command registered with a nil Run handler")
			}
		}
	}
	if !found {
		t.Fatalf("reachability subcommand is not registered in the commands table")
	}
}

// writeFixtureModule creates a throwaway Go module on disk with two leaf
// packages, "core" and "storage". When storageImportsCore is true, storage's
// source imports core (a real, toolchain-verifiable edge) — the exact
// cycle-644 shape when core is later pinned to call into storage. When
// false, the two packages are mutually unaware (the false-positive guard
// case). Returns the module root and the two packages' full import paths.
func writeFixtureModule(t *testing.T, storageImportsCore bool) (root, corePkg, storagePkg string) {
	t.Helper()
	root = t.TempDir()

	const modulePath = "reachabilityfixture"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	coreDir := filepath.Join(root, "core")
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		t.Fatalf("mkdir core: %v", err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "core.go"), []byte("package core\n\nfunc Y() {}\n"), 0o644); err != nil {
		t.Fatalf("write core.go: %v", err)
	}

	storageDir := filepath.Join(root, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatalf("mkdir storage: %v", err)
	}
	storageSrc := "package storage\n\nfunc X() {}\n"
	if storageImportsCore {
		storageSrc = "package storage\n\nimport \"" + modulePath + "/core\"\n\nfunc X() { core.Y() }\n"
	}
	if err := os.WriteFile(filepath.Join(storageDir, "storage.go"), []byte(storageSrc), 0o644); err != nil {
		t.Fatalf("write storage.go: %v", err)
	}

	return root, modulePath + "/core", modulePath + "/storage"
}

// TestReachabilityCheckPin_CyclicFixture_DetectsViolation is the primary
// positive case (the cycle-644 shape): storage really does import core, so
// pinning core.UpdateStateMap( inside a storage-package file would create an
// import cycle. runReachability must exit non-zero and print a message that
// matches reachabilityprobe.Violation.Error()'s format (names the referenced
// symbol, the pinning package, and "import cycle").
func TestReachabilityCheckPin_CyclicFixture_DetectsViolation(t *testing.T) {
	root, corePkg, storagePkg := writeFixtureModule(t, true)

	var stdout, stderr bytes.Buffer
	rc := runReachability([]string{
		"check-pin",
		"--root", root,
		"--pinning-package", corePkg,
		"--referenced-package", storagePkg,
		"--symbol", "UpdateStateMap",
		"--pkgs", "./core,./storage",
	}, nil, &stdout, &stderr)

	if rc == 0 {
		t.Fatalf("runReachability check-pin on a cyclic fixture (storage imports core) = exit 0, want non-zero; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	combined := stdout.String() + stderr.String()
	for _, want := range []string{storagePkg, "UpdateStateMap", corePkg, "import cycle"} {
		if !strings.Contains(combined, want) {
			t.Errorf("runReachability check-pin output = %q, want it to contain %q (Violation.Error() format)", combined, want)
		}
	}
}

// TestReachabilityCheckPin_AcyclicFixture_NoFalsePositive is the negative
// counterpart (adversarial-testing SKILL.md §6): two packages that share no
// import edge must NOT be flagged. A stub that always reports a violation
// would fail this immediately.
func TestReachabilityCheckPin_AcyclicFixture_NoFalsePositive(t *testing.T) {
	root, corePkg, storagePkg := writeFixtureModule(t, false)

	var stdout, stderr bytes.Buffer
	rc := runReachability([]string{
		"check-pin",
		"--root", root,
		"--pinning-package", corePkg,
		"--referenced-package", storagePkg,
		"--symbol", "UpdateStateMap",
		"--pkgs", "./core,./storage",
	}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("runReachability check-pin on an acyclic fixture (no import edge) = exit %d, want 0; stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
}

// TestReachabilityCheckPin_MissingRequiredFlag is the malformed-input edge
// case: omitting a required flag must fail loudly (non-zero exit), never
// silently proceed with an empty package name that can never match anything.
func TestReachabilityCheckPin_MissingRequiredFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runReachability([]string{
		"check-pin",
		"--referenced-package", "storage",
		"--symbol", "UpdateStateMap",
		"--pkgs", "./core,./storage",
	}, nil, &stdout, &stderr)

	if rc == 0 {
		t.Fatalf("runReachability check-pin with --pinning-package omitted = exit 0, want non-zero")
	}
}

// TestReachabilityCommand_UnknownSubcommand — an unrecognized subcommand
// under `reachability` must fail loudly rather than silently no-op.
func TestReachabilityCommand_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runReachability([]string{"bogus-subcommand"}, nil, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("runReachability with unknown subcommand = exit 0, want non-zero")
	}
}
