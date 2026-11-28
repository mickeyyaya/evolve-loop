//go:build acs

package cycle2684

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runAcsassertTests(t *testing.T, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./pkg/acsassert",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2684_001_TheSubprocessCapPlusTheLargestLaneSumFitsTheCheckedInGoTimeout(t *testing.T) {
	runAcsassertTests(t,
		"TestSubprocessCapFitsGoTimeout",
		"TestSubprocessCapFitsGoTimeout_RejectsATightPolicy",
	)
}

func TestC2684_002_TheBoundMessageNamesTheCallerDeadlineOnlyWhenItIsTheEarlierBound(t *testing.T) {
	runAcsassertTests(t, "TestBoundLineNamesCallerDeadline")
}
