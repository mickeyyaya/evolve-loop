//go:build acs

package cycle1833

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

const (
	releasepreflightPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
	namingStepIndex        = 4
	allStepsPassed         = 5
	simulationFailureCause = "bridge simulation failed"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func releaseRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	pluginJSON := filepath.Join(repoRoot, ".claude-plugin", "plugin.json")
	if err := os.MkdirAll(filepath.Dir(pluginJSON), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginJSON, []byte(`{"name":"x","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return repoRoot
}

func preflightOptions(repoRoot string, log *bytes.Buffer) releasepreflight.Options {
	return releasepreflight.Options{
		Target:           "1.0.1",
		RepoRoot:         repoRoot,
		LedgerPath:       filepath.Join(repoRoot, "absent-ledger.jsonl"),
		Stderr:           log,
		Now:              time.Now,
		GitClean:         func(string) (bool, error) { return true, nil },
		CurrentBranch:    func(string) (string, error) { return "main", nil },
		HeadSHA:          func(string) (string, error) { return "", nil },
		GateTestRunner:   func(string, string) error { return nil },
		SimulationRunner: func(string) error { return nil },
		CIConclusion: func(string) (releasepreflight.CIRunStatus, error) {
			return releasepreflight.CIRunStatus{Conclusion: "success"}, nil
		},
	}
}

func lockManifestDirectory(t *testing.T, manifestPath string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	if err := os.WriteFile(manifestPath, []byte(`{"forbidden":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(manifestPath)
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Errorf("restore %s: %v", dir, err)
		}
	})
}

func loopManifestSymlink(t *testing.T, manifestPath string) {
	t.Helper()
	if err := os.Symlink(filepath.Base(manifestPath), manifestPath); err != nil {
		t.Fatal(err)
	}
}

func TestC1833_001_UnreadableNamingManifestFailsPreflightNamingStep(t *testing.T) {
	for _, tc := range []struct {
		name           string
		makeUnreadable func(t *testing.T, manifestPath string)
	}{
		{"manifest directory without search permission", lockManifestDirectory},
		{"manifest is a symlink loop", loopManifestSymlink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := releaseRepo(t)
			manifestPath := filepath.Join(repoRoot, naminguard.DefaultManifestPath)
			if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
				t.Fatal(err)
			}
			tc.makeUnreadable(t, manifestPath)
			var log bytes.Buffer
			res, err := releasepreflight.Run(preflightOptions(repoRoot, &log))
			if !errors.Is(err, releasepreflight.ErrCheckFailed) {
				t.Fatalf("Run err = %v; want ErrCheckFailed: an unreadable naming manifest must fail the naming step, not pass as absent\nlog=%s", err, log.String())
			}
			if res.StepsPassed != namingStepIndex {
				t.Errorf("StepsPassed = %d; want %d (the gate-suite step that runs the naming guard fails)", res.StepsPassed, namingStepIndex)
			}
			if msg := err.Error(); !strings.Contains(msg, "naming guard error") || !strings.Contains(msg, filepath.Base(manifestPath)) {
				t.Errorf("err = %v; want a naming-guard error that names %s", err, filepath.Base(manifestPath))
			}
		})
	}
}

func TestC1833_002_MissingNamingManifestPassesPreflight(t *testing.T) {
	var log bytes.Buffer
	res, err := releasepreflight.Run(preflightOptions(releaseRepo(t), &log))
	if err != nil {
		t.Fatalf("Run err = %v; want nil: a repo without a naming manifest has nothing to guard\nlog=%s", err, log.String())
	}
	if res.StepsPassed != allStepsPassed {
		t.Errorf("StepsPassed = %d; want %d", res.StepsPassed, allStepsPassed)
	}
	if !strings.Contains(log.String(), "OK: no dead naming tokens") {
		t.Errorf("log lacks the naming step's clean pass\nlog=%s", log.String())
	}
}

func TestC1833_003_CILookupFailureIsTheUnavailableVerdictNotAnError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, repoRoot string)
	}{
		{"git and gh absent from PATH", func(t *testing.T, _ string) { t.Setenv("PATH", t.TempDir()) }},
		{"not a git repository", func(t *testing.T, repoRoot string) {
			t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(repoRoot))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := releaseRepo(t)
			tc.setup(t, repoRoot)
			var log bytes.Buffer
			opts := preflightOptions(repoRoot, &log)
			opts.CIConclusion = nil
			res, err := releasepreflight.Run(opts)
			if err != nil {
				t.Fatalf("Run err = %v; want nil: absent CI tooling must not block a release\nlog=%s", err, log.String())
			}
			if res.CIConclusion != "" {
				t.Errorf("CIConclusion = %q; want the unavailable verdict \"\"", res.CIConclusion)
			}
			if !strings.Contains(log.String(), "remote CI conclusion unavailable") {
				t.Errorf("log lacks the unavailable advisory\nlog=%s", log.String())
			}
		})
	}
}

func TestC1833_004_FailOpenRulesArePinnedByNamedUnitTests(t *testing.T) {
	names := []string{
		"TestDefaultNameGuard_MissingManifestIsCleanPass",
		"TestDefaultCIConclusion_LookupFailuresAreUnavailableNotErrors",
		"TestDefaultNameGuard_StatErrorsOtherThanNotExistFail",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: releasepreflightPkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

// acs-predicate: config-check
func TestC1833_005_ReleasepreflightSourceCarriesNoWhyComments(t *testing.T) {
	for _, file := range []string{"releasepreflight.go", "preflight_run.go"} {
		path := filepath.Join(moduleRoot(t), "internal", "releasepreflight", file)
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, group := range parsed.Comments {
			if group == parsed.Doc {
				continue
			}
			t.Errorf("%s: comment %q; want none besides the package doc (the fail-open rules are pinned by tests)",
				fset.Position(group.Pos()), strings.TrimSpace(group.Text()))
		}
	}
}

func TestC1833_006_SimulationFailureWarningCarriesNoStaleRoadmapVersions(t *testing.T) {
	var log bytes.Buffer
	opts := preflightOptions(releaseRepo(t), &log)
	opts.SimulationRunner = func(string) error { return errors.New(simulationFailureCause) }
	res, err := releasepreflight.Run(opts)
	if err != nil {
		t.Fatalf("Run err = %v; want nil: the simulation suite is advisory\nlog=%s", err, log.String())
	}
	if res.SimulationAdvisoryOK == nil || *res.SimulationAdvisoryOK {
		t.Errorf("SimulationAdvisoryOK = %v; want false", res.SimulationAdvisoryOK)
	}
	if want := "WARN: auto-respond simulation suite failed (advisory): " + simulationFailureCause; !strings.Contains(log.String(), want) {
		t.Errorf("log lacks %q\nlog=%s", want, log.String())
	}
	for _, stale := range []string{"v12.1.5", "v12.2.0"} {
		if strings.Contains(log.String(), stale) {
			t.Errorf("log still names the stale roadmap version %s\nlog=%s", stale, log.String())
		}
	}
}
