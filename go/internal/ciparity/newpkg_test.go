package ciparity

// newpkg_test.go — RED contract for cycle-547's
// apicover-new-package-graduation-gate task.
//
// PROBLEM (scout Key Finding 3, 3rd confirmed recurrence of
// warnship_apicover_ci_gap): apicoverEnforceChangedDefault computes
// `touched := IntersectEnforced(changed, enforceBytes)` — a brand-new
// go/internal/<pkg> is in `changed` but, being new, cannot yet be listed in
// .apicover-enforce, so the intersection silently drops it and the
// unnamed-export gate never inspects it.
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS
// the RED evidence):
//
//	NewUngraduatedPackages(changed []string, enforceBytes []byte) []string
//	returns the changed package patterns that (a) live under go/internal/
//	(package go/cmd/... entrypoints are never apicover-scoped, so must NEVER
//	be flagged) and (b) are NOT present in the enforce list — i.e. exactly
//	the set IntersectEnforced silently drops. Normalizes the same way
//	IntersectEnforced does (strips a trailing "/..."), dedupes, and returns a
//	sorted slice (nil when nothing is ungraduated).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestNewUngraduatedPackages_FlagsChangedInternalPkgAbsentFromEnforceList
//   - Negative : TestNewUngraduatedPackages_AlreadyGraduatedPackagesNotFlagged
//     (the strongest anti-no-op: a naive "flag everything changed" impl fails
//     this — already-enforced packages must NOT reappear as "new")
//   - Edge     : TestNewUngraduatedPackages_CmdPackagesNeverFlagged (go/cmd/...
//     is out of apicover's scope entirely, per the AC)
//   - Edge     : TestNewUngraduatedPackages_DedupesAndSorts
import (
	"reflect"
	"testing"
)

func TestNewUngraduatedPackages_FlagsChangedInternalPkgAbsentFromEnforceList(t *testing.T) {
	enforce := []byte("./internal/config\n")
	changed := []string{"./internal/brandnew/..."}
	got := NewUngraduatedPackages(changed, enforce)
	want := []string{"./internal/brandnew"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewUngraduatedPackages(%v) = %v, want %v — a new internal package absent from .apicover-enforce must be flagged", changed, got, want)
	}
}

func TestNewUngraduatedPackages_AlreadyGraduatedPackagesNotFlagged(t *testing.T) {
	enforce := []byte("./internal/config\n./internal/router\n")
	changed := []string{"./internal/config/...", "./internal/router/..."}
	got := NewUngraduatedPackages(changed, enforce)
	if len(got) != 0 {
		t.Fatalf("NewUngraduatedPackages(%v) = %v, want empty — already-graduated packages must not be flagged as ungraduated", changed, got)
	}
}

func TestNewUngraduatedPackages_CmdPackagesNeverFlagged(t *testing.T) {
	enforce := []byte("")
	changed := []string{"./cmd/evolve/...", "./cmd/apicover/..."}
	got := NewUngraduatedPackages(changed, enforce)
	if len(got) != 0 {
		t.Fatalf("NewUngraduatedPackages(%v) = %v, want empty — go/cmd/... is out of apicover's enforcement scope and must never be flagged", changed, got)
	}
}

func TestNewUngraduatedPackages_DedupesAndSorts(t *testing.T) {
	enforce := []byte("")
	changed := []string{"./internal/zeta/...", "./internal/alpha/...", "./internal/zeta/..."}
	got := NewUngraduatedPackages(changed, enforce)
	want := []string{"./internal/alpha", "./internal/zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewUngraduatedPackages(%v) = %v, want %v (deduped, sorted)", changed, got, want)
	}
}
