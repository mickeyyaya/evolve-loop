// Package version exposes the build-stamped binary identity.
//
// Phase 1 task #5 ships only a placeholder; task #6 lands the
// ldflag + runtime/debug.BuildInfo implementation under TDD.
package version

// Get returns a human-readable build identity. Replaced under TDD in task #6.
func Get() string {
	return "evolve dev (unscaffolded)"
}
