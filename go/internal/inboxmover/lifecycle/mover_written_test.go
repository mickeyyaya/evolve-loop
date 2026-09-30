package lifecycle

import "testing"

func TestIsMoverWritten_CoversTheRouteAndEveryLifecycleField(t *testing.T) {
	for key, want := range map[string]bool{
		RouteField: true, "routed_reason": true, "routed_cycle": true, "failure_count": true, "last_failure_reason": true,
		"consumed": true, "continuation": true, "retired_at": true,
		"weight": false, "summary": false, "acceptance": false, "id": false,
	} {
		if got := IsMoverWritten(key); got != want {
			t.Errorf("IsMoverWritten(%q) = %v, want %v", key, got, want)
		}
	}
}
