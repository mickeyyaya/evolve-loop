package core

import "testing"

const (
	buildFloorHead = "review gate: phase \"build\" deliverable rejected after 1 correction(s): build handoff floor: "
	bridgeNoise    = "… and its own install under $HOME. The phase sandbox grants no write outside the cell, so the probe runs outside it.\nFAIL\n"
)

func TestNormalizeReasonForFingerprint_PinnedPairs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		a, b      string
		wantMerge bool
	}{
		{
			name: "2071/2072 scout bridge timeout differs only in cycle=N",
			a: "phase scout: scout: bridge: bridge: launch exit=81: artifact-timeout: cause=submit_wedged reason=\"nudge submit_wedged (resends=3)\" " +
				"phase=scout cycle=2071 driver=claude-tmux artifact=\"scout-report.md\" waited=300s: core: bridge artifact timeout",
			b: "phase scout: scout: bridge: bridge: launch exit=81: artifact-timeout: cause=submit_wedged reason=\"nudge submit_wedged (resends=3)\" " +
				"phase=scout cycle=2072 driver=claude-tmux artifact=\"scout-report.md\" waited=300s: core: bridge artifact timeout",
			wantMerge: true,
		},
		{
			name: "2049/2051 build handoff floor differs only in cycle suffixes and finding counts",
			a: buildFloorHead + "2 deterministic check failure(s) — fix these exactly before handoff:\n" +
				"  [ste-lint] the changed documents fail the STE lint: 3 finding(s) in 2 file(s) " +
				"(first: docs/explain/builds/cycle-2049-01m6cy05n0qsch7y9v4apqg24d.md:15 STE-SENTENCE); fix each finding\n" +
				"  ./acs/cycle2049 (-tags acs): unit tests FAIL\n" + bridgeNoise +
				"FAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle2049\t1.937s\n" +
				"worktree .evolve/worktrees/cycle-cd3ae73e-2049/go phase=build cycle=2049",
			b: buildFloorHead + "2 deterministic check failure(s) — fix these exactly before handoff:\n" +
				"  [ste-lint] the changed documents fail the STE lint: 5 finding(s) in 1 file(s) " +
				"(first: docs/explain/builds/cycle-2051-01m6cy3x8e7tq2w9d5h4ncr0fa.md:15 STE-SENTENCE); fix each finding\n" +
				"  ./acs/cycle2051 (-tags acs): unit tests FAIL\n" + bridgeNoise +
				"FAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle2051\t2.280s\n" +
				"worktree .evolve/worktrees/cycle-0b12ef44-2051/go phase=build cycle=2051",
			wantMerge: true,
		},
		{
			name: "2054/2093 two different build defects stay apart",
			a: buildFloorHead + "1 deterministic check failure(s) — fix these exactly before handoff:\n" +
				"  ./acs/cycle2054 (-tags acs): unit tests FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle2054\t1.100s",
			b: buildFloorHead + "1 deterministic check failure(s) — fix these exactly before handoff:\n" +
				"  Explanation Documentation: Changed Areas does not explain material path go/internal/repocontract/repocontract.go",
			wantMerge: false,
		},
		{
			name: "2073/2089 build floors with a different number of failing checks stay apart",
			a: buildFloorHead + "3 deterministic check failure(s) — fix these exactly before handoff:\n" +
				"  [ste-lint] the changed documents fail the STE lint: 1 finding(s) in 1 file(s) " +
				"(first: docs/explain/builds/cycle-2073-01m6cy05n0qsch7y9v4apqg24d.md:15 STE-SENTENCE); fix each finding\n" +
				"  ./acs/cycle2073 (-tags acs): unit tests FAIL\n" + bridgeNoise +
				"FAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle2073\t1.937s",
			b: buildFloorHead + "2 deterministic check failure(s) — fix these exactly before handoff:\n" +
				"  ./acs/cycle2089 (-tags acs): unit tests FAIL\n" + bridgeNoise +
				"FAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle2089\t2.280s",
			wantMerge: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := normalizeReasonForFingerprint(tc.a), normalizeReasonForFingerprint(tc.b)
			if merged := a == b; merged != tc.wantMerge {
				t.Errorf("merged=%v, want %v\n a -> %q\n b -> %q", merged, tc.wantMerge, a, b)
			}
		})
	}
}

func TestNormalizeReasonForFingerprint_CycleAndCountShapesFold(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		a, b string
	}{
		{"cycle=N", "driver=x cycle=2049 phase=build", "driver=x cycle=2051 phase=build"},
		{"cycle glued to its number", "./acs/cycle2049 (-tags acs): unit tests FAIL", "./acs/cycle2051 (-tags acs): unit tests FAIL"},
		{"cycle with a colon", "refused for cycle:2049", "refused for cycle:2051"},
		{"cycle with an underscore", "refused for cycle_2049", "refused for cycle_2051"},
		{"lane hash that starts with a letter", "wrote outside .evolve/worktrees/cycle-cd3ae73e-2408/go", "wrote outside .evolve/worktrees/cycle-0b12ef44-2409/go"},
		{
			"cycle number followed by a ULID",
			"docs/explain/builds/cycle-2049-01m6cy05n0qsch7y9v4apqg24d.md:15",
			"docs/explain/builds/cycle-2051-01M6CY3X8E7TQ2W9D5H4NCR0FA.md:15",
		},
		{"finding count and file count", "fail: 1 finding(s) in 1 file(s)", "fail: 7 finding(s) in 3 file(s)"},
		{"plural finding count and singular file", "fail: 2 findings in 1 file", "fail: 1 finding in 4 files"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := normalizeReasonForFingerprint(tc.a), normalizeReasonForFingerprint(tc.b); got != want {
				t.Errorf("variance still splits the fingerprint:\n a -> %q\n b -> %q", got, want)
			}
		})
	}
}

func TestNormalizeReasonForFingerprint_CountOfFailingChecksStaysIdentity(t *testing.T) {
	t.Parallel()

	a := normalizeReasonForFingerprint(buildFloorHead + "3 deterministic check failure(s) — fix these exactly before handoff:")
	b := normalizeReasonForFingerprint(buildFloorHead + "2 deterministic check failure(s) — fix these exactly before handoff:")
	if a == b {
		t.Errorf("the number of failing checks is defect identity but folded: %q", a)
	}
}
