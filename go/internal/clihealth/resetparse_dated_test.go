package clihealth

import (
	"strings"
	"testing"
	"time"
)

const codexDatedWall = "■ You've hit your usage limit. Upgrade to Plus to continue using Codex (https://chatgpt.com/explore/plus), or try again at Oct 14th, 2026 4:13 PM."

func TestParseResetHint_ADatedWall(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("TEST", 8*3600)
	now := time.Date(2026, 9, 30, 13, 22, 0, 0, loc)

	cases := []struct {
		name string
		pane string
		want time.Time
		ok   bool
	}{
		{"a two-week wall is re-checked daily, so an early lift is seen within a day", codexDatedWall, now.Add(24 * time.Hour), true},
		{"a dated wall within the day is its exact instant", "try again at Sep 30th, 2026 6:13 PM", time.Date(2026, 9, 30, 18, 15, 0, 0, loc), true},
		{"a full month name without an ordinal", "try again at September 30, 2026 6:13 PM", time.Date(2026, 9, 30, 18, 15, 0, 0, loc), true},
		{"a dated wall already past is no hint", "try again at Sep 29th, 2026 4:13 PM", time.Time{}, false},
		{"an unknown month is no hint", "try again at Foo 14th, 2026 4:13 PM", time.Time{}, false},
		{"an impossible day is no hint", "try again at Feb 30th, 2027 4:13 PM", time.Time{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ParseResetHint(c.pane, now)
			if ok != c.ok || !got.Equal(c.want) {
				t.Errorf("ParseResetHint(%q) = (%v, %v), want (%v, %v)", c.pane, got, ok, c.want, c.ok)
			}
		})
	}
}

func TestParseResetHint_ADatedWallAcrossTheYearEnd(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("TEST", 8*3600)
	now := time.Date(2026, 12, 31, 20, 0, 0, 0, loc)

	cases := []struct {
		name string
		pane string
		want time.Time
		ok   bool
	}{
		{"January of the printed year, not now's year", "try again at Jan 1st, 2027 1:13 AM", time.Date(2027, 1, 1, 1, 15, 0, 0, loc), true},
		{"December, the last month", "try again at Dec 31st, 2026 11:13 PM", time.Date(2026, 12, 31, 23, 15, 0, 0, loc), true},
		{"an impossible clock time is no hint", "try again at Jan 1st, 2027 13:13 PM", time.Time{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ParseResetHint(c.pane, now)
			if ok != c.ok || !got.Equal(c.want) {
				t.Errorf("ParseResetHint(%q) = (%v, %v), want (%v, %v)", c.pane, got, ok, c.want, c.ok)
			}
		})
	}
}

func TestNewBenchEntry_ADatedWallBenchesADayAndKeepsTheBannerAsEvidence(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("TEST", 8*3600)
	now := time.Date(2026, 9, 30, 13, 22, 0, 0, loc)
	pane := "    <artifact-path>/workspace/scout-report.md</artifact-path>\n" + codexDatedWall + "\n› Ask Codex to do anything\n"

	entry := NewBenchEntry(Entry{Strikes: 32}, "codex", "rate_limit", pane, now)

	if want := now.Add(24 * time.Hour); !entry.BenchedUntil.Equal(want) {
		t.Errorf("BenchedUntil = %v, want %v: the stated reset parses (capped at a day), not the strike-scaled cooldown", entry.BenchedUntil, want)
	}
	if !strings.Contains(entry.Evidence, "try again at Oct 14th, 2026 4:13 PM") {
		t.Errorf("Evidence = %q, want the wall banner with its stated reset, not the pasted prompt's first line", entry.Evidence)
	}
}
