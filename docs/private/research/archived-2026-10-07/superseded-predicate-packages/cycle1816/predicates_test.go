//go:build acs

package cycle1816

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const premiumSpecName = "violatesPremiumPlacement"

const premiumSpecTableTest = "TestViolatesPremiumPlacement"

const placementGuardPattern = `^Test\w*EffortOnlyOnDeepOrTopProfiles$`

const specProbeOverlaySource = `package profiles

import "testing"

func TestACSPremiumPlacementSpecProbe(t *testing.T) {
	var spec func(Profile) bool = violatesPremiumPlacement
	cases := []struct {
		cli, tier, effort string
		want              bool
	}{
		{"codex-tmux", "balanced", "ultra", true},
		{"codex-tmux", "", "ultra", true},
		{"codex-tmux", "fast", "ultra", true},
		{"codex-tmux", "balanced", "max", true},
		{"codex-tmux", "fast", "max", true},
		{"claude-tmux", "balanced", "max", true},
		{"claude-tmux", "", "max", true},
		{"codex-tmux", "deep", "ultra", false},
		{"codex-tmux", "top", "ultra", false},
		{"codex-tmux", "top", "max", false},
		{"claude-tmux", "deep", "max", false},
		{"codex-tmux", "balanced", "xhigh", false},
		{"codex-tmux", "balanced", "high", false},
		{"codex-tmux", "fast", "low", false},
		{"codex-tmux", "balanced", "", false},
		{"codex-tmux", "deep", "high", false},
	}
	for _, c := range cases {
		p := Profile{Name: "acs-probe", Role: "acs-probe", CLI: c.cli, ModelTierDefault: c.tier, EffortLevel: c.effort}
		if got := spec(p); got != c.want {
			t.Errorf("violatesPremiumPlacement(cli=%s tier=%q effort=%q) = %v, want %v", c.cli, c.tier, c.effort, got, c.want)
		}
	}
}
`

type probeProfile struct {
	name, cli, tier, effort string
}

var premiumViolators = []probeProfile{
	{"probe-balanced-ultra", "codex-tmux", "balanced", "ultra"},
	{"probe-unset-ultra", "codex-tmux", "", "ultra"},
	{"probe-fast-max", "codex-tmux", "fast", "max"},
	{"probe-claude-balanced-max", "claude-tmux", "balanced", "max"},
}

var premiumCompliant = []probeProfile{
	{"probe-deep-ultra", "codex-tmux", "deep", "ultra"},
	{"probe-top-max", "codex-tmux", "top", "max"},
	{"probe-balanced-xhigh", "codex-tmux", "balanced", "xhigh"},
	{"probe-balanced-high", "codex-tmux", "balanced", "high"},
}

func goModuleDir(t *testing.T) (string, string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	return root, filepath.Join(root, "go")
}

func runProfilesTests(t *testing.T, goDir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"test", "-count=1", "-v"}, append(args, "./internal/profiles")...)...)
	cmd.Dir = goDir
	cmd.Env = append(os.Environ(), "PWD="+goDir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("go test did not run in %s: %v\n%s", goDir, err, out)
	return "", -1
}

func tail(out string) string {
	const keep = 4000
	if len(out) <= keep {
		return out
	}
	return "…" + out[len(out)-keep:]
}

func writeOverlay(t *testing.T, replace map[string]string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func shadowRepoRoot(t *testing.T, root string, probes []probeProfile) string {
	t.Helper()
	shadow := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "go"), filepath.Join(shadow, "go")); err != nil {
		t.Fatal(err)
	}
	shadowProfiles := filepath.Join(shadow, ".evolve", "profiles")
	if err := os.MkdirAll(shadowProfiles, 0o755); err != nil {
		t.Fatal(err)
	}
	realProfiles, err := filepath.Glob(filepath.Join(root, ".evolve", "profiles", "*.json"))
	if err != nil || len(realProfiles) == 0 {
		t.Fatalf("no tracked profiles under %s: %v", root, err)
	}
	for _, src := range realProfiles {
		raw, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(shadowProfiles, filepath.Base(src)), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range probes {
		doc := map[string]string{"name": p.name, "role": "scout", "cli": p.cli, "effort_level": p.effort}
		if p.tier != "" {
			doc["model_tier_default"] = p.tier
		}
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(shadowProfiles, p.name+".json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(shadow, "go")
}

func premiumSpecDecl(t *testing.T, pkgDir string) (string, int) {
	t.Helper()
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(pkgDir, e.Name())
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.Name == premiumSpecName {
				return path, fset.Position(fn.Name.Pos()).Offset
			}
		}
	}
	t.Fatalf("RED: no func %s declared in package profiles (%s) — the placement law has no extracted Specification", premiumSpecName, pkgDir)
	return "", 0
}

func mutatedSpecOverlay(t *testing.T, realGoDir, overlayGoDir, verdict string) string {
	t.Helper()
	specFile, nameOffset := premiumSpecDecl(t, filepath.Join(realGoDir, "internal", "profiles"))
	src, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	const renamed = premiumSpecName + "UnderMutation"
	mutant := string(src[:nameOffset]) + renamed + string(src[nameOffset+len(premiumSpecName):]) +
		"\nfunc " + premiumSpecName + "(p Profile) bool {\n\t_ = " + renamed + "\n\treturn " + verdict + "\n}\n"
	mutantPath := filepath.Join(t.TempDir(), filepath.Base(specFile))
	if err := os.WriteFile(mutantPath, []byte(mutant), 0o644); err != nil {
		t.Fatal(err)
	}
	return writeOverlay(t, map[string]string{
		filepath.Join(overlayGoDir, "internal", "profiles", filepath.Base(specFile)): mutantPath,
	})
}

func TestC1816_001_PremiumSpecReservesMaxAndUltraForDeepOrTop(t *testing.T) {
	_, goDir := goModuleDir(t)
	probeSource := filepath.Join(t.TempDir(), "probe_test.go")
	if err := os.WriteFile(probeSource, []byte(specProbeOverlaySource), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := writeOverlay(t, map[string]string{
		filepath.Join(goDir, "internal", "profiles", "zz_acs_c1816_premium_spec_probe_test.go"): probeSource,
	})
	out, code := runProfilesTests(t, goDir, "-overlay", overlay, "-run", "^TestACSPremiumPlacementSpecProbe$")
	if code != 0 || !strings.Contains(out, "--- PASS: TestACSPremiumPlacementSpecProbe") {
		t.Errorf("RED: func %s(Profile) bool must flag max and ultra on a fast, balanced or unset tier and nothing else (exit %d):\n%s", premiumSpecName, code, tail(out))
	}
}

func TestC1816_002_TreeGuardRejectsUltraOnBalancedThroughTheSpec(t *testing.T) {
	root, realGoDir := goModuleDir(t)

	violatingGoDir := shadowRepoRoot(t, root, append(append([]probeProfile{}, premiumViolators...), premiumCompliant...))
	out, code := runProfilesTests(t, violatingGoDir, "-run", placementGuardPattern)
	if !strings.Contains(out, "=== RUN") {
		t.Fatalf("RED: no test matching %s ran over the shadow corpus:\n%s", placementGuardPattern, tail(out))
	}
	if code == 0 {
		t.Errorf("RED: the placement guard passed a profile tree holding premium rungs on fast/balanced/unset tiers:\n%s", tail(out))
	}
	for _, v := range premiumViolators {
		if !strings.Contains(out, v.name) {
			t.Errorf("RED: the placement guard never named %s (cli %s, tier %q, effort %q)", v.name, v.cli, v.tier, v.effort)
		}
	}

	compliantGoDir := shadowRepoRoot(t, root, premiumCompliant)
	if out, code := runProfilesTests(t, compliantGoDir, "-run", placementGuardPattern); code != 0 || !strings.Contains(out, "--- PASS:") {
		t.Errorf("RED: the placement guard failed a tree whose premium rungs sit only on deep/top (exit %d):\n%s", code, tail(out))
	}

	neverViolates := mutatedSpecOverlay(t, realGoDir, violatingGoDir, "false")
	if out, code := runProfilesTests(t, violatingGoDir, "-overlay", neverViolates, "-run", placementGuardPattern); code != 0 || !strings.Contains(out, "--- PASS:") {
		t.Errorf("RED: with %s forced to false the guard still failed — its verdict does not come from the Specification (exit %d):\n%s", premiumSpecName, code, tail(out))
	}
}

func TestC1816_003_SpecTableTestRunsInTheSuiteAndKillsMutants(t *testing.T) {
	_, goDir := goModuleDir(t)
	out, code := runProfilesTests(t, goDir, "-run", "^"+premiumSpecTableTest+"$")
	if code != 0 || !strings.Contains(out, "--- PASS: "+premiumSpecTableTest) {
		t.Fatalf("RED: %s must exist in the untagged profiles suite and pass (exit %d):\n%s", premiumSpecTableTest, code, tail(out))
	}
	for _, verdict := range []string{"false", "true"} {
		overlay := mutatedSpecOverlay(t, goDir, goDir, verdict)
		out, code := runProfilesTests(t, goDir, "-overlay", overlay, "-run", "^"+premiumSpecTableTest+"$")
		if code == 0 {
			t.Errorf("RED: %s passed with %s forced to return %s — the table cannot tell a vacuous law from the real one", premiumSpecTableTest, premiumSpecName, verdict)
			continue
		}
		if !strings.Contains(out, "--- FAIL: "+premiumSpecTableTest) {
			t.Errorf("RED: the %s mutant did not build, so the table test never judged it (exit %d):\n%s", verdict, code, tail(out))
		}
	}
}
