package ship

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/credsource"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
)

const shadowAnswer = "[credential] shadow: source stub answered for github.com; the push and gh keep today's credential path"

type stubCredentialSource struct{}

func (stubCredentialSource) Name() string { return "stub" }

func (stubCredentialSource) Lookup(context.Context, string) (credsource.Answer, error) {
	return credsource.Answer{Source: "stub", Secret: "tok-1"}, nil
}

func openStub() (credsource.Source, error) { return stubCredentialSource{}, nil }

func writeCredentialStage(t *testing.T, root, stage string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"isolation":{"credential_source":{"stage":"` + stage + `"}}}`
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialReport_ReadsTheStageOfTheRootAndNamesTheSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if got := CredentialReport(context.Background(), root, openStub); got != "" {
		t.Fatalf("no policy: %q, want no line", got)
	}
	writeCredentialStage(t, root, "shadow")
	if got := CredentialReport(context.Background(), root, openStub); got != shadowAnswer {
		t.Fatalf("shadow: %q, want %q", got, shadowAnswer)
	}
}

type deadlineSource struct {
	err  error
	left *time.Duration
}

func (deadlineSource) Name() string { return "deadline" }

func (d deadlineSource) Lookup(ctx context.Context, _ string) (credsource.Answer, error) {
	if dl, ok := ctx.Deadline(); ok {
		*d.left = time.Until(dl)
	}
	if d.err != nil {
		return credsource.Answer{}, d.err
	}
	return credsource.Answer{Source: "deadline", Secret: "tok-9"}, nil
}

func TestCredentialReport_EachStageAndEachAnswer(t *testing.T) {
	t.Parallel()
	const tail = "; the push and gh keep today's credential path"
	fail := func() (credsource.Source, error) { return nil, errors.New("must not open") }
	for _, tc := range []struct {
		name   string
		policy string
		open   func(left *time.Duration) func() (credsource.Source, error)
		want   string
	}{
		{name: "off", policy: `{"isolation":{"credential_source":{"stage":"off"}}}`,
			open: func(*time.Duration) func() (credsource.Source, error) { return fail }, want: ""},
		{name: "the stage did not load", policy: `{"isolation":{"credential_source":{"stage":"enforce"}}}`,
			open: func(*time.Duration) func() (credsource.Source, error) { return fail },
			want: "[credential] the stage did not load: "},
		{name: "no source", policy: `{"isolation":{"credential_source":{"stage":"shadow"}}}`,
			open: func(*time.Duration) func() (credsource.Source, error) {
				return func() (credsource.Source, error) { return nil, errors.New("no realization for plan9") }
			},
			want: "[credential] shadow: no source for github.com: no realization for plan9" + tail},
		{name: "not answered", policy: `{"isolation":{"credential_source":{"stage":"shadow"}}}`,
			open: func(left *time.Duration) func() (credsource.Source, error) {
				return func() (credsource.Source, error) {
					return deadlineSource{err: context.DeadlineExceeded, left: left}, nil
				}
			},
			want: "[credential] shadow: source deadline did not answer for github.com: context deadline exceeded" + tail},
		{name: "answered", policy: `{"isolation":{"credential_source":{"stage":"shadow"}}}`,
			open: func(left *time.Duration) func() (credsource.Source, error) {
				return func() (credsource.Source, error) { return deadlineSource{left: left}, nil }
			},
			want: "[credential] shadow: source deadline answered for github.com" + tail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(tc.policy), 0o644); err != nil {
				t.Fatal(err)
			}
			var left time.Duration
			got := CredentialReport(context.Background(), root, tc.open(&left))
			if !strings.HasPrefix(got, tc.want) || (tc.name != "the stage did not load" && got != tc.want) {
				t.Fatalf("CredentialReport = %q, want %q", got, tc.want)
			}
			if strings.Contains(got, "tok-9") {
				t.Fatal("the report carries the secret")
			}
			if strings.HasPrefix(tc.name, "answered") || strings.HasPrefix(tc.name, "not answered") {
				if left <= 0 || left > credentialProbeTimeout {
					t.Fatalf("the lookup had %v left, want a deadline within %v", left, credentialProbeTimeout)
				}
			}
		})
	}
}

func TestWiredLanding_ThePushLogsWhichCredentialSourceAnsweredAndKeepsTodaysPush(t *testing.T) {
	t.Parallel()
	opts, _, _ := pinOptions(t, ClassCycle)
	writeCredentialStage(t, opts.ProjectRoot, "shadow")
	opts.credentialSource = openStub
	r := newArgvRecorder(opts)
	scriptGreenPost(r)
	res := &RunResult{}
	if err := pushWithRepair(context.Background(), opts, res, landing.PushRequest{Branch: pinBranch, Site: landing.SiteDirect}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Logs, shadowAnswer) {
		t.Fatalf("logs = %q, want %q", res.Logs, shadowAnswer)
	}
	if argv := r.argvs(); len(argv) == 0 || argv[0] != "push origin "+pinBranch+" [streams]" {
		t.Fatalf("git calls = %v, want today's push first", argv)
	}
}
