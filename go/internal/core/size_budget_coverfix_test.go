package core

import "testing"

func TestScaledCorrectionLimit_NeverShrinksTheLadderBelowBase(t *testing.T) {
	t.Parallel()
	if got := scaledCorrectionLimit(4, 0.5); got != 4 {
		t.Fatalf("scaledCorrectionLimit(4, 0.5) = %d, want the base 4", got)
	}
}
