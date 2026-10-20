package inboxbatch

import (
	"encoding/json"
	"testing"
)

func TestItem_ReadsTheStaleDropStampsAndIgnoresAMalformedOne(t *testing.T) {
	t.Parallel()
	var it Item
	if err := json.Unmarshal([]byte(`{"id":"a","premise_verified_at":"2026-10-02T00:00:00Z","stale_drops":2,"last_stale_drop_at":"2026-10-01T00:00:00Z"}`), &it); err != nil {
		t.Fatal(err)
	}
	if it.PremiseVerified != "2026-10-02T00:00:00Z" || it.StaleDrops != 2 || it.LastStaleDropAt != "2026-10-01T00:00:00Z" {
		t.Fatalf("item = %+v, want the three stamps read", it)
	}
	var odd Item
	if err := json.Unmarshal([]byte(`{"id":"b","premise_verified_at":{"note":"x"},"stale_drops":"two"}`), &odd); err != nil || odd.PremiseVerified != "" || odd.StaleDrops != 0 {
		t.Fatalf("odd = %+v, %v; a malformed stamp reads as absent", odd, err)
	}
}

func TestVerifiedAfter(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		verified, last string
		want           bool
	}{
		{"2026-10-02T00:00:00Z", "2026-10-01T00:00:00Z", true},
		{"2026-10-01T00:00:00Z", "2026-10-02T00:00:00Z", false},
		{"2026-10-01T00:00:00Z", "", true},
		{"", "2026-10-01T00:00:00Z", false},
	} {
		if got := VerifiedAfter(tc.verified, tc.last); got != tc.want {
			t.Errorf("VerifiedAfter(%q, %q) = %v, want %v", tc.verified, tc.last, got, tc.want)
		}
	}
}
