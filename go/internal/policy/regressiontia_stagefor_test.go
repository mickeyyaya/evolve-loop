package policy

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestRegressionTIAStageFor_ReadsTheBlock(t *testing.T) {
	for _, stage := range []string{"shadow", "enforce", "off"} {
		root := writePolicyJSON(t, `{"regression_tia":{"stage":"`+stage+`"}}`)
		if got := RegressionTIAStageFor(root); got != stage {
			t.Errorf("RegressionTIAStageFor with stage %q = %q, want it honored", stage, got)
		}
	}
}

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
