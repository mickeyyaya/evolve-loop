//go:build acs

package cycle2679

import (
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func requireBridgeTestsPass(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		stdout, stderr, code, err := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go", "test", "-count=1", "-v", "-run", "^"+name+"$", "./internal/bridge")
		passed := regexp.MustCompile(`--- PASS: ` + name + ` `).MatchString(stdout)
		if err != nil || code != 0 || !passed {
			t.Errorf("%s: exit %d err %v pass-line %v\n%s%s", name, code, err, passed, acsassert.OutputTail(stdout, 30), acsassert.OutputTail(stderr, 10))
		}
	}
}

func TestC2679_001_DriverTestPinsTheMessageTextAndTheHeadlessArgv(t *testing.T) {
	requireBridgeTestsPass(t,
		"TestAgyDriver_MessageNamesTheRoutedFlags",
		"TestAgyDriver_MessageWithoutRouteDataNamesTheAgyDefault",
		"TestAgyDriver_HeadlessArgvCarriesTheRoutedModelAndEffort",
		"TestAgyDriver_HeadlessArgvWithoutRouteDataHasNoModelOrEffortFlag",
	)
}

func TestC2679_002_NoMessageSaysAgyHasNoModelFlag(t *testing.T) {
	requireBridgeTestsPass(t, "TestAgyDriver_NoMessageSaysAgyHasNoModelFlag")
}

func TestC2679_003_ModelAndEffortComeFromTheRouteAndARouteChangeChangesTheArgv(t *testing.T) {
	requireBridgeTestsPass(t,
		"TestAgyDriver_RouteChangeChangesTheArgv",
		"TestAgyDriver_RouteIntentFromConfigReachesTheArgv",
	)
}
