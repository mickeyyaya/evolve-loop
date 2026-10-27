package continuation

import (
	"strings"
	"testing"
)

func TestRequireOperatorAuthority(t *testing.T) {
	t.Run("no flag refuses and names the flag, whatever the environment says", func(t *testing.T) {
		t.Setenv("EVOLVE_OPERATOR_CONFIRM", "1")
		err := RequireOperatorAuthority(false)
		if err == nil || !strings.Contains(err.Error(), "-operator") {
			t.Fatalf("RequireOperatorAuthority(false) = %v, want a refusal that names -operator", err)
		}
	})

	t.Run("the flag authorizes", func(t *testing.T) {
		if err := RequireOperatorAuthority(true); err != nil {
			t.Errorf("RequireOperatorAuthority(true) refused an explicitly authorized caller: %v", err)
		}
	})
}
