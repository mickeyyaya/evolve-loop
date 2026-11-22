//go:build acs

package cycle2600

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func recordPRTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: pkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2600_001_AStalePullRefWritesNoRecordOfTheOldCommitList(t *testing.T) {
	recordPRTests(t, "./internal/integration", "TestRecordPRStalePullRefWritesNoRecordOfTheOldCommitList")
}

func TestC2600_002_APullRefThatCatchesUpInsideTheBoundRecordsTheFullCommitList(t *testing.T) {
	recordPRTests(t, "./internal/integration",
		"TestRecordPRStalePullRefThatCatchesUpInsideTheBoundRecordsTheFullCommitList",
		"TestRecordPRStalePullRefDoesNotRefuseAPullRefThatIsCurrent")
}

func TestC2600_003_APullRefThatNeverCatchesUpIsRefusedNamingBothSHAs(t *testing.T) {
	recordPRTests(t, "./internal/integration", "TestRecordPRStalePullRefThatNeverCatchesUpNamesBothSHAs")
}

func TestC2600_004_TheRecordPRVerbExitsRefusedForAStalePullRefAndRecordsACurrentOne(t *testing.T) {
	recordPRTests(t, "./cmd/evolve",
		"TestRecordPRStalePullRefExitsRefusedNamingBothSHAs",
		"TestRecordPRStalePullRefLeavesACurrentPullRefAlone")
}
