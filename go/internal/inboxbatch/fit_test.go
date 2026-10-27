package inboxbatch

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFit_BoundsAFieldAtARuneAndStripsControlBytes(t *testing.T) {
	long := strings.Repeat("é", MaxFieldLen)
	got := Fit("a\tb\n"+long, MaxFieldLen)
	if len(got) > MaxFieldLen || !utf8.ValidString(got) || strings.ContainsAny(got, "\t\n") || !strings.HasPrefix(got, "a b ") {
		t.Errorf("Fit = %q (%d bytes), want at most %d valid bytes with control bytes as spaces", got, len(got), MaxFieldLen)
	}
	if short := Fit("ok", MaxAcceptanceLen); short != "ok" {
		t.Errorf("Fit of a short clean value = %q, want it unchanged", short)
	}
	if MaxAcceptanceLen <= MaxFieldLen {
		t.Errorf("MaxAcceptanceLen %d must exceed MaxFieldLen %d", MaxAcceptanceLen, MaxFieldLen)
	}
}

func TestConsumedDirName_IsTheFolderTheConsumeStepWrites(t *testing.T) {
	if ConsumedDirName != "consumed" {
		t.Errorf("ConsumedDirName = %q, want the consumed folder", ConsumedDirName)
	}
}
