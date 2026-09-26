package looppreflight

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// checkSandboxNestedFallback samples one out-of-allowlist write in a nested session: off skips,
// shadow warns, enforce halts. A denied write never waives the launch-time gate.
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

// defaultSandboxCanary writes a sentinel in the project's parent, outside the inner sandbox's
// allow-list. Only a permission error returns true; any other setup failure stays unverified.
func defaultSandboxCanary(projectRoot string) func() bool {
	return func() bool {
		f, err := os.CreateTemp(filepath.Dir(projectRoot), ".evolve-sandbox-canary-*")
		if err != nil {
			return os.IsPermission(err)
		}
		// Best-effort: POSIX unlinks the sentinel whatever Close returns.
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
		return false
	}
}
