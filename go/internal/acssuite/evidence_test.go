package acssuite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_PartialExecutionCannotAuthorizeShip(t *testing.T) {
	v, err := Run(Options{Root: t.TempDir(), Cycle: 1, GoExec: func(_ context.Context, _, pattern string, _ []string) (string, error) {
		if pattern != "./acs/cycle1" {
			return "", nil
		}
		return `{"Action":"pass","Package":"fixture/acs/cycle1","Test":"TestPassed"}
{"Action":"run","Package":"fixture/acs/cycle1","Test":"TestInterrupted"}
`, errors.New("predicate process interrupted")
	}})
	if err == nil && v.ShipEligible {
		t.Fatal("partial execution with a passing prefix authorized ship")
	}
}

func evidenceFixture(t *testing.T) []byte {
	t.Helper()
	v := Verdict{SchemaVersion: "1.0", Cycle: 7, Verdict: "PASS", ShipEligible: true}
	v.record(Result{ACID: "cycle7/TestA", Predicate: "go/acs/cycle7/...:TestA", ResultStr: "green"})
	v.PredicateSuite.Total = 1
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestReadVerdict(t *testing.T) {
	raw := evidenceFixture(t)
	if _, err := ReadVerdict(raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"schema_version", "cycle", "results", "predicate_suite", "red_count", "ship_eligible"} {
		t.Run("missing_"+field, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			delete(m, field)
			changed, _ := json.Marshal(m)
			if _, err := ReadVerdict(changed); err == nil {
				t.Fatalf("missing %s authorized ship", field)
			}
		})
	}
	for _, mutate := range []func(*Verdict){
		func(v *Verdict) { v.Cycle = 0 },
		func(v *Verdict) { v.Results = nil },
		func(v *Verdict) { v.Results = append(v.Results, v.Results[0]) },
		func(v *Verdict) { v.RedCount = -1 },
		func(v *Verdict) { v.Results[0].ExitCode = 1 },
		func(v *Verdict) { v.Results[0].ResultStr = "unknown" },
		func(v *Verdict) { v.PredicateSuite.Total++ },
		func(v *Verdict) { v.ShipEligible = false },
	} {
		var v Verdict
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		mutate(&v)
		changed, _ := json.Marshal(v)
		if _, err := ReadVerdict(changed); err == nil {
			t.Fatalf("inconsistent verdict accepted: %s", changed)
		}
	}
}

func TestSealEvidence(t *testing.T) {
	raw := evidenceFixture(t)
	id := EvidenceIdentity{Cycle: 7, RunID: "run-7", Round: 2, TreeSHA: strings.Repeat("a", 40)}
	path := filepath.Join(t.TempDir(), "audit-report.md")
	if err := os.WriteFile(path, []byte("## Verdict\nPASS\n<!-- evolve-acs-evidence: {} -->"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SealEvidence(path, raw, id); err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyEvidence(string(report), raw, id); err != nil {
		t.Fatal(err)
	}
	if err := SealEvidence(path, raw, EvidenceIdentity{}); err == nil {
		t.Fatal("missing host identity accepted")
	}
	wrong := id
	wrong.Cycle++
	if err := SealEvidence(path, raw, wrong); err == nil {
		t.Fatal("foreign-cycle execution accepted")
	}
}

func TestVerifyEvidence(t *testing.T) {
	raw := evidenceFixture(t)
	id := EvidenceIdentity{Cycle: 7, RunID: "run-7", Round: 2, TreeSHA: strings.Repeat("a", 40)}
	path := filepath.Join(t.TempDir(), "audit-report.md")
	if err := os.WriteFile(path, []byte("## Verdict\nPASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SealEvidence(path, raw, id); err != nil {
		t.Fatal(err)
	}
	report, _ := os.ReadFile(path)
	for _, mutate := range []func(*EvidenceIdentity){
		func(v *EvidenceIdentity) { v.Cycle++ }, func(v *EvidenceIdentity) { v.RunID += "-other" },
		func(v *EvidenceIdentity) { v.Round++ }, func(v *EvidenceIdentity) { v.TreeSHA = strings.Repeat("b", 40) },
	} {
		wrong := id
		mutate(&wrong)
		if _, err := VerifyEvidence(string(report), raw, wrong); err == nil {
			t.Fatalf("wrong identity accepted: %+v", wrong)
		}
	}
	for _, altered := range []string{string(raw) + "\n", "{}", "null", "{broken"} {
		if _, err := VerifyEvidence(string(report), []byte(altered), id); err == nil {
			t.Fatal("altered verdict accepted")
		}
	}
	v, _ := ReadVerdict(raw)
	for _, altered := range []string{"", string(report) + string(report), strings.Replace(string(report), inventorySHA(v), "bad", 1)} {
		if _, err := VerifyEvidence(altered, raw, id); err == nil {
			t.Fatal("missing or duplicate receipt accepted")
		}
	}
}

func TestInvalidateEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit-report.md")
	raw := evidenceFixture(t)
	id := EvidenceIdentity{Cycle: 7, RunID: "run-7", Round: 1, TreeSHA: strings.Repeat("a", 40)}
	for _, candidate := range []string{"## Verdict\nPASS\n", "## Verdict\nPASS\n<!-- evolve-acs-evidence: malformed\n"} {
		if err := os.WriteFile(path, []byte(candidate), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := InvalidateEvidence(path); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "<!-- evolve-acs-evidence:") {
			t.Fatal("candidate retained host marker")
		}
		if err := SealEvidence(path, raw, id); err != nil {
			t.Fatal(err)
		}
		if err := InvalidateEvidence(path); err != nil {
			t.Fatal(err)
		}
		body, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyEvidence(string(body), raw, id); err == nil {
			t.Fatal("retired receipt still authorizes execution")
		}
	}
	if err := InvalidateEvidence(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("unreadable report retirement silently succeeded")
	}
}

func TestRun_PartialRetryCannotEraseRed(t *testing.T) {
	calls := 0
	v, err := Run(Options{Root: t.TempDir(), Cycle: 1, GoExec: func(_ context.Context, _, pattern string, _ []string) (string, error) {
		if pattern != "./acs/cycle1" {
			return "", nil
		}
		calls++
		if calls == 1 {
			return `{"Action":"fail","Package":"fixture/acs/cycle1","Test":"TestRequired"}`, errors.New("red")
		}
		return `{"Action":"pass","Package":"fixture/acs/cycle1","Test":"TestRequired"}`, errors.New("process interrupted after passing prefix")
	}})
	if err == nil && v.ShipEligible {
		t.Fatal("incomplete retry erased first-run red")
	}
}
