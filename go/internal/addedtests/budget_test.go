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
