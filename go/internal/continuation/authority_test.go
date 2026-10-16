package continuation

import (
	"strings"
	"testing"
)

func TestOperatorConfirmEnv(t *testing.T) {
	if OperatorConfirmEnv != "EVOLVE_OPERATOR_CONFIRM" {
		t.Errorf("OperatorConfirmEnv = %q, want EVOLVE_OPERATOR_CONFIRM", OperatorConfirmEnv)
	}
}

func TestRequireOperatorAuthority(t *testing.T) {
	t.Run("no flag and no env refuses, naming both authority paths", func(t *testing.T) {
		t.Setenv(OperatorConfirmEnv, "")
		err := RequireOperatorAuthority(false)
		if err == nil {
			t.Fatal("RequireOperatorAuthority(false) with no authority returned nil — an unauthorized caller would drop a binding")
		}
		for _, want := range []string{"-operator", OperatorConfirmEnv} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal names no %s — the error IS the operator's guidance: %v", want, err)
			}
		}
	})

	t.Run("the flag authorizes", func(t *testing.T) {
		t.Setenv(OperatorConfirmEnv, "")
		if err := RequireOperatorAuthority(true); err != nil {
			t.Errorf("RequireOperatorAuthority(true) refused an explicitly authorized caller: %v", err)
		}
	})

	t.Run("an affirmative env value authorizes", func(t *testing.T) {
		t.Setenv(OperatorConfirmEnv, "1")
		if err := RequireOperatorAuthority(false); err != nil {
			t.Errorf("RequireOperatorAuthority(false) refused with %s=1: %v", OperatorConfirmEnv, err)
		}
	})

	t.Run("a set-but-negative env value is not authority", func(t *testing.T) {
		for _, raw := range []string{"0", "false", "no", "off", "maybe"} {
			t.Setenv(OperatorConfirmEnv, raw)
			if err := RequireOperatorAuthority(false); err == nil {
				t.Errorf("RequireOperatorAuthority(false) accepted %s=%q as consent", OperatorConfirmEnv, raw)
			}
		}
	})
}
