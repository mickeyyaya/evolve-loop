//go:build acs

package cycle1795

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
)

var repoContractSuites = []string{
	"./internal/profiles/...",
	"./internal/phasecoherence/...",
	"./internal/phasespec/...",
}

func preflightRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	auditRel := filepath.Join(".evolve", "runs", "cycle-99", "audit-report.md")
	files := map[string]string{
		filepath.Join(".claude-plugin", "plugin.json"): `{"name":"x","version":"1.0.0"}`,
		auditRel: "# Audit\n\nVerdict: PASS\n\nConfidence: 1.0\n",
		filepath.Join(".evolve", "ledger.jsonl"): fmt.Sprintf(
			`{"ts":"%s","cycle":99,"role":"auditor","kind":"agent_subprocess","model":"opus","exit_code":0,"artifact_path":"%s","artifact_sha256":"deadbeef","git_head":"none","tree_state_sha":"none"}`+"\n",
			time.Now().UTC().Format(time.RFC3339), filepath.Join(root, auditRel)),
	}
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runWithRunner(t *testing.T, runner func(string, string) error) error {
	t.Helper()
	_, err := releasepreflight.Run(releasepreflight.Options{
		Target:           "1.0.1",
		RepoRoot:         preflightRepo(t),
		Now:              time.Now,
		GitClean:         func(string) (bool, error) { return true, nil },
		CurrentBranch:    func(string) (string, error) { return "main", nil },
		GateTestRunner:   runner,
		SimulationRunner: func(string) error { return nil },
	})
	return err
}

func TestC1795_001_PreflightRunsRepoContractSuites(t *testing.T) {
	ran := map[string]bool{}
	err := runWithRunner(t, func(_ string, suite string) error { ran[suite] = true; return nil })
	if err != nil {
		t.Fatalf("preflight with all-green runner failed: %v", err)
	}
	for _, want := range repoContractSuites {
		if !ran[want] {
			t.Errorf("gate runner never invoked for %s (ran: %v)", want, ran)
		}
	}
}

func TestC1795_002_RedRepoContractSuiteFailsPreflight(t *testing.T) {
	for _, red := range repoContractSuites {
		err := runWithRunner(t, func(_ string, suite string) error {
			if suite == red {
				return errors.New("suite red")
			}
			return nil
		})
		if !errors.Is(err, releasepreflight.ErrCheckFailed) {
			t.Errorf("red %s: err = %v, want ErrCheckFailed", red, err)
			continue
		}
		if !strings.Contains(err.Error(), red) {
			t.Errorf("red %s: error does not name the failing suite: %v", red, err)
		}
	}
}

func TestC1795_003_ExistingSuitesStillGate(t *testing.T) {
	for _, red := range []string{"./internal/guards/...", "./internal/phases/ship/..."} {
		err := runWithRunner(t, func(_ string, suite string) error {
			if suite == red {
				return errors.New("suite red")
			}
			return nil
		})
		if !errors.Is(err, releasepreflight.ErrCheckFailed) {
			t.Errorf("red %s: err = %v, want ErrCheckFailed", red, err)
		}
	}
}

func TestC1795_004_GateCountMatchesSuiteList(t *testing.T) {
	calls := 0
	res, err := releasepreflight.Run(releasepreflight.Options{
		Target:           "1.0.1",
		RepoRoot:         preflightRepo(t),
		Now:              time.Now,
		GitClean:         func(string) (bool, error) { return true, nil },
		CurrentBranch:    func(string) (string, error) { return "main", nil },
		GateTestRunner:   func(string, string) error { calls++; return nil },
		SimulationRunner: func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls < 5 || res.GateTestsPassed != calls {
		t.Errorf("calls=%d GateTestsPassed=%d, want >=5 suites all counted", calls, res.GateTestsPassed)
	}
}
