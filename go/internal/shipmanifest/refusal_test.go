package shipmanifest

import (
	"errors"
	"slices"
	"testing"
)

const refusalHeader = "The following paths are ignored by one of your .gitignore files:\n"

func TestIgnoredInAddRefusal_ParsesTheOffenderList(t *testing.T) {
	stderr := refusalHeader +
		".evolve/inbox/processed\n" +
		".evolve/inbox/rejected\n" +
		"hint: Use -f if you really want to add them.\n" +
		"hint: Disable this message with \"git config set advice.addIgnoredFile false\"\n"
	if got := IgnoredInAddRefusal(stderr); !slices.Equal(got, []string{".evolve/inbox/processed", ".evolve/inbox/rejected"}) {
		t.Fatalf("offenders = %v, want the two paths git named", got)
	}
}

func TestIgnoredInAddRefusal_NoHeaderMeansNoOffenders(t *testing.T) {
	for _, stderr := range []string{"", "fatal: Invalid path '/go'\n", "hint: Use -f if you really want to add them.\n"} {
		if got := IgnoredInAddRefusal(stderr); len(got) != 0 {
			t.Errorf("stderr %q must yield no offenders, got %v — a fuzzy parse would silently under-stage unrelated failures", stderr, got)
		}
	}
}

func TestIgnoredInAddRefusal_DecodesQuotedPaths(t *testing.T) {
	stderr := refusalHeader + `"caf\303\251-dir"` + "\n" + "hint: Use -f if you really want to add them.\n"
	if got := IgnoredInAddRefusal(stderr); !slices.Equal(got, []string{"café-dir"}) {
		t.Fatalf("quoted offender must decode to the on-disk path: %v", got)
	}
}

func TestIgnoredInAddRefusal_StopsAtTheFirstHint(t *testing.T) {
	stderr := refusalHeader + "real-offender\n" + "hint: Use -f if you really want to add them.\n" + "not-an-offender\n"
	if got := IgnoredInAddRefusal(stderr); !slices.Equal(got, []string{"real-offender"}) {
		t.Fatalf("parse must stop at the hint boundary: %v", got)
	}
}

func TestIgnoredInAddRefusal_ReadsTheRefusalInsideAWrappedError(t *testing.T) {
	wrapped := "treefence: git add: exit status 1: " + refusalHeader + "dir/\nhint: Use -f if you really want to add them."
	if got := IgnoredInAddRefusal(wrapped); !slices.Equal(got, []string{"dir/"}) {
		t.Fatalf("a caller's error text that carries git's stderr names the same offenders: %v", got)
	}
}

type stageAttempts struct {
	calls   [][]string
	answers []error
	stderr  []string
}

func (s *stageAttempts) stage(paths []string) (string, error) {
	i := len(s.calls)
	s.calls = append(s.calls, paths)
	return s.stderr[i], s.answers[i]
}

func TestStageRetrying_AnEmptySetStagesNothing(t *testing.T) {
	calls := 0
	staged, refused, err := StageRetrying(nil, func([]string) (string, error) {
		calls++
		return "", nil
	})
	if calls != 0 || staged != nil || refused != nil || err != nil {
		t.Fatalf("`git add -A --` with no pathspec stages the whole tree, so an empty set never reaches stage: calls %d, %v %v %v", calls, staged, refused, err)
	}
}

func TestStageRetrying_DropsWhatGitRefusesAndRetriesOnce(t *testing.T) {
	refused := errors.New("exit 1")
	for _, tc := range []struct {
		name        string
		attempts    stageAttempts
		wantStaged  []string
		wantRefused []string
		wantCalls   int
		wantErr     bool
	}{
		{"a clean add stages the whole set once",
			stageAttempts{answers: []error{nil}, stderr: []string{""}},
			[]string{"a.go", "ign"}, nil, 1, false},
		{"a named ignored path is dropped and the rest retried",
			stageAttempts{answers: []error{refused, nil}, stderr: []string{refusalHeader + "ign\nhint: x\n", ""}},
			[]string{"a.go"}, []string{"ign"}, 2, false},
		{"a retry that fails again reports the retry's failure",
			stageAttempts{answers: []error{refused, refused}, stderr: []string{refusalHeader + "ign\nhint: x\n", "fatal"}},
			[]string{"a.go"}, []string{"ign"}, 2, true},
		{"a failure that names no ignored path is not retried",
			stageAttempts{answers: []error{refused}, stderr: []string{"fatal: Invalid path '/go'\n"}},
			[]string{"a.go", "ign"}, nil, 1, true},
		{"a refusal naming every path is refused, never retried as an empty add",
			stageAttempts{answers: []error{refused}, stderr: []string{refusalHeader + "a.go\nign\nhint: x\n"}},
			[]string{"a.go", "ign"}, nil, 1, true},
		{"a refusal naming only paths outside the set is not retried",
			stageAttempts{answers: []error{refused}, stderr: []string{refusalHeader + "elsewhere\nhint: x\n"}},
			[]string{"a.go", "ign"}, nil, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			staged, dropped, err := StageRetrying([]string{"a.go", "ign"}, tc.attempts.stage)
			if !slices.Equal(staged, tc.wantStaged) || !slices.Equal(dropped, tc.wantRefused) || (err != nil) != tc.wantErr {
				t.Fatalf("staged %v dropped %v err %v", staged, dropped, err)
			}
			if len(tc.attempts.calls) != tc.wantCalls {
				t.Fatalf("attempts %v, want %d", tc.attempts.calls, tc.wantCalls)
			}
		})
	}
}
