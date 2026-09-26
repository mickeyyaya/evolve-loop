package llmroute

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// want is the chain with excludeProfileCLI=false (Resolve); wantExcl with true (ChainFor).
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
			name:     "profile-cli-in-fallback-is-the-asymmetry",
			primary:  "codex-tmux",
			prof:     &profiles.Profile{CLI: "agy-tmux", CLIFallback: []string{"agy-tmux", "claude-tmux"}},
			want:     []string{"codex-tmux", "agy-tmux", "claude-tmux"},
			wantExcl: []string{"codex-tmux", "claude-tmux"},
		},
		{
			name:     "no-profile-cli-both-shapes-identical",
			primary:  "claude-tmux",
			prof:     &profiles.Profile{CLIFallback: []string{"codex-tmux", "agy-tmux"}},
			want:     []string{"claude-tmux", "codex-tmux", "agy-tmux"},
			wantExcl: []string{"claude-tmux", "codex-tmux", "agy-tmux"},
		},
		{
			name:     "profile-cli-equals-primary",
			primary:  "codex-tmux",
			prof:     &profiles.Profile{CLI: "codex-tmux", CLIFallback: []string{"claude-tmux"}},
			want:     []string{"codex-tmux", "claude-tmux"},
			wantExcl: []string{"codex-tmux", "claude-tmux"},
		},
		{
			name:     "nil-profile-single-candidate",
			primary:  "claude-tmux",
			prof:     nil,
			want:     []string{"claude-tmux"},
			wantExcl: []string{"claude-tmux"},
		},
		{
			name:     "empty-fallback",
			primary:  "claude-tmux",
			prof:     &profiles.Profile{CLI: "agy-tmux"},
			want:     []string{"claude-tmux"},
			wantExcl: []string{"claude-tmux"},
		},
		{
			name:    "trim-drop-empties-and-dedup-first-wins",
			primary: "claude-tmux",
			prof: &profiles.Profile{
				CLIFallback: []string{"", "   ", "  codex-tmux  ", "codex-tmux", "claude-tmux", "\t\n", "agy-tmux"},
			},
			want:     []string{"claude-tmux", "codex-tmux", "agy-tmux"},
			wantExcl: []string{"claude-tmux", "codex-tmux", "agy-tmux"},
		},
		{
			name:     "all-noise-fallback-collapses-to-primary",
			primary:  "agy-tmux",
			prof:     &profiles.Profile{CLI: "agy-tmux", CLIFallback: []string{"", " ", "\t"}},
			want:     []string{"agy-tmux"},
			wantExcl: []string{"agy-tmux"},
		},
	}
}

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

// The env map forces the primary, leaving prof.CLI free to play the swapped-away original.
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

var errTestTriggerExit = &testLaunchErr{}

type testLaunchErr struct{}

func (*testLaunchErr) Error() string { return "test: trigger exit" }
