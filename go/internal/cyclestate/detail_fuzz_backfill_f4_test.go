package cyclestate

import (
	"strings"
	"testing"
)

func FuzzWithoutDetail(f *testing.F) {
	for _, seed := range []string{"plain", "a [[detail: x ]] b", "a [[detail: open", " [[detail:  [[detail: nested ]] ]]", "", " ]]"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, reason string) {
		got := WithoutDetail(reason)
		if strings.Contains(got, detailOpen) {
			t.Fatalf("WithoutDetail(%q) = %q still holds a detail opener", reason, got)
		}
		if again := WithoutDetail(got); again != got {
			t.Fatalf("WithoutDetail is not idempotent: %q -> %q", got, again)
		}
		if len(got) > len(reason) {
			t.Fatalf("stripping never grows the reason: %q -> %q", reason, got)
		}
	})
}
