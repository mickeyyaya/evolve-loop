package looppreflight

import (
	"path/filepath"
	"testing"
)

func TestSandboxCanarySetupFailureIsUnverified(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent", "project")
	if defaultSandboxCanary(root)() {
		t.Fatal("missing parent is a setup failure, not verified confinement")
	}
}
