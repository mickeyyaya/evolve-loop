//go:build acs

package cycle1013

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const tokensPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runTokensTest(t *testing.T, pattern string, wantPass ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{Package: tokensPkg, Pattern: pattern, Names: wantPass})
}

func TestC1013_001_tripwire_surfaced_in_text_and_json(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SurfacesTripwireInTextAndJSON$",
		"TestRunTokensReport_SurfacesTripwireInTextAndJSON")
}

func TestC1013_002_tripwire_survives_empty_phases(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty$",
		"TestRunTokensReport_SurfacesTripwireEvenWhenPhasesEmpty")
}

func TestC1013_003_zero_tripwire_stays_quiet(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_ZeroTripwireStaysQuiet$",
		"TestRunTokensReport_ZeroTripwireStaysQuiet")
}

func TestC1013_004_tripwire_control_bytes_sanitized(t *testing.T) {
	runTokensTest(t,
		"^TestRunTokensReport_SanitizesTripwireControlBytes$",
		"TestRunTokensReport_SanitizesTripwireControlBytes")
}
