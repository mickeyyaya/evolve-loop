package phasecontract

import "testing"

func TestScoutContract_NeverRequiresItsOwnChallengeToken(t *testing.T) {
	t.Parallel()
	c, ok := For("scout")
	if !ok {
		t.Fatal("scout has no built-in contract")
	}
	if c.RequireChallengeToken {
		t.Fatal("scout mints the challenge token, so requiring its echo is circular")
	}
}
