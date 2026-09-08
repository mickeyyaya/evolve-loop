package looppreflight

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// checkSandboxNestedFallback samples an out-of-allowlist write under a nested
// session. Off leaves the diagnostic disabled, shadow warns, and enforce halts
// on an unsuccessful probe. Even a denied write does not attest the full
// profile's read/write restrictions and never waives the launch-time gate.
func checkSandboxNestedFallback(o resolved) CheckResult {
	const name = "sandbox-nested-fallback"
	if o.nestedFallbackStage == config.StageOff {
		return CheckResult{Name: name, Level: LevelPass, Message: "canary disabled (sandbox.nested_fallback=off)"}
	}
	if !sandboxWanted(o.profileLister, o.profileGetter) {
		return CheckResult{Name: name, Level: LevelPass, Message: "no profile requests sandboxing — nested fallback not engaged"}
	}
	if host := o.hostProbe(); !host.ClaudeCode.Nested {
		return CheckResult{Name: name, Level: LevelPass, Message: "standalone session — nested fallback not engaged"}
	}
	if o.sandboxCanaryProbe() {
		return CheckResult{Name: name, Level: LevelPass, Message: "sampled write denied; profile-specific read/write confinement remains UNVERIFIED"}
	}
	detail := "the probe did not establish an out-of-allowlist write denial (write succeeded or probe setup failed); no profile-specific read or write restriction is verified"
	if o.nestedFallbackStage == config.StageEnforce {
		return CheckResult{Name: name, Level: LevelHalt, Message: "nested fallback UNVERIFIED (enforce)", Detail: detail}
	}
	return CheckResult{Name: name, Level: LevelWarn, Message: "nested fallback UNVERIFIED (shadow)", Detail: detail}
}

// defaultSandboxCanary returns the production canary: it attempts a write
// OUTSIDE the inner sandbox's write allow-list (a sentinel in the project's
// PARENT directory — the inner sandbox makes the repo read-only and confines
// writes to the worktree/workspace/tmp) and reports whether the OUTER
// environment denied this single write. False includes both a successful
// write and an inconclusive setup failure.
//
// Only a permission-denied result counts as a sampled write denial. Missing
// parents, descriptor exhaustion, and other setup failures remain unverified.
// Even a permission denial can be ordinary DAC, not proof of the full policy.
func defaultSandboxCanary(projectRoot string) func() bool {
	return func() bool {
		f, err := os.CreateTemp(filepath.Dir(projectRoot), ".evolve-sandbox-canary-*")
		if err != nil {
			return os.IsPermission(err)
		}
		// Best-effort cleanup: the sentinel is unlinked regardless of the Close
		// outcome on POSIX, so both errors are intentionally discarded.
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
		return false
	}
}
