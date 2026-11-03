package core

import "testing"

const (
	measuredReasonCycle2071 = "phase scout: scout: bridge: bridge: launch exit=81: artifact-timeout: cause=submit_wedged reason=\"nudge submit_wedged (resends=3)\" phase=scout cycle=2071 driver=claude-tmux artifact=\"scout-report.md\" waited=300s interval=300s extends_used=0 max_extends=6 last_review=pause liveness=idle progressed=false busy=false transient=false detector_error=\"\": core: bridge artifact timeout; the dispatch walk claude-tmux@balanced=81 -> agy-tmux@balanced=85 -> agy-claude-tmux@balanced=85 met a quota wall, but rung claude-tmux@balanced failed with no evidence of exhaustion, so the walk is not a quota pause"
	measuredReasonCycle2072 = "phase scout: scout: bridge: bridge: launch exit=81: artifact-timeout: cause=submit_wedged reason=\"nudge submit_wedged (resends=3)\" phase=scout cycle=2072 driver=claude-tmux artifact=\"scout-report.md\" waited=300s interval=300s extends_used=0 max_extends=6 last_review=pause liveness=idle progressed=false busy=false transient=false detector_error=\"\": core: bridge artifact timeout; the dispatch walk claude-tmux@balanced=81 -> agy-tmux@balanced=85 -> agy-claude-tmux@balanced=85 met a quota wall, but rung claude-tmux@balanced failed with no evidence of exhaustion, so the walk is not a quota pause"
	measuredReasonCycle2049 = "review gate: phase \"build\" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff:\n  [ste-lint] the changed documents fail the STE lint: 1 finding(s) in 1 file(s) (first: docs/explain/builds/cycle-2049-01m5xtvnwk94g9ryfgzhqre9m0.md:8 STE-SENTENCE); fix each finding; `evolve docs ste-lint --strict --changed <base> --added` lists them"
	measuredReasonCycle2051 = "review gate: phase \"build\" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff:\n  [ste-lint] the changed documents fail the STE lint: 2 finding(s) in 1 file(s) (first: docs/explain/builds/cycle-2051-01m5xwqdjqyc9mhva9bxdvs52e.md:8 STE-SENTENCE); fix each finding; `evolve docs ste-lint --strict --changed <base> --added` lists them"
	measuredReasonCycle2054 = "review gate: phase \"build\" deliverable rejected after 1 correction(s): build handoff floor: 2 deterministic check failure(s) — fix these exactly before handoff:\n  Explanation Documentation: Changed Areas does not explain material path go/internal/routingtest/engine.go\n  Explanation Documentation: Changed Areas does not explain material path go/internal/routingtest/fakes.go"
	measuredReasonCycle2093 = "review gate: phase \"build\" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff:\n  Explanation Documentation: Changed Areas does not explain material path go/internal/repocontract/repocontract.go"
	measuredReasonCycle2073 = "review gate: phase \"build\" deliverable rejected after 1 correction(s): build handoff floor: 3 deterministic check failure(s) — fix these exactly before handoff:\n  [ste-lint] the changed documents fail the STE lint: 1 finding(s) in 1 file(s) (first: docs/explain/builds/cycle-2073-01m6cy05n0qsch7y9v4apqg24d.md:15 STE-SENTENCE); fix each finding; `evolve docs ste-lint --strict --changed <base> --added` lists them\n  ./acs/cycle2073 (-tags acs): unit tests FAIL\n… and its own install under $HOME. The phase sandbox grants no write outside the cell, so the probe runs outside it.\n[bridge] probe ollama runs unsandboxed: A version or model probe of ollama talks to the local ollama server and reads its model store under $HOME. The phase sandbox grants neither, so the probe runs outside it.\nFAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle2073\t1.937s\nFAIL\n\n  enforced package tests FAIL (coverage run doubles as their selfcheck):\n[bridge:manifest] cli=fake-alt-tmux declares no capabilities: it has no tool use. A manifest written before X9 also lacks parsers, collectors, transcript and context_window; add them or name a base.\n[bridge:manifest] cli=f1-badregex declares no capabilities: it has no tool use. A manifest written before X9 also lacks parsers, collectors, transcript and context_window; add them or name a base.\n[bridge:manifest] cli=f1-badregex declares no capabilities: it has no tool use. A manifest written before X9 also lacks parsers, collectors, transcript and context_window; add them or name a base.\n[bridge…"
	measuredReasonCycle2089 = "review gate: phase \"build\" deliverable rejected after 1 correction(s): build handoff floor: 2 deterministic check failure(s) — fix these exactly before handoff:\n  ./acs/cycle2089 (-tags acs): unit tests FAIL\n… and its own install under $HOME. The phase sandbox grants no write outside the cell, so the probe runs outside it.\n[bridge] probe ollama runs unsandboxed: A version or model probe of ollama talks to the local ollama server and reads its model store under $HOME. The phase sandbox grants neither, so the probe runs outside it.\nFAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle2089\t2.280s\nFAIL\n\n  enforced package tests FAIL (coverage run doubles as their selfcheck):\n[bridge:manifest] cli=fake-alt-tmux declares no capabilities: it has no tool use. A manifest written before X9 also lacks parsers, collectors, transcript and context_window; add them or name a base.\n[bridge:manifest] cli=f1-badregex declares no capabilities: it has no tool use. A manifest written before X9 also lacks parsers, collectors, transcript and context_window; add them or name a base.\n[bridge:manifest] cli=f1-badregex declares no capabilities: it has no tool use. A manifest written before X9 also lacks parsers, collectors, transcript and context_window; add them or name a base.\n[bridge…"
)

func TestNormalizeReasonForFingerprint_MeasuredNearDuplicatePairsShareOneFingerprint(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		phase, class string
		a, b         string
		wantSame     bool
	}{
		{"2071/2072 differ only in cycle=N", "scout", "infra-error", measuredReasonCycle2071, measuredReasonCycle2072, true},
		{"2049/2051 differ in a cycle-N-ulid path and a finding count", "build", "gate-block", measuredReasonCycle2049, measuredReasonCycle2051, true},
		{"2054/2093 name different unexplained paths", "build", "gate-block", measuredReasonCycle2054, measuredReasonCycle2093, false},
		{"2073/2089 differ by a whole ste-lint check", "build", "infra-error", measuredReasonCycle2073, measuredReasonCycle2089, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := fingerprint(tc.phase, tc.class, []string{tc.a})
			b := fingerprint(tc.phase, tc.class, []string{tc.b})
			if (a == b) != tc.wantSame {
				t.Errorf("same fingerprint = %v, want %v\n a -> %q\n b -> %q", a == b, tc.wantSame,
					normalizeReasonForFingerprint(tc.a), normalizeReasonForFingerprint(tc.b))
			}
		})
	}
}

func TestNormalizeReasonForFingerprint_EachFoldRewritesOnlyItsToken(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{"phase=scout cycle=2071 driver=claude-tmux", "phase=scout cycle=N driver=claude-tmux"},
		{"recycle=12 stays", "recycle=12 stays"},
		{"docs/explain/builds/cycle-2049-01m5xtvnwk94g9ryfgzhqre9m0.md:8", "docs/explain/builds/cycle-N-<ulid>.md:8"},
		{"runs/cycle-2049-01M5XTVNWK94G9RYFGZHQRE9M0/x", "runs/cycle-N-<ulid>/x"},
		{"the STE lint: 2 finding(s) in 1 file(s)", "the STE lint: <n> finding(s) in 1 file(s)"},
		{"12 findings and 1 finding", "<n> findings and <n> finding"},
		{"2 deterministic check failure(s)", "2 deterministic check failure(s)"},
	} {
		if got := normalizeReasonForFingerprint(tc.in); got != tc.want {
			t.Errorf("normalizeReasonForFingerprint(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
