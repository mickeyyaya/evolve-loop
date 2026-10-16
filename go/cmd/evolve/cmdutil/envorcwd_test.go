package cmdutil

import (
	"path/filepath"
	"testing"
)

func TestEnvOrCwd(t *testing.T) {
	const key = "EVOLVE_CMDUTIL_TEST_ROOT"

	t.Setenv(key, "/tmp/some/root")
	if got := EnvOrCwd(key); !filepath.IsAbs(got) {
		t.Errorf("EnvOrCwd(set) = %q, want an absolute path", got)
	}

	got := EnvOrCwd("EVOLVE_CMDUTIL_DEFINITELY_UNSET_ABCXYZ")
	if !filepath.IsAbs(got) {
		t.Errorf("EnvOrCwd(unset) = %q, want an absolute cwd fallback", got)
	}
}
