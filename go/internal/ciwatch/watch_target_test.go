package ciwatch

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func fakeGH(t *testing.T, answer func(args []string) ([]byte, error)) *[][]string {
	t.Helper()
	orig := execCapture
	t.Cleanup(func() { execCapture = orig })
	var calls [][]string
	execCapture = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return answer(args)
	}
	return &calls
}

func TestNewGHWorkflowFetcher_ObservesOnlyTheNamedWorkflow(t *testing.T) {
	calls := fakeGH(t, func(args []string) ([]byte, error) {
		if i := slices.Index(args, "--workflow"); i >= 0 && args[i+1] == "release.yml" {
			return []byte(`[{"status":"completed","conclusion":"success","url":"https://ci/release","databaseId":9}]`), nil
		}
		return nil, errors.New("unexpected workflow")
	})
	st, err := NewGHWorkflowFetcher(".", "release.yml")(context.Background(), "abc")
	if err != nil || st.Conclusion != ConclusionSuccess || st.RunID != 9 {
		t.Fatalf("fetch = %+v, %v; want release.yml's green run 9", st, err)
	}
	if got := strings.Join((*calls)[0], " "); !strings.Contains(got, "--commit abc") {
		t.Errorf("run list did not filter on the watched commit: %q", got)
	}
}

func TestResolveCommitAndPRHead(t *testing.T) {
	const full = "0123456789ABCDEF0123456789abcdef01234567"
	cases := []struct {
		name    string
		answer  string
		ghErr   error
		resolve func() (string, error)
		want    string
		wantErr string
	}{
		{"tag resolves through the commits api", `{"sha":"` + full + `"}`, nil,
			func() (string, error) { return ResolveCommit(context.Background(), ".", "v1.2.3") }, strings.ToLower(full), ""},
		{"short sha resolves to the full sha", `{"sha":"` + full + `"}`, nil,
			func() (string, error) { return ResolveCommit(context.Background(), ".", "0123456") }, strings.ToLower(full), ""},
		{"pr resolves to its head", `{"headRefOid":"` + full + `"}`, nil,
			func() (string, error) { return ResolvePRHead(context.Background(), ".", "7") }, strings.ToLower(full), ""},
		{"empty pr head is refused", `{"headRefOid":""}`, nil,
			func() (string, error) { return ResolvePRHead(context.Background(), ".", "7") }, "", "PR 7 resolved to"},
		{"gh failure propagates", "", errors.New("HTTP 502"),
			func() (string, error) { return ResolveCommit(context.Background(), ".", "v1") }, "", "HTTP 502"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeGH(t, func([]string) ([]byte, error) { return []byte(tc.answer), tc.ghErr })
			got, err := tc.resolve()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestWatch_NoEscalationReportsRedWithoutAnInboxItem(t *testing.T) {
	fetch := func(context.Context, string) (RunStatus, error) {
		return RunStatus{Status: StatusCompleted, Conclusion: "failure"}, nil
	}
	opts, inbox, _ := watchOpts(t, fetch)
	opts.NoEscalation = true
	rec, err := Watch(context.Background(), opts)
	if err != nil || rec.Conclusion != "failure" {
		t.Fatalf("Watch = %+v, %v; want the red conclusion", rec, err)
	}
	if names := inboxFiles(t, inbox); len(names) != 0 {
		t.Errorf("NoEscalation filed %v", names)
	}
	opts.InboxDir = ""
	if _, err := Watch(context.Background(), opts); err != nil {
		t.Errorf("NoEscalation must not require InboxDir: %v", err)
	}
}
