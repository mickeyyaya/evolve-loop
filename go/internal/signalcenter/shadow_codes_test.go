package signalcenter

import "testing"

func TestShadowCodes_AreRegisteredUnderTheModuleThatEmitsThem(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		code     Code
		spelling string
		module   Module
	}{
		{CodeAuditCascadeShadow, "AUDIT_CASCADE_SHADOW", ModuleAudit},
		{CodeParallelEvaluateShadow, "PARALLEL_EVALUATE_SHADOW", ModuleParallelEval},
		{CodeLogAccessShadow, "LOG_ACCESS_SHADOW", ModuleLog},
	} {
		if string(tc.code) != tc.spelling {
			t.Errorf("code %q drifted from its spelling %q", tc.code, tc.spelling)
		}
		m, ok := IsRegistered(tc.code)
		if !ok || m != tc.module {
			t.Errorf("%s: registered=%v under %q, want %q", tc.code, ok, m, tc.module)
		}
	}
}
