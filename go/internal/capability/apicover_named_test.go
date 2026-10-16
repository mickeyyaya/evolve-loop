package capability

import "testing"

func TestInspection_MissingManifestShape(t *testing.T) {
	var insp Inspection
	insp, err := Inspect(t.TempDir(), "no-such-adapter")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !insp.Manifest.BudgetNative || !insp.Manifest.PermissionScoping {
		t.Errorf("absent manifest: both supports must default true, got %+v", insp.Manifest)
	}
	if len(insp.Warns) != 0 {
		t.Errorf("absent manifest must emit no warns, got %v", insp.Warns)
	}
}
