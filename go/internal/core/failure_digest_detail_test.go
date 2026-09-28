package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func digestOf(t *testing.T, reason string) FailureDigest {
	t.Helper()
	ws := t.TempDir()
	b, err := json.Marshal(auditFailReason{SchemaVersion: 1, Phase: "audit", Reasons: []string{reason}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := AssembleFailureDigest(1, ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestAssembleFailureDigest_AReasonsDetailNeverChangesItsIdentity(t *testing.T) {
	refusal := "host predicate execution: predicate execution tree includes undeclared inputs absent from the ship tree; explicitly stage intended Build files or remove the inputs, then re-run Audit"
	bare := digestOf(t, refusal)
	for _, detail := range []string{
		"go/acs/cycle1694/predicates_test.go, .evolve/evals/atomicwrite-linked-state-sweep.md",
		".evolve/evals/statemap-export-resolve-write-target.md, go/internal/bridge/x.go",
	} {
		d := digestOf(t, cyclestate.WithDetail(refusal, detail))
		if d.Fingerprint != bare.Fingerprint || d.PreClass != bare.PreClass {
			t.Errorf("detail %q moved the identity: fingerprint %s/%s class %s/%s", detail, d.Fingerprint, bare.Fingerprint, d.PreClass, bare.PreClass)
		}
	}
}

func TestNormalizeReasonForFingerprint_ContractBlocksShareAnIdentityAcrossTheirDetail(t *testing.T) {
	reason := "contract gate blocked build"
	if normalizeReasonForFingerprint(cyclestate.WithDetail(reason, "a.go, b.go")) != normalizeReasonForFingerprint(reason) {
		t.Fatal("a contract block's identity ignores its detail, so the escalation counts repeats")
	}
}
