package ciparity

import (
	"reflect"
	"testing"
)

func TestNewUngraduatedPackages_FlagsChangedInternalPkgAbsentFromEnforceList(t *testing.T) {
	t.Parallel()
	enforce := []byte("./internal/config\n")
	changed := []string{"./internal/brandnew/..."}
	got := NewUngraduatedPackages(changed, enforce)
	want := []string{"./internal/brandnew"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewUngraduatedPackages(%v) = %v, want %v — a new internal package absent from .apicover-enforce must be flagged", changed, got, want)
	}
}

func TestNewUngraduatedPackages_AlreadyGraduatedPackagesNotFlagged(t *testing.T) {
	t.Parallel()
	enforce := []byte("./internal/config\n./internal/router\n")
	changed := []string{"./internal/config/...", "./internal/router/..."}
	got := NewUngraduatedPackages(changed, enforce)
	if len(got) != 0 {
		t.Fatalf("NewUngraduatedPackages(%v) = %v, want empty — already-graduated packages must not be flagged as ungraduated", changed, got)
	}
}

func TestNewUngraduatedPackages_CmdPackagesNeverFlagged(t *testing.T) {
	t.Parallel()
	enforce := []byte("")
	changed := []string{"./cmd/evolve/...", "./cmd/apicover/..."}
	got := NewUngraduatedPackages(changed, enforce)
	if len(got) != 0 {
		t.Fatalf("NewUngraduatedPackages(%v) = %v, want empty — go/cmd/... is out of apicover's enforcement scope and must never be flagged", changed, got)
	}
}

func TestNewUngraduatedPackages_DedupesAndSorts(t *testing.T) {
	t.Parallel()
	enforce := []byte("")
	changed := []string{"./internal/zeta/...", "./internal/alpha/...", "./internal/zeta/..."}
	got := NewUngraduatedPackages(changed, enforce)
	want := []string{"./internal/alpha", "./internal/zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewUngraduatedPackages(%v) = %v, want %v (deduped, sorted)", changed, got, want)
	}
}
