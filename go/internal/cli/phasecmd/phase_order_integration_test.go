//go:build integration

package phasecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseorder"
)

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Errorf("restore cwd %s: %v", prev, err)
		}
	})
}

func writeRegistry(t *testing.T, root, body string) {
	t.Helper()
	path := config.RegistryPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runPhaseOrder(t *testing.T) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut strings.Builder
	code = RunPhaseOrder(nil, nil, &out, &errOut)
	return code, out.String(), errOut.String()
}

const twoPhaseRegistry = `{"phases":[{"name":"scout"},{"name":""},{"name":"ship"}]}`

func TestRunPhaseOrder_ResolvesTheRegistryRoot(t *testing.T) {
	t.Setenv("EVOLVE_USE_PHASE_REGISTRY", "")
	t.Setenv("GIT_CEILING_DIRECTORIES", "")
	envRoot := t.TempDir()
	writeRegistry(t, envRoot, twoPhaseRegistry)
	repo := gittest.Fixture(t)
	writeRegistry(t, repo.Dir, `{"phases":[{"name":"from-git-toplevel"}]}`)
	plain := t.TempDir()
	writeRegistry(t, plain, `{"phases":[{"name":"from-cwd"}]}`)
	parent := t.TempDir()
	writeRegistry(t, filepath.Join(parent, "proj"), `{"phases":[{"name":"from-relative-env"}]}`)

	for _, tc := range []struct {
		name, envRoot, cwd, want string
	}{
		{"absolute env root", envRoot, repo.Dir, "scout\nship\n"},
		{"relative env root against cwd", "proj", parent, "from-relative-env\n"},
		{"git toplevel from a subdirectory", "", filepath.Join(repo.Dir, "docs", "architecture"), "from-git-toplevel\n"},
		{"cwd outside any repo", "", plain, "from-cwd\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EVOLVE_PROJECT_ROOT", tc.envRoot)
			t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(plain))
			chdirForTest(t, tc.cwd)

			code, stdout, stderr := runPhaseOrder(t)

			if code != 0 || stdout != tc.want || stderr != "" {
				t.Fatalf("RunPhaseOrder = %d, stdout %q, stderr %q; want 0, %q, no stderr", code, stdout, stderr, tc.want)
			}
		})
	}
}

func TestRunPhaseOrder_RegistryOffSwitchPrintsTheHardcodedOrder(t *testing.T) {
	root := t.TempDir()
	writeRegistry(t, root, `not json`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv("EVOLVE_USE_PHASE_REGISTRY", "0")

	code, stdout, stderr := runPhaseOrder(t)

	want := strings.Join(phaseorder.HardcodedOrder, "\n") + "\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Fatalf("RunPhaseOrder = %d, %q, %q; want 0 and the hardcoded order", code, stdout, stderr)
	}
}

func TestRunPhaseOrder_OnlyTheExactZeroTurnsTheRegistryOff(t *testing.T) {
	for _, value := range []string{"1", "false", "00", " 0"} {
		t.Run(value, func(t *testing.T) {
			root := t.TempDir()
			writeRegistry(t, root, twoPhaseRegistry)
			t.Setenv("EVOLVE_PROJECT_ROOT", root)
			t.Setenv("EVOLVE_USE_PHASE_REGISTRY", value)

			code, stdout, _ := runPhaseOrder(t)

			if code != 0 || stdout != "scout\nship\n" {
				t.Fatalf("EVOLVE_USE_PHASE_REGISTRY=%q: %d, %q; want the registry order", value, code, stdout)
			}
		})
	}
}

func TestRunPhaseOrder_MissingRegistryFallsBackToTheHardcodedOrder(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	t.Setenv("EVOLVE_USE_PHASE_REGISTRY", "")

	code, stdout, _ := runPhaseOrder(t)

	if code != 0 || stdout != strings.Join(phaseorder.HardcodedOrder, "\n")+"\n" {
		t.Fatalf("RunPhaseOrder = %d, %q; want the hardcoded order", code, stdout)
	}
}

func TestRunPhaseOrder_UnreadableRegistryIsExitOneWithTheError(t *testing.T) {
	for name, body := range map[string]string{
		"malformed": `{"phases":`,
		"empty":     `{"phases":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeRegistry(t, root, body)
			t.Setenv("EVOLVE_PROJECT_ROOT", root)
			t.Setenv("EVOLVE_USE_PHASE_REGISTRY", "")

			code, stdout, stderr := runPhaseOrder(t)

			if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "[phase-order] ERROR: ") || !strings.Contains(stderr, config.RegistryPath(root)) {
				t.Fatalf("RunPhaseOrder = %d, %q, %q; want exit 1 and an ERROR naming the registry", code, stdout, stderr)
			}
		})
	}
}

func TestRunPhaseOrder_UnresolvableEnvRootWarnsAndKeepsTheRawRoot(t *testing.T) {
	saved := absoluteRoot
	t.Cleanup(func() { absoluteRoot = saved })
	var gotLabel, gotRoot string
	absoluteRoot = func(label, p string, warn func(string)) string {
		gotLabel, gotRoot = label, p
		warn("getwd failed")
		return p
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", "rel-root")
	t.Setenv("EVOLVE_USE_PHASE_REGISTRY", "")

	code, stdout, stderr := runPhaseOrder(t)

	if gotLabel != "EVOLVE_PROJECT_ROOT" || gotRoot != "rel-root" {
		t.Fatalf("absoluteRoot(%q, %q), want the env label and the raw root", gotLabel, gotRoot)
	}
	if code != 0 || stdout != strings.Join(phaseorder.HardcodedOrder, "\n")+"\n" {
		t.Fatalf("RunPhaseOrder = %d, %q; want the hardcoded fallback", code, stdout)
	}
	if stderr != "[phase-order] WARN: getwd failed\n" {
		t.Fatalf("stderr = %q, want the prefixed WARN line", stderr)
	}
}

func TestRunPhaseOrder_HelpPrintsUsageWithoutResolvingARoot(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	chdirForTest(t, t.TempDir())
	for _, args := range [][]string{{"-h"}, {"--help"}, {"extra", "--help"}} {
		var out, errOut strings.Builder

		code := RunPhaseOrder(args, nil, &out, &errOut)

		want := "Usage: evolve phase-order\nEnv: EVOLVE_USE_PHASE_REGISTRY=0 forces hardcoded order\n"
		if code != 0 || out.String() != want || errOut.String() != "" {
			t.Fatalf("RunPhaseOrder(%q) = %d, %q, %q; want 0 and the usage", args, code, out.String(), errOut.String())
		}
	}
}
