package looppreflight

import (
	"strconv"
	"strings"
	"testing"
)

func TestBootRCName_DefaultBranch_IncludesExitCode(t *testing.T) {
	unknownCodes := []struct {
		rc   int
		name string
	}{
		{1, "generic-error"},
		{42, "arbitrary"},
		{99, "high-unrecognized"},
		{128, "signal-base"},
	}
	for _, tc := range unknownCodes {
		t.Run(tc.name, func(t *testing.T) {
			got := bootRCName(tc.rc)
			if !strings.Contains(got, strconv.Itoa(tc.rc)) {
				t.Errorf("bootRCName(%d) = %q: numeric exit code absent from diagnostic string; operators cannot identify the actual failure mode without it", tc.rc, got)
			}
		})
	}
}
