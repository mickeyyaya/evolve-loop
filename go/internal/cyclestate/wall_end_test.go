package cyclestate

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWallEnd_RoundTripsThroughJSON(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(WallEnd{CLI: "agy", Pattern: "rate_limit", ResetAt: at, Evidence: "ws/escalation.json"})
	if err != nil {
		t.Fatal(err)
	}
	var back WallEnd
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.CLI != "agy" || back.Pattern != "rate_limit" || !back.ResetAt.Equal(at) || back.Evidence != "ws/escalation.json" {
		t.Errorf("round trip lost a field: %+v", back)
	}
	bare, err := json.Marshal(WallEnd{CLI: "agy"})
	if err != nil {
		t.Fatal(err)
	}
	if string(bare) != `{"cli":"agy"}` {
		t.Errorf("bare end %s, want no false date", bare)
	}
}

func TestWallEnd_MarshalJSONKeepsAResetDateOnlyWhenSet(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	withDate, err := WallEnd{CLI: "agy", ResetAt: at}.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(withDate), "2026-11-06T10:00:00Z") {
		t.Errorf("a set reset date must be written: %s", withDate)
	}
	bare, err := WallEnd{CLI: "agy"}.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bare), "0001") {
		t.Errorf("an unset reset date must not be written: %s", bare)
	}
}
