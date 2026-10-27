package explanationdocs

import (
	"strings"
	"testing"
)

func TestFailurePrefix_EveryChangedAreaFailureStartsWithIt(t *testing.T) {
	t.Parallel()
	failures := changedAreaFailures("- `b.go`: why\n- `../x`: why\n", []string{"a.go"}, []string{"a.go"})

	if len(failures) < 2 {
		t.Fatalf("failures = %q; want the uncovered material path and the cited path", failures)
	}
	for _, f := range failures {
		if !strings.HasPrefix(f, FailurePrefix) {
			t.Errorf("failure %q does not start with %q, which the failure class parser counts", f, FailurePrefix)
		}
	}
}
