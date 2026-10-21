package dossier

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreClass_TheCommittedClassNamesRoundTripThroughTheFailureRecord(t *testing.T) {
	for class, wire := range map[string]string{
		PreClassInfraError: "infra-error", PreClassGuardAbort: "guard-abort", PreClassGateBlock: "gate-block",
		PreClassVerdictFail: "verdict-fail", PreClassUnknown: "unknown",
	} {
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, failureDigestFile), []byte(`{"cycle":3,"fingerprint":"f","pre_class":"`+wire+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		rec, ok := failureRecord(ws, 3)
		if !ok || rec.PreClass != class {
			t.Errorf("a digest with pre_class %q gives %+v, %v; want the constant %q", wire, rec, ok, class)
		}
	}
}
