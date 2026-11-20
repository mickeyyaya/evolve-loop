package gc

import (
	"errors"
	"slices"
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

func TestSalvageBundleVerified_OriginMainBranches(t *testing.T) {
	cases := []struct {
		name         string
		originErr    error
		revListErr   error
		aheadOut     string
		wantCreate   bool
		wantNegRange bool
	}{
		{"ahead of origin/main uses thin range", nil, nil, "1\n", true, true},
		{"in origin/main skips bundle", nil, nil, "0\n", false, false},
		{"without origin/main writes full bundle", errors.New("exit 1"), nil, "", true, false},
		{"rev-list error falls back to full bundle", nil, errors.New("exit 1"), "", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var createArgs []string
			git := func(_ string, args ...string) (string, error) {
				switch args[0] {
				case "rev-parse":
					if tc.originErr != nil {
						return "", tc.originErr
					}
					return salvageShaF1 + "\n", nil
				case "rev-list":
					if tc.revListErr != nil {
						return "", tc.revListErr
					}
					return tc.aheadOut, nil
				case "bundle":
					switch args[1] {
					case "create":
						createArgs = append([]string(nil), args...)
					case "list-heads":
						return salvageShaF1 + " refs/heads/b\n", nil
					}
				}
				return "", nil
			}
			if err := bundleVerified(git, t.TempDir(), "x.bundle", "refs/heads/b", salvageShaF1); err != nil {
				t.Fatalf("bundleVerified: %v", err)
			}
			if tc.wantCreate && len(createArgs) == 0 {
				t.Fatal("bundle create did not run")
			}
			if !tc.wantCreate && len(createArgs) != 0 {
				t.Fatalf("bundle create ran unexpectedly: %v", createArgs)
			}
			hasNeg := slices.Contains(createArgs, "^"+salvageOriginMainRef)
			if hasNeg != tc.wantNegRange {
				t.Errorf("create args %v negative origin/main = %v, want %v", createArgs, hasNeg, tc.wantNegRange)
			}
		})
	}
}
