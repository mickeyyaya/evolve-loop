package policy

import (
	"os"
	"path/filepath"
	"testing"
)

// regressiontia_stagefor_test.go names RegressionTIAStageFor — the root-based
// accessor the audit phase actually calls — and pins the direction of its
// degradation. regressiontia_test.go covers the Policy-value accessor; this
// covers the disk path, where the interesting failures live.

func writePolicyJSON(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestRegressionTIAStageFor_ReadsTheBlock proves the accessor is actually
// wired to the regression_tia block and not returning a constant.
func TestRegressionTIAStageFor_ReadsTheBlock(t *testing.T) {
	for _, stage := range []string{"shadow", "enforce", "off"} {
		root := writePolicyJSON(t, `{"regression_tia":{"stage":"`+stage+`"}}`)
		if got := RegressionTIAStageFor(root); got != stage {
			t.Errorf("RegressionTIAStageFor with stage %q = %q, want it honored", stage, got)
		}
	}
}

// TestRegressionTIAStageFor_DegradesToOff is the NEGATIVE axis: every way of
// failing to read a stage — absent block, absent file, malformed JSON, a typo —
// must resolve to the dormant default. A config problem may never ARM
// selection, because selection is the only thing here that can hide a
// regression class.
func TestRegressionTIAStageFor_DegradesToOff(t *testing.T) {
	cases := map[string]string{
		"absent block":    writePolicyJSON(t, `{}`),
		"malformed json":  writePolicyJSON(t, `{"regression_tia":`),
		"typo stage":      writePolicyJSON(t, `{"regression_tia":{"stage":"shadwo"}}`),
		"no policy file":  t.TempDir(),
		"empty root path": "",
	}
	for name, root := range cases {
		if got := RegressionTIAStageFor(root); got != "off" {
			t.Errorf("%s: RegressionTIAStageFor = %q, want \"off\"", name, got)
		}
	}
}
