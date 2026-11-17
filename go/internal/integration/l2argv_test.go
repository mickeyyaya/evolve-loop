package integration

import (
	"slices"
	"testing"
)

func TestPackageTestArgv_IsAGoTestOfTheOnePackage(t *testing.T) {
	t.Parallel()
	argv := PackageTestArgv("./internal/beta/...")
	if !slices.Equal([]string{argv[0], argv[1]}, []string{"go", "test"}) || argv[len(argv)-1] != "./internal/beta/..." || !slices.Contains(argv, "-count=1") {
		t.Errorf("PackageTestArgv = %v, want go test -count=1 with the package last", argv)
	}
}
