package inboxmover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetiredAtCycle_ReadsTheConsumedCycleAndRejectsUnreadableRecords(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name string
		path string
		want int
	}{
		{"empty path", "", 0},
		{"missing file", filepath.Join(dir, "absent.json"), 0},
		{"top-level cycle wins", write("a.json", `{"cycle":7,"consumed":{"cycle":3}}`), 7},
		{"consumed cycle", write("b.json", `{"consumed":{"cycle":3}}`), 3},
		{"consumed string cycle", write("c.json", `{"consumed":{"cycle":"9"}}`), 9},
		{"not consumed", write("d.json", `{"id":"x"}`), 0},
		{"malformed", write("e.json", `{"consumed":`), 0},
	}
	for _, c := range cases {
		if got := retiredAtCycle(c.path); got != c.want {
			t.Errorf("%s: retiredAtCycle = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestParseConsumedRecord_RejectsNonScalarCycleAndNonObjectConsumed(t *testing.T) {
	for _, item := range []string{`{"consumed":{"cycle":true}}`, `{"consumed":{"cycle":[1]}}`, `{"consumed":5}`, `{"consumed":{"cycle":1e3}}`} {
		if _, _, err := ParseConsumedRecord([]byte(item)); err == nil {
			t.Errorf("ParseConsumedRecord(%s): want an error", item)
		}
	}
}
