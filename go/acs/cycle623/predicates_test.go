//go:build acs

package cycle623

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	tokenusagePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	bridgePkg         = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	adaptersBridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	subagentPkg       = "github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC623_001_DefaultResolverWrapsTranscriptScanner(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestDefaultResolver_TranscriptFixture_ReturnsTranscriptSource")
	if !ok {
		t.Errorf("tokenusage.DefaultResolver missing or does not recover real transcript usage:\n%s", out)
	}
}

func TestC623_002_DefaultResolverFailsOpenOnEmptyRoot(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestDefaultResolver_EmptyConfigRoot_ReturnsSourceNoneNotError")
	if !ok {
		t.Errorf("tokenusage.DefaultResolver does not fail open on an empty configRoot:\n%s", out)
	}
}

func TestC623_003_EngineExposesTokenResolverPresence(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestHasTokenResolver_TrueWhenDepsFieldSet|TestHasTokenResolver_FalseWhenDepsFieldNil")
	if !ok {
		t.Errorf("bridge.Engine.HasTokenResolver missing or incorrect:\n%s", out)
	}
}

func TestC623_004_AdaptersBridgeCompositionRootWiresResolver(t *testing.T) {
	ok, out := runGoTest(t, adaptersBridgePkg,
		"TestProductionEngineDeps_WiresNonNilTokenResolver|TestEngineFactory_WiresTokenResolver")
	if !ok {
		t.Errorf("adapters/bridge.Adapter does not wire a non-nil TokenResolver into its production engineFactory:\n%s", out)
	}
}

func TestC623_005_AdaptersBridgeResolverIsGenuine(t *testing.T) {
	ok, out := runGoTest(t, adaptersBridgePkg, "TestProductionEngineDeps_ResolverAppliesRealFixture")
	if !ok {
		t.Errorf("adapters/bridge's wired TokenResolver does not genuinely scan HOME/.claude (looks disconnected from tokenusage.DefaultResolver):\n%s", out)
	}
}

func TestC623_006_SubagentCompositionRootWiresResolver(t *testing.T) {
	ok, out := runGoTest(t, subagentPkg,
		"TestExecAdapterDeps_WiresNonNilTokenResolver|TestExecAdapterDeps_MissingHome_StillReturnsNonNilResolver")
	if !ok {
		t.Errorf("subagent.execAdapterDeps does not wire a non-nil TokenResolver (including the missing-HOME edge case):\n%s", out)
	}
}

func TestC623_007_SubagentResolverIsGenuine(t *testing.T) {
	ok, out := runGoTest(t, subagentPkg, "TestExecAdapterDeps_ResolverAppliesRealFixture")
	if !ok {
		t.Errorf("subagent's wired TokenResolver does not genuinely scan HOME/.claude (looks disconnected from tokenusage.DefaultResolver):\n%s", out)
	}
}
