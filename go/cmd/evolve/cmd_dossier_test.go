package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func writeDossierFloorPolicy(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const js = `{"version":1,"floor":[{"id":"dossier-closeout","enforced_since_cycle":2}]}`
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDossierVerify_FloorEnrolledButAbsent_Fails is the Potemkin-fix: when the
// policy floor enrolls "dossier-closeout" but no dossiers exist, verify must
// FAIL — previously it returned OK on an absent dir, so the declared gate
// enforced nothing.
func TestDossierVerify_FloorEnrolledButAbsent_Fails(t *testing.T) {
	root := t.TempDir()
	writeDossierFloorPolicy(t, root)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	if rc := runDossierVerify(nil, &out, &errb); rc == 0 {
		t.Errorf("verify must FAIL when floor enrolls dossier-closeout but no dossiers exist; got rc=0\nstderr=%s", errb.String())
	}
}

// TestDossierVerify_NoFloor_AbsentIsOK keeps back-compat: with no floor
// enrollment an absent dir is a no-op success (safe to run mid-batch).
func TestDossierVerify_NoFloor_AbsentIsOK(t *testing.T) {
	root := t.TempDir() // no policy.json → empty policy → no enrollment
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	if rc := runDossierVerify(nil, &out, &errb); rc != 0 {
		t.Errorf("verify with no floor + absent dir must be OK; got rc=%d stderr=%s", rc, errb.String())
	}
}

// TestDossierVerify_FloorEnrolledWithValidDossier_OK confirms enforcement is
// satisfied once a real, valid dossier is present.
func TestDossierVerify_FloorEnrolledWithValidDossier_OK(t *testing.T) {
	root := t.TempDir()
	writeDossierFloorPolicy(t, root)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	cyclesDir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(cyclesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d, err := dossier.Build(1, dossier.BuildOpts{WorkspacePath: "/w", Goal: "g"})
	if err != nil {
		t.Fatal(err)
	}
	if err := dossier.Write(d, cyclesDir, false); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := runDossierVerify(nil, &out, &errb); rc != 0 {
		t.Errorf("verify with a valid dossier must pass; got rc=%d stderr=%s", rc, errb.String())
	}
}
