package capability

import "testing"

// TestInspection_MissingManifestShape names the capability.Inspection type
// (Inspect returns it but the bare type is never named in a test) and pins the
// absent-manifest contract the subagent validators rely on: both supports
// default true and the warn list is empty. Inspection holds a []string, so it is
// asserted field-by-field.
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
