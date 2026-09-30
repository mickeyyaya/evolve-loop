//go:build acs

// Package cycle1776 materializes the acceptance criteria for the single
// triage-committed task of this fleet lane (inbox-consume-records-resolution):
//
//   - `evolve inbox consume <item> [--resolution X] [--cycle N]` must write a
//     consumed{at, via, cycle, resolution} stamp onto the MOVED item JSON in
//     .evolve/inbox/consumed/, reusing the on-disk shape
//     continuation_release.go already reads (`{"consumed":{"cycle":...}}`).
//   - The two pre-existing zero-flag consume tests
//     (TestRunInbox_Consume_MovesItemAndAcksFingerprint,
//     TestRunInbox_Consume_ItemWithoutFingerprintStillMoves) must keep
//     passing unmodified — the stamp write is additive to the move-then-ack
//     invariant, not a replacement for it.
//
// Predicate strategy: cmd/evolve is package main, so these predicates drive
// the real behavioral Go tests authored alongside this contract
// (cmd/evolve/cmd_inbox_consume_test.go) through acsassert.GoTests — a
// narrowed `-run` subprocess (never a `/...` sweep; cmd/evolve is a known
// slow suite per go/acs/README.md's flaky-predicate-shape table) that
// verifies each named subtest actually reported PASS. This exercises the
// real CLI dispatch path (runInbox -> runInboxConsume), not a source grep.
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

// TestC1776_001_ConsumeWithFlagsStampsConsumedRecord is AC1: passing
// --resolution/--cycle stamps consumed{at,via,cycle,resolution} on the moved
// item, with via defaulting to "console-manual" and the passed values
// landing verbatim.
func TestC1776_001_ConsumeWithFlagsStampsConsumedRecord(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cmdEvolvePkg,
		Pattern: "^TestRunInbox_Consume_WithFlagsStampsConsumedRecord$",
		Names:   []string{"TestRunInbox_Consume_WithFlagsStampsConsumedRecord"},
	})
}

// TestC1776_002_ConsumeNoFlagsDefaultsConsumedStamp is the no-flag half of
// AC1: the stamp is written even when --resolution/--cycle are omitted,
// defaulting cycle to "console" and resolution to "".
func TestC1776_002_ConsumeNoFlagsDefaultsConsumedStamp(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cmdEvolvePkg,
		Pattern: "^TestRunInbox_Consume_NoFlagsDefaultsConsumedStamp$",
		Names:   []string{"TestRunInbox_Consume_NoFlagsDefaultsConsumedStamp"},
	})
}

// TestC1776_003_ConsumeMissingItemWithFlagsStillFails is the negative test
// (adversarial-testing SKILL §6): flags must not bypass the existing
// missing-item error path, and a failed consume must stamp nothing.
func TestC1776_003_ConsumeMissingItemWithFlagsStillFails(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cmdEvolvePkg,
		Pattern: "^TestRunInbox_Consume_MissingItemWithFlagsStillFailsAndStampsNothing$",
		Names:   []string{"TestRunInbox_Consume_MissingItemWithFlagsStillFailsAndStampsNothing"},
	})
}

// TestC1776_004_PreExistingConsumeRegressionTestsStillPass is AC2: the two
// tests predating this task must survive the stamp write unmodified — the
// move-then-ack ordering invariant (cmd_inbox_consume.go:82-86) must not
// regress when the stamp write is added between rename and ack.
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
