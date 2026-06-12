//go:build acs

// Package cycle308 contains the ACS predicates and bug reproduction verifiers
// for cycle 308. The three bugs targeted by this cycle are absence-of-API
// failures: the TDD test files (inboxmover_release_test.go,
// malformed_floors_test.go, versioninventory_test.go) fail to compile against
// the pre-fix source because the required functions did not exist at HEAD.
//
// This file verifies the post-fix API contract: if any of the required symbols
// are missing the predicate fails, which reproduces the original build failure
// class against the package-under-test.
package cycle308

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// TestCycle308_InboxReleaseAPIExists verifies that ReleaseCycleProcessing
// was added to inboxmover. Pre-fix: this function did not exist (compile error).
// Post-fix: the call compiles and behaves correctly.
func TestCycle308_InboxReleaseAPIExists(t *testing.T) {
	// If ReleaseCycleProcessing is missing the compiler rejects this file
	// before the test even runs — reproducing the original build failure.
	res, err := inboxmover.ReleaseCycleProcessing(inboxmover.Options{}, 0)
	if err != nil {
		t.Fatalf("ReleaseCycleProcessing: unexpected error on absent dir: %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("absent dir: Recovered = %d, want 0", res.Recovered)
	}
}

// TestCycle308_MalformedFloorWarningsExist verifies that
// MalformedCommittedFloorWarning and MalformedDeferredFloorWarning were added
// to triagecap. Pre-fix: absent → compile error. Post-fix: calls compile.
func TestCycle308_MalformedFloorWarningsExist(t *testing.T) {
	// Call with a nonexistent path → must return "" (absent is silent).
	if w := triagecap.MalformedCommittedFloorWarning("/no/such/file"); w != "" {
		t.Errorf("MalformedCommittedFloorWarning(/no/such): want \"\", got %q", w)
	}
	if w := triagecap.MalformedDeferredFloorWarning("/no/such/file"); w != "" {
		t.Errorf("MalformedDeferredFloorWarning(/no/such): want \"\", got %q", w)
	}
}

// TestCycle308_CLIVersionsFieldExists verifies that looppreflight.Result
// gained a CLIVersions map. Pre-fix: the field did not exist (compile error on
// r.CLIVersions in versioninventory_test.go). Post-fix: accessible here.
func TestCycle308_CLIVersionsFieldExists(t *testing.T) {
	// Directly reference the struct field — if absent this file fails to compile.
	var r looppreflight.Result
	if r.CLIVersions == nil {
		r.CLIVersions = map[string]string{"test": "1.0.0"}
	}
	if r.CLIVersions["test"] != "1.0.0" {
		t.Errorf("CLIVersions roundtrip: got %q, want 1.0.0", r.CLIVersions["test"])
	}
}
