//go:build acs

package cycle1776

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func moduleRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func TestC1776_001_ConsumeWithFlagsStampsConsumedRecord(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cmdEvolvePkg,
		Pattern: "^TestRunInbox_Consume_WithFlagsStampsConsumedRecord$",
		Names:   []string{"TestRunInbox_Consume_WithFlagsStampsConsumedRecord"},
	})
}

func TestC1776_002_ConsumeNoFlagsDefaultsConsumedStamp(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cmdEvolvePkg,
		Pattern: "^TestRunInbox_Consume_NoFlagsDefaultsConsumedStamp$",
		Names:   []string{"TestRunInbox_Consume_NoFlagsDefaultsConsumedStamp"},
	})
}

func TestC1776_003_ConsumeMissingItemWithFlagsStillFails(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cmdEvolvePkg,
		Pattern: "^TestRunInbox_Consume_MissingItemWithFlagsStillFailsAndStampsNothing$",
		Names:   []string{"TestRunInbox_Consume_MissingItemWithFlagsStillFailsAndStampsNothing"},
	})
}

func TestC1776_004_PreExistingConsumeRegressionTestsStillPass(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cmdEvolvePkg,
		Pattern: "^(TestRunInbox_Consume_MovesItemAndAcksFingerprint|TestRunInbox_Consume_ItemWithoutFingerprintStillMoves)$",
		Names: []string{
			"TestRunInbox_Consume_MovesItemAndAcksFingerprint",
			"TestRunInbox_Consume_ItemWithoutFingerprintStillMoves",
		},
	})
}
