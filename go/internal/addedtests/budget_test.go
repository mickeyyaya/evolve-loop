package addedtests

import (
	"testing"
	"time"
)

func TestPackageTimeout_IsAGoTestDurationWithHeadroomOverGosDefault(t *testing.T) {
	d, err := time.ParseDuration(PackageTimeout)
	if err != nil {
		t.Fatalf("PackageTimeout %q is not a duration go test accepts: %v", PackageTimeout, err)
	}
	if d <= 10*time.Minute {
		t.Errorf("PackageTimeout %v gives no headroom over go test's 10m default, the deadline that red-lined cycle 1679's ship", d)
	}
}

func TestPackageWallBudget_IsValidDurationWellBelowPackageTimeout(t *testing.T) {
	budget, err := time.ParseDuration(PackageWallBudget)
	if err != nil {
		t.Fatalf("PackageWallBudget %q is not a duration go test accepts: %v", PackageWallBudget, err)
	}
	timeout, err := time.ParseDuration(PackageTimeout)
	if err != nil {
		t.Fatalf("PackageTimeout %q is not a duration go test accepts: %v", PackageTimeout, err)
	}
	if budget <= 0 || budget > timeout/4 {
		t.Errorf("PackageWallBudget %v must be > 0 and <= PackageTimeout/4 (%v)", budget, timeout/4)
	}
}
