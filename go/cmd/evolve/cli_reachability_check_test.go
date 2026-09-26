package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// writeFixtureModule writes a real module with packages core and storage, so
// the import graph comes from the toolchain rather than a hand-built literal.
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

func TestReachabilityCommand_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runReachability([]string{"bogus-subcommand"}, nil, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("runReachability with unknown subcommand = exit 0, want non-zero")
	}
}
