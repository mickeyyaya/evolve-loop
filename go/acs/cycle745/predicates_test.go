//go:build acs

package cycle745

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgePkg         = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	adaptersBridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	subagentPkg       = "github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

func TestC745_001_EngineWarnsOnNilTokenResolver(t *testing.T) {
	runGoTest(t, bridgePkg, "TestEngine_WarnsOnNilTokenResolver")
}

func TestC745_002_NoWarnWhenResolverWired(t *testing.T) {
	runGoTest(t, bridgePkg, "TestEngine_NoTokenResolverWarnWhenWired")
}

func TestC745_003_CompositionRootsWireResolver(t *testing.T) {
	runGoTest(t, adaptersBridgePkg, "TestProductionEngineDeps_WiresNonNilTokenResolver")
	runGoTest(t, subagentPkg, "TestExecAdapterDeps_WiresNonNilTokenResolver")
}
