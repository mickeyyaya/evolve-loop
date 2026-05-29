package recipe

import (
	"errors"
	"testing"
)

func TestAwaitCompile_Errors(t *testing.T) {
	cases := []struct {
		name    string
		await   Await
		wantErr bool
	}{
		{"prompt_marker ok", Await{Kind: AwaitPromptMarker, TimeoutS: 5}, false},
		{"regex ok", Await{Kind: AwaitRegex, Regex: "done", TimeoutS: 5}, false},
		{"regex missing pattern", Await{Kind: AwaitRegex, TimeoutS: 5}, true},
		{"regex uncompilable", Await{Kind: AwaitRegex, Regex: "([", TimeoutS: 5}, true},
		{"any_of ok", Await{Kind: AwaitAnyOf, Substrs: []string{"a"}, TimeoutS: 5}, false},
		{"any_of empty substrs", Await{Kind: AwaitAnyOf, TimeoutS: 5}, true},
		{"all_of ok", Await{Kind: AwaitAllOf, Substrs: []string{"a", "b"}, TimeoutS: 5}, false},
		{"all_of empty substrs", Await{Kind: AwaitAllOf, TimeoutS: 5}, true},
		{"unknown kind", Await{Kind: "weird", TimeoutS: 5}, true},
		{"fail_regex ok", Await{Kind: AwaitPromptMarker, FailRegex: "error", TimeoutS: 5}, false},
		{"fail_regex uncompilable", Await{Kind: AwaitPromptMarker, FailRegex: "([", TimeoutS: 5}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.await.compile()
			if (err != nil) != tc.wantErr {
				t.Fatalf("compile() err=%v wantErr=%v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidRecipe) {
				t.Errorf("compile() err not ErrInvalidRecipe: %v", err)
			}
		})
	}
}

func TestCompiledAwait_Eval(t *testing.T) {
	cases := []struct {
		name   string
		await  Await
		pane   string
		marker string
		want   matchOutcome
	}{
		{"marker present", Await{Kind: AwaitPromptMarker, TimeoutS: 5}, "ready\n❯ ", "❯", matchSatisfied},
		{"marker absent", Await{Kind: AwaitPromptMarker, TimeoutS: 5}, "thinking...", "❯", matchPending},
		{"empty marker never satisfies", Await{Kind: AwaitPromptMarker, TimeoutS: 5}, "anything", "", matchPending},
		{"regex match", Await{Kind: AwaitRegex, Regex: "Insta(lled)?", TimeoutS: 5}, "Installed plugin", "❯", matchSatisfied},
		{"regex no match", Await{Kind: AwaitRegex, Regex: "Installed", TimeoutS: 5}, "still working", "❯", matchPending},
		{"any_of hit", Await{Kind: AwaitAnyOf, Substrs: []string{"added", "exists"}, TimeoutS: 5}, "marketplace added", "❯", matchSatisfied},
		{"any_of miss", Await{Kind: AwaitAnyOf, Substrs: []string{"added", "exists"}, TimeoutS: 5}, "nope", "❯", matchPending},
		{"all_of all present", Await{Kind: AwaitAllOf, Substrs: []string{"a", "b"}, TimeoutS: 5}, "a and b", "❯", matchSatisfied},
		{"all_of one missing", Await{Kind: AwaitAllOf, Substrs: []string{"a", "b"}, TimeoutS: 5}, "only a", "❯", matchPending},
		{"fail_regex trips before success", Await{Kind: AwaitAnyOf, Substrs: []string{"installed"}, FailRegex: "error", TimeoutS: 5}, "installed but error", "❯", matchFailed},
		{"fail_regex absent, success wins", Await{Kind: AwaitAnyOf, Substrs: []string{"installed"}, FailRegex: "error", TimeoutS: 5}, "installed ok", "❯", matchSatisfied},
		{"empty pane pending", Await{Kind: AwaitAnyOf, Substrs: []string{"x"}, TimeoutS: 5}, "", "❯", matchPending},
		{"multi-line marker on last line", Await{Kind: AwaitPromptMarker, TimeoutS: 5}, "line1\nline2\n❯", "❯", matchSatisfied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ca, err := tc.await.compile()
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if got := ca.eval(tc.pane, tc.marker); got != tc.want {
				t.Errorf("eval = %d, want %d", got, tc.want)
			}
		})
	}
}
