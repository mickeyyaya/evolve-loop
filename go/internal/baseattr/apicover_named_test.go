package baseattr

import "testing"

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	t.Parallel()
	var (
		_ Key
		_ Reds
		_ Verdict
		_ Runner
		_ Attributor
	)
	if KeysOf(nil).Attributable() || ParseGoTest("").Attributable() || GoTestRunner(nil) == nil {
		t.Error("an empty input is attributable, or the runner is nil")
	}
}
