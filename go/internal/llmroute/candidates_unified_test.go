package llmroute

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Cycle-1265 RED contract for task `unify-llmroute-candidate-chain-builders`.
//
// Before this cycle the "primary first, then deduped profile.cli_fallback"
// chain was implemented TWICE: candidatesFrom (llmroute.go:216, used by
// Resolve) and chainCandidates (dispatch.go:77, used by ChainFor). The only
// difference is that ChainFor's copy also excludes prof.CLI — the original
// primary the composition root deliberately swapped away from (dispatch.go
// doc comment) — while Resolve's copy intentionally KEEPS the profile chain
// intact after a policy pin (llmroute.go Resolve doc comment).
//
// The contract frozen here: ONE builder with that difference as an explicit
// parameter.
//
//	func buildCandidates(primary string, prof *profiles.Profile, excludeProfileCLI bool) []string
//
// living in dispatch.go (already the Plan/Dispatch home), with candidatesFrom
// and chainCandidates both DELETED, Resolve calling
// buildCandidates(primary, prof, false) and ChainFor calling
// buildCandidates(primary, prof, true). Both documented behaviours are
// preserved verbatim — this is a dedup, not a behaviour change.
//
// DO NOT MODIFY (TDD handoff, doNotModifyTests:true).

// unifiedCase is one row of the shared builder's behavioural matrix. want is
// the chain for excludeProfileCLI=false (Resolve's shape); wantExcl is the
// chain for excludeProfileCLI=true (ChainFor's shape).
type unifiedCase struct {
	name     string
	primary  string
	prof     *profiles.Profile
	want     []string
	wantExcl []string
}

func unifiedCases() []unifiedCase {
	return []unifiedCase{
		{
			// The asymmetry itself: prof.CLI names the swapped-away original
			// primary. Resolve keeps it (pinned phases retain CLI-failure
			// resilience); ChainFor drops it (never walk back into the CLI the
			// composition root routed away from).
			name:     "profile-cli-in-fallback-is-the-asymmetry",
			primary:  "codex-tmux",
			prof:     &profiles.Profile{CLI: "agy-tmux", CLIFallback: []string{"agy-tmux", "claude-tmux"}},
			want:     []string{"codex-tmux", "agy-tmux", "claude-tmux"},
			wantExcl: []string{"codex-tmux", "claude-tmux"},
		},
		{
			// Exclusion is inert when prof.CLI is absent — both shapes agree.
			name:     "no-profile-cli-both-shapes-identical",
			primary:  "claude-tmux",
			prof:     &profiles.Profile{CLIFallback: []string{"codex-tmux", "agy-tmux"}},
			want:     []string{"claude-tmux", "codex-tmux", "agy-tmux"},
			wantExcl: []string{"claude-tmux", "codex-tmux", "agy-tmux"},
		},
		{
			// Exclusion is inert when prof.CLI IS the explicit primary.
			name:     "profile-cli-equals-primary",
			primary:  "codex-tmux",
			prof:     &profiles.Profile{CLI: "codex-tmux", CLIFallback: []string{"claude-tmux"}},
			want:     []string{"codex-tmux", "claude-tmux"},
			wantExcl: []string{"codex-tmux", "claude-tmux"},
		},
		{
			// Edge: nil profile must never panic and must degrade to one candidate.
			name:     "nil-profile-single-candidate",
			primary:  "claude-tmux",
			prof:     nil,
			want:     []string{"claude-tmux"},
			wantExcl: []string{"claude-tmux"},
		},
		{
			// Edge: empty fallback list.
			name:     "empty-fallback",
			primary:  "claude-tmux",
			prof:     &profiles.Profile{CLI: "agy-tmux"},
			want:     []string{"claude-tmux"},
			wantExcl: []string{"claude-tmux"},
		},
		{
			// Edge/OOD: whitespace-only and empty entries are dropped, real
			// entries are trimmed, first occurrence wins, dup-of-primary dropped.
			name:    "trim-drop-empties-and-dedup-first-wins",
			primary: "claude-tmux",
			prof: &profiles.Profile{
				CLIFallback: []string{"", "   ", "  codex-tmux  ", "codex-tmux", "claude-tmux", "\t\n", "agy-tmux"},
			},
			want:     []string{"claude-tmux", "codex-tmux", "agy-tmux"},
			wantExcl: []string{"claude-tmux", "codex-tmux", "agy-tmux"},
		},
		{
			// Edge: fallback that is ONLY noise collapses to the primary alone —
			// never an empty or nil-padded chain (Dispatch fails loudly on an
			// empty chain, so the builder must always seed >=1 candidate).
			name:     "all-noise-fallback-collapses-to-primary",
			primary:  "agy-tmux",
			prof:     &profiles.Profile{CLI: "agy-tmux", CLIFallback: []string{"", " ", "\t"}},
			want:     []string{"agy-tmux"},
			wantExcl: []string{"agy-tmux"},
		},
	}
}

// TestUnifiedCandidateBuilder_SharedCore pins the single builder's behaviour on
// both settings of the exclusion parameter. This is the function that must
// REPLACE candidatesFrom and chainCandidates, not join them.
func TestUnifiedCandidateBuilder_SharedCore(t *testing.T) {
	for _, tc := range unifiedCases() {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildCandidates(tc.primary, tc.prof, false); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("buildCandidates(%q, prof, false)=%v, want %v", tc.primary, got, tc.want)
			}
			if got := buildCandidates(tc.primary, tc.prof, true); !reflect.DeepEqual(got, tc.wantExcl) {
				t.Errorf("buildCandidates(%q, prof, true)=%v, want %v", tc.primary, got, tc.wantExcl)
			}
		})
	}
}

// TestUnifiedCandidateBuilder_ExcludeIsNegative is the negative axis: with
// excludeProfileCLI=true the swapped-away prof.CLI must be ABSENT from the
// chain entirely. An implementation that merely reorders (or that ignores the
// parameter and returns Resolve's inclusive chain) fails here.
func TestUnifiedCandidateBuilder_ExcludeIsNegative(t *testing.T) {
	prof := &profiles.Profile{
		CLI:         "agy-tmux",
		CLIFallback: []string{"claude-tmux", "agy-tmux", "  agy-tmux  "},
	}
	got := buildCandidates("codex-tmux", prof, true)
	for _, c := range got {
		if c == "agy-tmux" {
			t.Fatalf("buildCandidates(..., true)=%v: prof.CLI %q must be excluded from the chain", got, prof.CLI)
		}
	}
	if len(got) == 0 || got[0] != "codex-tmux" {
		t.Fatalf("buildCandidates(..., true)=%v: explicit primary must lead the chain", got)
	}
}

// TestUnifiedCandidateBuilder_ChainForDelegates is the CALLER PROOF for
// ChainFor: the production entry point must produce exactly what the shared
// builder produces with the exclusion ON. A ChainFor that kept its own private
// copy of the loop would drift from this the moment either is edited.
func TestUnifiedCandidateBuilder_ChainForDelegates(t *testing.T) {
	for _, tc := range unifiedCases() {
		t.Run(tc.name, func(t *testing.T) {
			got := ChainFor(tc.primary, tc.prof).Candidates
			want := buildCandidates(tc.primary, tc.prof, true)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("ChainFor(%q, prof).Candidates=%v, want buildCandidates(...,true)=%v", tc.primary, got, want)
			}
			if !reflect.DeepEqual(got, tc.wantExcl) {
				t.Errorf("ChainFor(%q, prof).Candidates=%v, want %v (behaviour must be unchanged by the dedup)", tc.primary, got, tc.wantExcl)
			}
		})
	}
}

// TestUnifiedCandidateBuilder_ResolveDelegates is the CALLER PROOF for Resolve:
// the production entry point must produce exactly what the shared builder
// produces with the exclusion OFF. The primary is forced via the env map
// (tier 1 of envchain, ahead of profile.cli) so prof.CLI stays free to play the
// "swapped-away original" role the exclusion parameter is about.
func TestUnifiedCandidateBuilder_ResolveDelegates(t *testing.T) {
	for _, tc := range unifiedCases() {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"EVOLVE_AUDITOR_CLI": tc.primary}
			got := Resolve("auditor", "audit", "auto", env, tc.prof, nil, nil).Candidates
			want := buildCandidates(tc.primary, tc.prof, false)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Resolve(...).Candidates=%v, want buildCandidates(...,false)=%v", got, want)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Resolve(...).Candidates=%v, want %v (behaviour must be unchanged by the dedup)", got, tc.want)
			}
		})
	}
}

// TestUnifiedCandidateBuilder_DispatchWalkerUntouched guards the out-of-scope
// half of the acceptance criteria: Dispatch's walk and resolveTriggers are
// already single-authority and must survive the dedup unchanged.
func TestUnifiedCandidateBuilder_DispatchWalkerUntouched(t *testing.T) {
	prof := &profiles.Profile{CLI: "agy-tmux", CLIFallback: []string{"claude-tmux"}, CLIFallbackOnExit: []int{81}}
	plan := ChainFor("codex-tmux", prof)
	if !reflect.DeepEqual(plan.Triggers, []int{81}) {
		t.Errorf("Triggers=%v, want [81] (resolveTriggers unchanged)", plan.Triggers)
	}
	var attempts []string
	res := Dispatch(plan, func(cli string) (int, error) {
		attempts = append(attempts, cli)
		if cli == "codex-tmux" {
			return 81, errTestTriggerExit
		}
		return 0, nil
	})
	if !reflect.DeepEqual(attempts, []string{"codex-tmux", "claude-tmux"}) {
		t.Errorf("Dispatch attempts=%v, want [codex-tmux claude-tmux] (trigger exit advances the unified chain)", attempts)
	}
	if res.Err != nil {
		t.Errorf("Dispatch Err=%v, want nil (second candidate succeeded)", res.Err)
	}
}

// errTestTriggerExit is the launch error paired with a trigger exit code.
var errTestTriggerExit = &testLaunchErr{}

type testLaunchErr struct{}

func (*testLaunchErr) Error() string { return "test: trigger exit" }
