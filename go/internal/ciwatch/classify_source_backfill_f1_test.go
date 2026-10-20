package ciwatch

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

type ghRouteF1 struct {
	prefix string
	out    string
	err    error
}

func routedGHF1(t *testing.T, routes ...ghRouteF1) *[]string {
	t.Helper()
	var seen []string
	fakeGH(t, func(args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		seen = append(seen, joined)
		for _, r := range routes {
			if strings.HasPrefix(joined, r.prefix) {
				return []byte(r.out), r.err
			}
		}
		return nil, fmt.Errorf("unrouted gh call: %s", joined)
	})
	return &seen
}

func runJSONF1(id int64, head string, attempt int, status, conclusion string) string {
	return fmt.Sprintf(`{"attempt":%d,"conclusion":%q,"createdAt":"2026-10-01T12:00:00Z","databaseId":%d,"headSha":%q,"status":%q,"url":"u%d"}`, attempt, conclusion, id, head, status, id)
}

func listJSONF1(docs ...string) string { return "[" + strings.Join(docs, ",") + "]" }

func compareJSONF1(status, base string, files ...string) string {
	parts := make([]string, len(files))
	for i, f := range files {
		parts[i] = fmt.Sprintf(`{"filename":%q}`, f)
	}
	return fmt.Sprintf(`{"status":%q,"merge_base_commit":{"sha":%q},"files":[%s]}`, status, base, strings.Join(parts, ","))
}

const (
	compareMainF1 = "api repos/{owner}/{repo}/compare/main...h"
	commitHF1     = "api repos/{owner}/{repo}/commits/h"
)

func TestGHClassifySourceBackfillF1_GhErrorsCarryTheStderrOfAFailedProcess(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantTail string
	}{
		{"exit error with stderr", &exec.ExitError{Stderr: []byte("  HTTP 404: Not Found \n")}, ": HTTP 404: Not Found"},
		{"exit error without stderr", &exec.ExitError{}, ""},
		{"plain error", errors.New("spawn gh: not found"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			routedGHF1(t, ghRouteF1{prefix: "api x", err: tc.err})
			_, err := ghClassifySource{root: "."}.gh(context.Background(), "api", "x")
			if err == nil || !strings.HasPrefix(err.Error(), "gh api x: ") || !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want a gh-prefixed error wrapping %v", err, tc.err)
			}
			if tc.wantTail != "" && !strings.HasSuffix(err.Error(), tc.wantTail) {
				t.Fatalf("err = %q, want the trimmed stderr %q at the end", err, tc.wantTail)
			}
		})
	}
}

func TestGHClassifySourceBackfillF1_GhJSONRejectsAReplyThatIsNotJSONAndPropagatesGhFailure(t *testing.T) {
	routedGHF1(t, ghRouteF1{prefix: "api bad", out: "<html>rate limited</html>"}, ghRouteF1{prefix: "api down", err: errors.New("HTTP 502")})
	var doc ghCompareDoc
	src := ghClassifySource{root: "."}
	if err := src.ghJSON(context.Background(), &doc, "api", "bad"); err == nil || !strings.Contains(err.Error(), "gh api bad: decode:") {
		t.Fatalf("a non-JSON reply must be a decode error naming the call, got %v", err)
	}
	if err := src.ghJSON(context.Background(), &doc, "api", "down"); err == nil || !strings.Contains(err.Error(), "HTTP 502") || strings.Contains(err.Error(), "decode") {
		t.Fatalf("a gh failure is not a decode failure, got %v", err)
	}
}

func TestGHClassifySourceBackfillF1_ResolveAPullRequestUsesItsBaseBranchAndLatestRun(t *testing.T) {
	seen := routedGHF1(t,
		ghRouteF1{prefix: "pr view 12 --json headRefOid,baseRefName", out: `{"headRefOid":"h","baseRefName":"release"}`},
		ghRouteF1{prefix: "api repos/{owner}/{repo}/compare/release...h", out: compareJSONF1("ahead", "m", "go/a/a.go", "go/b/b.go")},
		ghRouteF1{prefix: "run list --workflow", out: listJSONF1(runJSONF1(60, "h", 1, "in_progress", ""))},
	)
	ch, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetPR, "12"})
	if err != nil {
		t.Fatal(err)
	}
	if ch.HeadSHA != "h" || ch.BaseSHA != "m" || !slices.Equal(ch.Files, []string{"go/a/a.go", "go/b/b.go"}) || ch.Truncated || ch.Run.RunID != 60 || ch.Target.Kind != TargetPR {
		t.Fatalf("change = %+v", ch)
	}
	if len(*seen) != 3 {
		t.Fatalf("a PR needs exactly view, compare and run lookup, got %v", *seen)
	}
}

func TestGHClassifySourceBackfillF1_AFullFileListMarksThePullRequestDiffTruncated(t *testing.T) {
	files := make([]string, compareFileCap)
	for i := range files {
		files[i] = fmt.Sprintf("f%03d.go", i)
	}
	routedGHF1(t,
		ghRouteF1{prefix: "pr view 3", out: `{"headRefOid":"h","baseRefName":"main"}`},
		ghRouteF1{prefix: "api repos/{owner}/{repo}/compare/main...h", out: compareJSONF1("ahead", "m", files...)},
		ghRouteF1{prefix: "run list", out: "[]"},
	)
	ch, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetPR, "3"})
	if err != nil || !ch.Truncated || len(ch.Files) != compareFileCap {
		t.Fatalf("change = %+v, err = %v; %d files is the API cap, so the list cannot be trusted", ch, err, compareFileCap)
	}
	if ch.Run.Status != "queued" || ch.Run.RunID != 0 {
		t.Fatalf("a PR with no run yet is queued, got %+v", ch.Run)
	}
}

func TestGHClassifySourceBackfillF1_EachPullRequestStepFailureIsReturnedWithThePartialChange(t *testing.T) {
	view := ghRouteF1{prefix: "pr view 5", out: `{"headRefOid":"h","baseRefName":"main"}`}
	cmp := ghRouteF1{prefix: "api repos/{owner}/{repo}/compare/main...h", out: compareJSONF1("ahead", "m", "a.go")}
	cases := []struct {
		name   string
		routes []ghRouteF1
		want   string
		head   string
	}{
		{"pr view fails", []ghRouteF1{{prefix: "pr view 5", err: errors.New("no such pr")}}, "no such pr", ""},
		{"compare fails", []ghRouteF1{view, {prefix: "api repos", err: errors.New("compare 502")}}, "compare 502", "h"},
		{"run lookup fails", []ghRouteF1{view, cmp, {prefix: "run list", err: errors.New("runs 502")}}, "runs 502", "h"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			routedGHF1(t, tc.routes...)
			ch, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetPR, "5"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if ch.HeadSHA != tc.head || ch.Target.Value != "5" {
				t.Fatalf("partial change = %+v, want head %q", ch, tc.head)
			}
		})
	}
}

func TestGHClassifySourceBackfillF1_ResolveASHAFindsItsCommitThenItsRun(t *testing.T) {
	routedGHF1(t,
		ghRouteF1{prefix: "api repos/{owner}/{repo}/commits/abc1234", out: `{"sha":"h","parents":[]}`},
		ghRouteF1{prefix: "run list --workflow", out: listJSONF1(runJSONF1(70, "h", 1, "completed", "success"))},
		ghRouteF1{prefix: compareMainF1, out: compareJSONF1("ahead", "m", "go/x.go")},
	)
	ch, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetSHA, "abc1234"})
	if err != nil || ch.HeadSHA != "h" || ch.BaseSHA != "m" || ch.Run.RunID != 70 || !slices.Equal(ch.Files, []string{"go/x.go"}) {
		t.Fatalf("change = %+v, err = %v", ch, err)
	}
}

func TestGHClassifySourceBackfillF1_ResolveFailuresAndUnknownKinds(t *testing.T) {
	t.Run("unknown commit", func(t *testing.T) {
		routedGHF1(t, ghRouteF1{prefix: "api repos/{owner}/{repo}/commits/dead", err: errors.New("404")})
		ch, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetSHA, "dead"})
		if err == nil || ch.HeadSHA != "" {
			t.Fatalf("change = %+v, err = %v; want the lookup failure and no head", ch, err)
		}
	})
	t.Run("run lookup of a sha fails", func(t *testing.T) {
		routedGHF1(t,
			ghRouteF1{prefix: "api repos/{owner}/{repo}/commits/abc1234", out: `{"sha":"h"}`},
			ghRouteF1{prefix: "run list", err: errors.New("runs 502")},
		)
		if _, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetSHA, "abc1234"}); err == nil || !strings.Contains(err.Error(), "runs 502") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a target kind the source cannot resolve", func(t *testing.T) {
		routedGHF1(t)
		_, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{Kind: "branch", Value: "main"})
		if !errors.Is(err, ErrBadTarget) || !strings.Contains(err.Error(), `kind "branch"`) {
			t.Fatalf("err = %v, want ErrBadTarget naming the kind", err)
		}
	})
	t.Run("a run whose view carries no head sha stops before the base lookup", func(t *testing.T) {
		seen := routedGHF1(t, ghRouteF1{prefix: "run view 9 --json", out: runJSONF1(9, "", 1, "queued", "")})
		ch, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetRun, "9"})
		if err != nil || ch.HeadSHA != "" || ch.BaseSHA != "" || len(*seen) != 1 {
			t.Fatalf("change = %+v, err = %v, calls = %v", ch, err, *seen)
		}
	})
	t.Run("run view fails", func(t *testing.T) {
		routedGHF1(t, ghRouteF1{prefix: "run view 9", err: errors.New("gone")})
		if _, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetRun, "9"}); err == nil || !strings.Contains(err.Error(), "gone") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the main comparison fails", func(t *testing.T) {
		routedGHF1(t,
			ghRouteF1{prefix: "run view 9 --json", out: runJSONF1(9, "h", 1, "queued", "")},
			ghRouteF1{prefix: "api repos", err: errors.New("compare 502")},
		)
		if _, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetRun, "9"}); err == nil || !strings.Contains(err.Error(), "compare 502") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestGHClassifySourceBackfillF1_MainBaseHandlesCommitsAlreadyOnMain(t *testing.T) {
	run := ghRouteF1{prefix: "run view 9 --json", out: runJSONF1(9, "h", 1, "queued", "")}
	t.Run("a commit behind main falls back to its first parent", func(t *testing.T) {
		routedGHF1(t, run,
			ghRouteF1{prefix: compareMainF1, out: compareJSONF1("behind", "h")},
			ghRouteF1{prefix: commitHF1, out: `{"sha":"h","parents":[{"sha":"p1"},{"sha":"p2"}]}`},
			ghRouteF1{prefix: "api repos/{owner}/{repo}/compare/p1...h", out: compareJSONF1("ahead", "p1", "go/y.go")},
		)
		ch, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetRun, "9"})
		if err != nil || ch.BaseSHA != "p1" || !slices.Equal(ch.Files, []string{"go/y.go"}) {
			t.Fatalf("change = %+v, err = %v; want the FIRST parent as the base", ch, err)
		}
	})
	t.Run("a root commit has no base and no files", func(t *testing.T) {
		routedGHF1(t, run,
			ghRouteF1{prefix: compareMainF1, out: compareJSONF1("identical", "h")},
			ghRouteF1{prefix: commitHF1, out: `{"sha":"h","parents":[]}`},
		)
		ch, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetRun, "9"})
		if err != nil || ch.BaseSHA != "" || len(ch.Files) != 0 || ch.Truncated {
			t.Fatalf("change = %+v, err = %v", ch, err)
		}
	})
	t.Run("the parent lookup fails", func(t *testing.T) {
		routedGHF1(t, run,
			ghRouteF1{prefix: compareMainF1, out: compareJSONF1("identical", "h")},
			ghRouteF1{prefix: commitHF1, err: errors.New("commit 502")},
		)
		if _, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetRun, "9"}); err == nil || !strings.Contains(err.Error(), "commit 502") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("the parent comparison fails", func(t *testing.T) {
		routedGHF1(t, run,
			ghRouteF1{prefix: compareMainF1, out: compareJSONF1("behind", "h")},
			ghRouteF1{prefix: commitHF1, out: `{"sha":"h","parents":[{"sha":"p1"}]}`},
			ghRouteF1{prefix: "api repos/{owner}/{repo}/compare/p1...h", err: errors.New("compare p1 502")},
		)
		ch, err := NewGHClassifySource(".", "").Resolve(context.Background(), Target{TargetRun, "9"})
		if err == nil || !strings.Contains(err.Error(), "compare p1 502") || ch.BaseSHA != "" {
			t.Fatalf("change = %+v, err = %v; a failed second comparison must not leak a half-known base", ch, err)
		}
	})
}

func TestGHClassifySourceBackfillF1_LatestRunOnAndListRunsFailuresStayQueuedOrPropagate(t *testing.T) {
	t.Run("no run yet is queued", func(t *testing.T) {
		routedGHF1(t, ghRouteF1{prefix: "run list", out: "[]"})
		st, err := NewGHClassifySource(".", "").LatestRunOn(context.Background(), "h")
		if err != nil || st.Status != "queued" || st.RunID != 0 {
			t.Fatalf("status = %+v, err = %v", st, err)
		}
	})
	t.Run("a failed listing is an error and still reads as queued", func(t *testing.T) {
		routedGHF1(t, ghRouteF1{prefix: "run list", err: errors.New("runs 502")})
		st, err := NewGHClassifySource(".", "").LatestRunOn(context.Background(), "h")
		if err == nil || st.Status != "queued" {
			t.Fatalf("status = %+v, err = %v", st, err)
		}
	})
	t.Run("a red listed run pulls its failing log, and a log failure aborts the listing", func(t *testing.T) {
		routedGHF1(t,
			ghRouteF1{prefix: "run list", out: listJSONF1(runJSONF1(80, "h", 2, "completed", "failure"))},
			ghRouteF1{prefix: "run view 80 --log-failed --attempt 2", err: errors.New("log 502")},
		)
		if _, err := NewGHClassifySource(".", "").MainRunsBefore(context.Background(), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 3); err == nil || !strings.Contains(err.Error(), "log 502") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a run with no attempt number asks for the log without an attempt flag", func(t *testing.T) {
		seen := routedGHF1(t,
			ghRouteF1{prefix: "run list", out: listJSONF1(runJSONF1(81, "h", 0, "completed", "failure"))},
			ghRouteF1{prefix: "run view 81 --log-failed", out: ghLog("job", "--- FAIL: TestA (0s)", "FAIL\tm/a\t0.1s")},
			ghRouteF1{prefix: "api repos/{owner}/{repo}/actions/runs/81/artifacts", out: `{"artifacts":[]}`},
		)
		st, err := NewGHClassifySource(".", "").LatestRunOn(context.Background(), "h")
		if err != nil || len(st.FailingTests) != 1 || st.FailingTests[0].Test != "TestA" {
			t.Fatalf("status = %+v, err = %v", st, err)
		}
		if last := (*seen)[len(*seen)-1]; strings.Contains(last, "--attempt") {
			t.Fatalf("attempt 0 must not pass --attempt: %q", last)
		}
	})
}

func TestGHClassifySourceBackfillF1_RerunFailed(t *testing.T) {
	t.Run("the rerun request itself fails", func(t *testing.T) {
		routedGHF1(t, ghRouteF1{prefix: "run rerun 5 --failed", err: errors.New("not permitted")})
		if _, err := NewGHClassifySource(".", "").RerunFailed(context.Background(), 5, 1, time.Minute, time.Millisecond); err == nil || !strings.Contains(err.Error(), "not permitted") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a status poll fails", func(t *testing.T) {
		routedGHF1(t, ghRouteF1{prefix: "run rerun 5", out: ""}, ghRouteF1{prefix: "run view 5", err: errors.New("view 502")})
		if _, err := NewGHClassifySource(".", "").RerunFailed(context.Background(), 5, 1, time.Minute, time.Millisecond); err == nil || !strings.Contains(err.Error(), "view 502") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a deadline that cannot fit another poll returns the last observation", func(t *testing.T) {
		routedGHF1(t, ghRouteF1{prefix: "run rerun 5", out: ""}, ghRouteF1{prefix: "run view 5 --json", out: runJSONF1(5, "h", 1, "in_progress", "")})
		guard, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		st, err := NewGHClassifySource(".", "").RerunFailed(guard, 5, 1, 0, 30*time.Minute)
		if err != nil || st.Attempt != 1 || st.Status != "in_progress" {
			t.Fatalf("status = %+v, err = %v; want the stale observation, not an error", st, err)
		}
	})
	t.Run("a cancelled context stops the wait", func(t *testing.T) {
		routedGHF1(t, ghRouteF1{prefix: "run rerun 5", out: ""}, ghRouteF1{prefix: "run view 5 --json", out: runJSONF1(5, "h", 1, "in_progress", "")})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		st, err := NewGHClassifySource(".", "").RerunFailed(ctx, 5, 1, time.Hour, time.Millisecond)
		if !errors.Is(err, context.Canceled) || st.RunID != 0 {
			t.Fatalf("status = %+v, err = %v; want context.Canceled and no run", st, err)
		}
	})
	t.Run("it keeps polling until a newer completed attempt appears", func(t *testing.T) {
		views := 0
		fakeGH(t, func(args []string) ([]byte, error) {
			joined := strings.Join(args, " ")
			switch {
			case strings.HasPrefix(joined, "run rerun 5"):
				return nil, nil
			case strings.HasPrefix(joined, "run view 5 --json"):
				views++
				if views < 3 {
					return []byte(runJSONF1(5, "h", 1, "in_progress", "")), nil
				}
				return []byte(runJSONF1(5, "h", 2, "completed", "success")), nil
			}
			return nil, fmt.Errorf("unrouted: %s", joined)
		})
		st, err := NewGHClassifySource(".", "").RerunFailed(context.Background(), 5, 1, time.Minute, time.Millisecond)
		if err != nil || st.Attempt != 2 || st.Conclusion != ConclusionSuccess || views != 3 {
			t.Fatalf("status = %+v, err = %v, views = %d", st, err, views)
		}
	})
}
