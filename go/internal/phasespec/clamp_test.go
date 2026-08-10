package phasespec

// clamp_test.go — RED contract for the load-time registrar-equivalent clamp
// (inbox loadtime-userspec-registrar-clamp 0.93; ADR-0073 security review,
// Finding 1 downstream trace). DiscoverUserSpecsFromRoots loads any on-disk
// .evolve/phases/*/phase.json into the catalog with NONE of the registrar's
// normalization: a smuggled/residue/typo spec claiming writes_source:true
// became a schedulable phase with worktree-write ELIGIBILITY
// (core.worktreePhase reads spec.WritesSource straight from the catalog) and
// no sandbox anywhere — specrunner has no sandbox plumbing; the registrar
// mint path enforces the invariant by CONSTRUCTION (it persists a
// sandbox-enabled profile). The discovery clamp enforces it by VERIFICATION:
// writes_source survives only when the spec's dispatch profile — the SAME
// on-disk profile the runner resolves at dispatch (TrimPrefix(AgentName,
// "evolve-")) — exists with sandbox enabled. On-disk reads are correct here
// (dispatch-parity, not a repo-contract scan — ADR-0084 I1 does not apply).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClampDiscoveredSpecs_StripsUnsandboxedWritesSource(t *testing.T) {
	t.Parallel()
	specs := []PhaseSpec{
		{Name: "smuggled-writer", Optional: true, WritesSource: true},
		{Name: "honest-check", Optional: true},
	}
	clamped, warns := ClampDiscoveredSpecs(specs, func(string) bool { return false })
	if clamped[0].WritesSource {
		t.Fatal("writes_source survived with NO sandboxed dispatch profile — the ADR-0073 worktree-write hole")
	}
	if clamped[1].WritesSource {
		t.Error("non-writer gained writes_source")
	}
	joined := strings.Join(warns, "\n")
	if !strings.Contains(joined, "smuggled-writer") || !strings.Contains(joined, "writes_source") {
		t.Errorf("the strip must warn loudly naming the phase: %v", warns)
	}
}

func TestClampDiscoveredSpecs_KeepsVerifiedWriterAndForcesOptional(t *testing.T) {
	t.Parallel()
	specs := []PhaseSpec{
		{Name: "minted-writer", Agent: "evolve-minted-writer", Optional: false, WritesSource: true},
	}
	clamped, warns := ClampDiscoveredSpecs(specs, func(profile string) bool {
		return profile == "minted-writer" // the registrar-persisted, sandbox-enabled profile
	})
	if !clamped[0].WritesSource {
		t.Fatal("a writer with a verified sandboxed profile must keep writes_source")
	}
	if !clamped[0].Optional {
		t.Fatal("Optional must be FORCED true (registrar parity) — a user phase can never satisfy the spine floor")
	}
	if !strings.Contains(strings.Join(warns, "\n"), "minted-writer") {
		t.Errorf("the Optional force must warn (the spec author claimed mandatory): %v", warns)
	}
}

func TestSandboxedProfilePredicate_ReadsDispatchProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sandboxed", `{"name":"sandboxed","sandbox":{"enabled":true}}`)
	write("unsandboxed", `{"name":"unsandboxed"}`)
	write("disabled", `{"name":"disabled","sandbox":{"enabled":false}}`)
	// Registrar mirror: a writer needs to write — read-only sandboxes grant
	// eligibility without capability and must not verify.
	write("readonly", `{"name":"readonly","sandbox":{"enabled":true,"read_only_repo":true}}`)

	pred := SandboxedProfilePredicate(dir)
	if !pred("sandboxed") {
		t.Error("sandbox-enabled profile not recognized")
	}
	for _, name := range []string{"unsandboxed", "disabled", "absent", "readonly"} {
		if pred(name) {
			t.Errorf("profile %q must NOT verify as sandboxed-writable", name)
		}
	}
}

func TestClampDiscoveredSpecs_NilPredicateFailsClosed(t *testing.T) {
	t.Parallel()
	clamped, _ := ClampDiscoveredSpecs([]PhaseSpec{{Name: "w", Optional: true, WritesSource: true}}, nil)
	if clamped[0].WritesSource {
		t.Fatal("nil predicate must fail closed — writes_source stripped")
	}
}
