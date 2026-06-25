package looppreflight

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// checkSandboxNestedFallback is the verified-fallback gate (P2). Under a nested
// session the inner OS sandbox is skipped, so the loop relies on the OUTER
// environment to confine source-writing phases. Rather than ASSUME that, this
// check runs a write-canary — gated by the sandbox.nested_fallback dial — to
// VERIFY it: if a write outside the inner sandbox's allow-list succeeds, the
// outer session does not compensate.
//
//	off (default) ⇒ canary disabled (no behavior change)
//	shadow        ⇒ WARN when unverified
//	enforce       ⇒ HALT when unverified
//
// Only engaged when sandboxing is wanted AND the session is nested — the exact
// scenario the dial governs.
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
		return CheckResult{Name: name, Level: LevelPass, Message: "verified: outer environment blocked an out-of-allowlist write"}
	}
	detail := "a write OUTSIDE the inner sandbox's allow-list succeeded — the outer Claude Code session does not confine source-writing phases at the OS layer; set sandbox.nested_fallback=off to silence, or run under a genuinely-confined outer session"
	if o.nestedFallbackStage == config.StageEnforce {
		return CheckResult{Name: name, Level: LevelHalt, Message: "nested fallback UNVERIFIED (enforce)", Detail: detail}
	}
	return CheckResult{Name: name, Level: LevelWarn, Message: "nested fallback UNVERIFIED (shadow)", Detail: detail}
}

// defaultSandboxCanary returns the production canary: it attempts a write
// OUTSIDE the inner sandbox's write allow-list (a sentinel in the project's
// PARENT directory — the inner sandbox makes the repo read-only and confines
// writes to the worktree/workspace/tmp) and reports whether the OUTER
// environment blocked it. blocked=true ⇒ outer confinement verified; false ⇒
// the write succeeded, so the outer environment does not confine.
//
// A setup error (couldn't even attempt the write) is treated as blocked — a
// write we could not perform is not evidence of an unconfined environment, and
// the conservative reading avoids a spurious HALT.
func defaultSandboxCanary(projectRoot string) func() bool {
	return func() bool {
		f, err := os.CreateTemp(filepath.Dir(projectRoot), ".evolve-sandbox-canary-*")
		if err != nil {
			return true
		}
		// Best-effort cleanup: the sentinel is unlinked regardless of the Close
		// outcome on POSIX, so both errors are intentionally discarded.
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
		return false
	}
}
