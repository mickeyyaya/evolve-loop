package textcap

import (
	"strings"
	"testing"
)

// Test 10 — the two rune-cap rules the advisor prompt and the carryover
// lifecycle project (ADR-0103 unit 04): distinct markers, rune-safe bounds,
// TrimSpace on one rule only.
func TestTruncateRunes_TrimsCapsAndMarks(t *testing.T) {
	under, at, over := strings.Repeat("é", 499), strings.Repeat("é", 500), strings.Repeat("é", 501)
	if got := TruncateRunes(under, 500); got != under {
		t.Errorf("499 runes under a 500 cap must pass through: %d runes", len([]rune(got)))
	}
	if got := TruncateRunes(at, 500); got != at {
		t.Errorf("exactly 500 runes must pass through: %d runes", len([]rune(got)))
	}
	if got, want := TruncateRunes(over, 500), at+" …[truncated]"; got != want {
		t.Errorf("501 runes cap at 500 + the marker:\n got %q\nwant %q", got[len(got)-20:], want[len(want)-20:])
	}
	if got := TruncateRunes("  padded  ", 500); got != "padded" {
		t.Errorf("surrounding whitespace is trimmed before the bound: %q", got)
	}
	if got := TruncateRunes("   ", 5); got != "" {
		t.Errorf("whitespace-only trims to empty: %q", got)
	}
	if got := TruncateRunes("  "+over+"  ", 500); !strings.HasSuffix(got, " …[truncated]") || len([]rune(got)) != 500+len([]rune(" …[truncated]")) {
		t.Errorf("trim happens before the cap: %d runes", len([]rune(got)))
	}
}

func TestCapRunes_CapsWithTheEllipsisRune(t *testing.T) {
	under, at, over := strings.Repeat("é", 499), strings.Repeat("é", 500), strings.Repeat("é", 501)
	if got := CapRunes(under, 500); got != under {
		t.Errorf("499 runes pass through: %d", len([]rune(got)))
	}
	if got := CapRunes(at, 500); got != at {
		t.Errorf("exactly 500 runes pass through: %d", len([]rune(got)))
	}
	if got, want := CapRunes(over, 500), at+"…"; got != want {
		t.Errorf("501 runes cap at 500 + the ellipsis rune: %q", got[len(got)-6:])
	}
	if got := CapRunes("  padded  ", 500); got != "  padded  " {
		t.Errorf("CapRunes never trims: %q", got)
	}
	if CapRunes(over, 500) == TruncateRunes(over, 500) {
		t.Error("the two rules carry distinct markers — never fold them")
	}
}
