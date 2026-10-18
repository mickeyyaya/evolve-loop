package llmroute

import (
	"reflect"
	"testing"
)

func ollamaIsToolless(driver string) bool { return driver == "ollama-tmux" }

func TestDropToolless_DropsEveryCandidateTheHostFlagsAsToolless(t *testing.T) {
	cases := []struct {
		name        string
		candidates  []string
		wantKept    []string
		wantDropped []string
	}{
		{"a toolless primary leaves the fallbacks in order", []string{"ollama-tmux", "claude-tmux", "agy-tmux"}, []string{"claude-tmux", "agy-tmux"}, []string{"ollama-tmux"}},
		{"a toolless fallback leaves the primary first", []string{"claude-tmux", "ollama-tmux", "agy-tmux"}, []string{"claude-tmux", "agy-tmux"}, []string{"ollama-tmux"}},
		{"a bare family is judged as its default driver", []string{"ollama", "claude"}, []string{"claude"}, []string{"ollama"}},
		{"a name the host does not flag is left to the launch", []string{"frobnicate", "claude-tmux"}, []string{"frobnicate", "claude-tmux"}, nil},
		{"a chain of tool-capable drivers is untouched", []string{"claude-tmux", "agy-tmux"}, []string{"claude-tmux", "agy-tmux"}, nil},
		{"a chain of toolless drivers is left empty", []string{"ollama-tmux", "ollama"}, nil, []string{"ollama-tmux", "ollama"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := Plan{Candidates: tc.candidates, Triggers: []int{80}, PrimarySource: "profile.builder.cli", Model: "deep", Tiers: []string{"deep"}}
			before := append([]string(nil), in.Candidates...)

			got, dropped := DropToolless(in, ollamaIsToolless)

			if !reflect.DeepEqual(got.Candidates, tc.wantKept) || !reflect.DeepEqual(dropped, tc.wantDropped) {
				t.Fatalf("DropToolless(%v) = kept %v dropped %v, want kept %v dropped %v", tc.candidates, got.Candidates, dropped, tc.wantKept, tc.wantDropped)
			}
			if got.PrimarySource != in.PrimarySource || got.Model != in.Model || !reflect.DeepEqual(got.Triggers, in.Triggers) || !reflect.DeepEqual(got.Tiers, in.Tiers) {
				t.Fatalf("every other field of the plan is carried through: %+v", got)
			}
			if !reflect.DeepEqual(in.Candidates, before) {
				t.Fatalf("the input plan is never mutated: %v, was %v", in.Candidates, before)
			}
		})
	}
}

func TestDropToolless_WithNoHostPredicateTheChainIsUntouched(t *testing.T) {
	in := Plan{Candidates: []string{"ollama-tmux", "claude-tmux"}}

	got, dropped := DropToolless(in, nil)

	if !reflect.DeepEqual(got.Candidates, in.Candidates) || dropped != nil {
		t.Fatalf("a host that flags nothing drops nothing: kept %v dropped %v", got.Candidates, dropped)
	}
}
