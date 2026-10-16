//go:build acs

package cycle1910

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runPublisherTests(t *testing.T, pattern string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-race",
		"-run", pattern, "./internal/events/publisher/")
	if err != nil || code != 0 {
		t.Errorf("publisher tests %q failed (exit=%d, err=%v)\n%s\n%s", pattern, code, err, out, errOut)
	}
}

func TestC1910_001_ABlockedPutReturnsAtClose(t *testing.T) {
	runPublisherTests(t, "^TestPublisher_ABlockedPutReturnsAtClose$")
}

func TestC1910_002_EveryBlockedPutReturnsAtClose(t *testing.T) {
	runPublisherTests(t, "^TestPublisher_EveryBlockedPutReturnsAtClose$")
}

func TestC1910_003_APutAfterCloseIsRefusedWithoutWaiting(t *testing.T) {
	runPublisherTests(t, "^TestPublisher_APutAfterCloseIsRefusedWithoutWaiting$")
}
