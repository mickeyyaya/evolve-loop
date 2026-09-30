//go:build acs

package cycle1015

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const tokensPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runTokensTest(t *testing.T, pattern string, wantPass ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{Package: tokensPkg, Pattern: pattern, Names: wantPass})
}

func TestC1015_001_report_surfaces_tripwire_naming_cli_agent_cycle(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SurfacesTripwireInTextAndJSON$",
		"TestRunTokensReport_SurfacesTripwireInTextAndJSON")
}

func TestC1015_002_report_surfaces_tripwire_even_when_phases_empty(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty$",
		"TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty")
}

func TestC1015_003_report_sanitizes_tripwire_control_bytes(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SanitizesTripwireControlBytes$",
		"TestRunTokensReport_SanitizesTripwireControlBytes")
}

func TestC1015_004_report_stays_quiet_when_zero_tripwires(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_ZeroTripwireStaysQuiet$",
		"TestRunTokensReport_ZeroTripwireStaysQuiet")
}
