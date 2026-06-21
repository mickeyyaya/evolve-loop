package cmdutil

import (
	"path/filepath"
	"testing"
)

// TestEnvOrCwd covers both branches: env-set returns the (absolutized) value,
// env-unset falls back to an absolute cwd.
func TestEnvOrCwd(t *testing.T) {
	const key = "EVOLVE_CMDUTIL_TEST_ROOT"

	t.Setenv(key, "/tmp/some/root")
	if got := EnvOrCwd(key); !filepath.IsAbs(got) {
		t.Errorf("EnvOrCwd(set) = %q, want an absolute path", got)
	}

	// A name that is not set → fall back to the (absolute) working directory.
	got := EnvOrCwd("EVOLVE_CMDUTIL_DEFINITELY_UNSET_ABCXYZ")
	if !filepath.IsAbs(got) {
		t.Errorf("EnvOrCwd(unset) = %q, want an absolute cwd fallback", got)
	}
}
