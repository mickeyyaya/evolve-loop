package gc

import (
	"errors"
	"strings"
	"testing"
)

func TestSalvageBundleVerified_EachStepFailureNamesItsCause(t *testing.T) {
	boom := errors.New("git exploded")
	cases := []struct {
		name  string
		fail  string
		heads string
		want  string
	}{
		{"create fails", "create", "", "git exploded"},
		{"verify fails", "verify", "", "git exploded"},
		{"list-heads fails", "list-heads", "", "git exploded"},
		{"bundle lists another tip", "", "deadbeef refs/heads/b\n", "does not hold refs/heads/b@" + salvageShaF1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			git := func(_ string, args ...string) (string, error) {
				if args[1] == tc.fail {
					return "", boom
				}
				return tc.heads, nil
			}
			err := bundleVerified(git, t.TempDir(), "x.bundle", "refs/heads/b", salvageShaF1)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestSalvageRef_ADetachedTreeBundlesItsHead(t *testing.T) {
	if got := salvageRef(""); got != "HEAD" {
		t.Errorf("salvageRef(\"\") = %q, want HEAD", got)
	}
	if got := salvageRef("b"); got != "refs/heads/b" {
		t.Errorf("salvageRef(b) = %q, want refs/heads/b", got)
	}
}
