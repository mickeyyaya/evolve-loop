//go:build acs

package gatepins

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type symbolPin struct {
	path     string
	function string
	want     shape
	reason   string
}

var sandboxPins = []symbolPin{
	{"go/internal/bridge/driver_agy.go", "agyDriver.Launch", refusesUnconfinedLaunch, "the agy driver refuses an unconfined launch when the sandbox is required"},
	{"go/internal/bridge/driver_codex.go", "codexDriver.Launch", refusesUnconfinedLaunch, "the codex driver refuses an unconfined launch when the sandbox is required"},
	{"go/internal/bridge/engine.go", "Deps.withDefaults", installsDefaultSandboxWrap, "the engine installs the OS sandbox wrapper when the caller gives none"},
	{"go/internal/bridge/engine.go", "launchArgs", passesRequireSandbox, "the engine passes the sandbox requirement of the request to the launch"},
}

func TestSandboxPinsHoldInLaneEditableFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, pin := range sandboxPins {
		t.Run(filepath.Base(pin.path)+"/"+pin.function, func(t *testing.T) {
			ok, err := holds(filepath.Join(root, filepath.FromSlash(pin.path)), pin.function, pin.want)
			if err != nil {
				t.Fatalf("%s: %v", pin.path, err)
			}
			if !ok {
				t.Errorf("%s %s lost its pinned shape: %s (ADR-0157). Restore it, or change this pin through an operator-gated evolve ship --class manual.", pin.path, pin.function, pin.reason)
			}
		})
	}
}

func TestSandboxPinsGuardOnlyFilesOutsideTheFence(t *testing.T) {
	for _, pin := range sandboxPins {
		if guards.IsProtectedSurface(pin.path) {
			t.Errorf("%s is a protected file; a symbol pin is for a file that lanes can change, so remove the pin or the manifest entry", pin.path)
		}
	}
}

var refusalSymbols = []string{"sandboxRequiredButUnavailable", "validateEnvPassthrough", "WithProtectedPath"}

func TestRefusalCallSitesAreFencedOrPinned(t *testing.T) {
	root := acsassert.RepoRoot(t)
	pinned := map[string]bool{}
	for _, pin := range sandboxPins {
		pinned[pin.path] = true
	}
	for _, dir := range []string{"go/internal", "go/cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			names, err := referencedNames(path)
			if err != nil {
				return err
			}
			if hit := firstOf(names, refusalSymbols); hit != "" && !guards.IsProtectedSurface(rel) && !pinned[rel] {
				t.Errorf("%s references the refusal %s but is neither on the fence nor pinned (ADR-0157 E5)", rel, hit)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
