package quotareset

import (
	"testing"
	"time"
)

func TestParseISO_ReadsBothLayoutsAndRejectsTheRest(t *testing.T) {
	t.Parallel()
	want := time.Date(2026, 11, 4, 0, 53, 30, 0, time.UTC)
	written := resultAt(want.In(time.FixedZone("+0800", 8*3600)), "bench").ISO
	for _, s := range []string{written, want.Format(time.RFC3339), "2026-11-04T08:53:30+08:00"} {
		got, err := ParseISO(s)
		if err != nil || !got.Equal(want) {
			t.Errorf("ParseISO(%q) = %v err %v, want %v", s, got, err, want)
		}
	}
	if _, err := ParseISO("08:53 tomorrow"); err == nil {
		t.Error("ParseISO accepted a value in neither layout")
	}
}
