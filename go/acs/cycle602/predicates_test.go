//go:build acs

package cycle602

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC602_001_LaunchPopulatesBridgeResponseTokens(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestEngineLaunch_PopulatesBridgeResponseTokens")
	if !ok {
		t.Errorf("Engine.Launch does not populate core.BridgeResponse.Tokens from the injected TokenResolver:\n%s", out)
	}
}

func TestC602_002_LaunchAppendsOneRecordPerAttempt(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestEngineLaunch_AppendsLLMCallRecordPerFallbackAttempt")
	if !ok {
		t.Errorf("Engine.Launch does not append exactly one llm-calls.ndjson record per fallback attempt:\n%s", out)
	}
}

func TestC602_003_ResolverErrorNeverFailsLaunch(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestEngineLaunch_CollectorErrorNeverFailsLaunch")
	if !ok {
		t.Errorf("a TokenResolver error is not handled fail-open (must WARN, zero Tokens, never fail the Launch):\n%s", out)
	}
}
