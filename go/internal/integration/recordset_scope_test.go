package integration

import "testing"

func TestRecordFailure_ConcernsTheListingAndItsOwnPoolOnly(t *testing.T) {
	t.Parallel()
	for name, row := range map[string]struct {
		scope FailureScope
		pool  int
		want  bool
	}{
		"a listing failure concerns each pool": {ScopeListing, 9, true},
		"a file of this pool":                  {ScopePool, 5, true},
		"a file of an older pool":              {ScopePool, 4, false},
		"a file of a named directory":          {ScopeNamed, 5, false},
	} {
		failure := RecordFailure{Pool: row.pool, Scope: row.scope}
		if got := failure.Concerns(5); got != row.want {
			t.Errorf("%s: got %v, want %v", name, got, row.want)
		}
	}
}
