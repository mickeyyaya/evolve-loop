package opscmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/marketplacepoll"
	"github.com/mickeyyaya/evolve-loop/go/internal/releasepipeline"
	"github.com/mickeyyaya/evolve-loop/go/internal/rollback"
)

func TestMarketplacePollIntFlag_AcceptsOnlyAPositiveIntegerAfterTheFlagF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		args      []string
		i         int
		wantValue int
		wantNext  int
		wantOK    bool
		wantErr   string
	}{
		{"positive", []string{"--max-wait-s", "7"}, 0, 7, 1, true, ""},
		{"one is the boundary", []string{"--max-wait-s", "1"}, 0, 1, 1, true, ""},
		{"zero", []string{"--max-wait-s", "0"}, 0, 0, 0, false, "--max-wait-s must be integer > 0"},
		{"negative", []string{"--max-wait-s", "-3"}, 0, 0, 0, false, "must be integer > 0"},
		{"not a number", []string{"--max-wait-s", "soon"}, 0, 0, 0, false, "must be integer > 0"},
		{"empty value", []string{"--max-wait-s", ""}, 0, 0, 0, false, "must be integer > 0"},
		{"no value at the end", []string{"--max-wait-s"}, 0, 0, 0, false, "--max-wait-s missing value"},
		{"no args at all", nil, 0, 0, 0, false, "missing value"},
		{"flag deeper in the list", []string{"x", "--max-wait-s", "9"}, 1, 9, 2, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var errb bytes.Buffer

			value, next, ok := marketplacePollIntFlag(tc.args, tc.i, "--max-wait-s", &errb)

			if value != tc.wantValue || next != tc.wantNext || ok != tc.wantOK || (tc.wantErr == "") != (errb.Len() == 0) || !strings.Contains(errb.String(), tc.wantErr) {
				t.Errorf("got (%d, %d, %v) err=%q, want (%d, %d, %v) containing %q", value, next, ok, errb.String(), tc.wantValue, tc.wantNext, tc.wantOK, tc.wantErr)
			}
		})
	}
}

func TestMarketplacePollFlagCase_AppliesEachFlagAndAdvancesTheCursorF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		args        []string
		wantHandled bool
		wantOK      bool
		wantIndex   int
		want        marketplacePollFlags
		wantErr     string
	}{
		{"dry run", []string{"--dry-run"}, true, true, 0, marketplacePollFlags{dryRun: true}, ""},
		{"max wait", []string{"--max-wait-s", "12"}, true, true, 1, marketplacePollFlags{maxWaitS: 12}, ""},
		{"max wait missing value", []string{"--max-wait-s"}, true, false, 0, marketplacePollFlags{}, "--max-wait-s missing value"},
		{"max wait not positive", []string{"--max-wait-s", "0"}, true, false, 0, marketplacePollFlags{}, "must be integer > 0"},
		{"poll interval", []string{"--poll-interval-s", "3"}, true, true, 1, marketplacePollFlags{pollIntervalS: 3}, ""},
		{"poll interval missing value", []string{"--poll-interval-s"}, true, false, 0, marketplacePollFlags{}, "--poll-interval-s missing value"},
		{"poll interval invalid", []string{"--poll-interval-s", "x"}, true, false, 0, marketplacePollFlags{}, "--poll-interval-s must be integer > 0"},
		{"marketplace dir", []string{"--marketplace-dir", "/m"}, true, true, 1, marketplacePollFlags{marketplaceDir: "/m"}, ""},
		{"marketplace dir missing value", []string{"--marketplace-dir"}, true, false, 1, marketplacePollFlags{}, "--marketplace-dir missing value"},
		{"unknown flag", []string{"--bogus"}, true, false, 0, marketplacePollFlags{}, "unknown flag: --bogus"},
		{"a single dash is a positional", []string{"-"}, false, true, 0, marketplacePollFlags{}, ""},
		{"a version is a positional", []string{"1.2.3"}, false, true, 0, marketplacePollFlags{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var f marketplacePollFlags
			var errb bytes.Buffer
			i := 0

			handled, ok := marketplacePollFlagCase(&f, tc.args[0], tc.args, &i, &errb)

			if handled != tc.wantHandled || ok != tc.wantOK || i != tc.wantIndex || f != tc.want || (tc.wantErr == "") != (errb.Len() == 0) || !strings.Contains(errb.String(), tc.wantErr) {
				t.Errorf("handled=%v ok=%v i=%d f=%+v err=%q, want %v %v %d %+v containing %q", handled, ok, i, f, errb.String(), tc.wantHandled, tc.wantOK, tc.wantIndex, tc.want, tc.wantErr)
			}
		})
	}
}

func TestMarketplacePollExitCode_MapsEachFailureClassToItsExitCodeF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		err        error
		want       int
		wantStderr string
	}{
		{"success", nil, 0, ""},
		{"timeout is quiet", marketplacepoll.ErrTimeout, 1, ""},
		{"wrapped timeout", fmt.Errorf("after 300s: %w", marketplacepoll.ErrTimeout), 1, ""},
		{"runtime error", marketplacepoll.ErrRuntime, 2, "[marketplace-poll] FAIL: marketplacepoll: runtime error"},
		{"wrapped runtime error", fmt.Errorf("bad plugin.json: %w", marketplacepoll.ErrRuntime), 2, "FAIL: bad plugin.json"},
		{"any other error", errors.New("disk on fire"), 2, "[marketplace-poll] FAIL: disk on fire"},
		{"timeout wins in a join", errors.Join(marketplacepoll.ErrTimeout, marketplacepoll.ErrRuntime), 1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var errb bytes.Buffer

			got := marketplacePollExitCode(tc.err, &errb)

			if got != tc.want || (tc.wantStderr == "") != (errb.Len() == 0) || !strings.Contains(errb.String(), tc.wantStderr) {
				t.Errorf("exit %d stderr %q, want %d containing %q", got, errb.String(), tc.want, tc.wantStderr)
			}
		})
	}
}

func TestReleasePipelineFlagCase_AppliesEachFlagAndAdvancesTheCursorF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		args        []string
		wantHandled bool
		wantOK      bool
		wantIndex   int
		want        releasePipelineFlags
		wantErr     string
	}{
		{"dry run", []string{"--dry-run"}, true, true, 0, releasePipelineFlags{dryRun: true}, ""},
		{"no rollback", []string{"--no-rollback"}, true, true, 0, releasePipelineFlags{noRollback: true}, ""},
		{"skip tests", []string{"--skip-tests"}, true, true, 0, releasePipelineFlags{skipTests: true}, ""},
		{"strict pass", []string{"--strict-pass"}, true, true, 0, releasePipelineFlags{strictPass: true}, ""},
		{"require preflight", []string{"--require-preflight"}, true, true, 0, releasePipelineFlags{requirePreflight: true}, ""},
		{"max poll wait", []string{"--max-poll-wait-s", "45"}, true, true, 1, releasePipelineFlags{maxPollWaitS: 45}, ""},
		{"max poll wait of one", []string{"--max-poll-wait-s", "1"}, true, true, 1, releasePipelineFlags{maxPollWaitS: 1}, ""},
		{"max poll wait missing value", []string{"--max-poll-wait-s"}, true, false, 1, releasePipelineFlags{}, "--max-poll-wait-s missing value"},
		{"max poll wait zero", []string{"--max-poll-wait-s", "0"}, true, false, 1, releasePipelineFlags{}, "--max-poll-wait-s must be integer > 0"},
		{"max poll wait not a number", []string{"--max-poll-wait-s", "x"}, true, false, 1, releasePipelineFlags{}, "must be integer > 0"},
		{"from tag", []string{"--from-tag", "v1.0.0"}, true, true, 1, releasePipelineFlags{fromTag: "v1.0.0"}, ""},
		{"from tag missing value", []string{"--from-tag"}, true, false, 1, releasePipelineFlags{}, "--from-tag missing value"},
		{"unknown flag", []string{"--bogus"}, true, false, 0, releasePipelineFlags{}, "unknown flag: --bogus"},
		{"a version is a positional", []string{"1.2.3"}, false, true, 0, releasePipelineFlags{}, ""},
		{"a single dash is a positional", []string{"-"}, false, true, 0, releasePipelineFlags{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var f releasePipelineFlags
			var errb bytes.Buffer
			i := 0

			handled, ok := releasePipelineFlagCase(&f, tc.args[0], tc.args, &i, &errb)

			if handled != tc.wantHandled || ok != tc.wantOK || i != tc.wantIndex || f != tc.want || (tc.wantErr == "") != (errb.Len() == 0) || !strings.Contains(errb.String(), tc.wantErr) {
				t.Errorf("handled=%v ok=%v i=%d f=%+v err=%q, want %v %v %d %+v containing %q", handled, ok, i, f, errb.String(), tc.wantHandled, tc.wantOK, tc.wantIndex, tc.want, tc.wantErr)
			}
		})
	}
}

func TestReleasePipelineExitCode_MapsEachStageFailureToItsExitCodeF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, 0},
		{"pre-publish", releasepipeline.ErrPrePublishFailed, 1},
		{"ship", releasepipeline.ErrShipFailed, 2},
		{"post-publish", releasepipeline.ErrPostPublishFailed, 3},
		{"wrapped ship", fmt.Errorf("step 4: %w", releasepipeline.ErrShipFailed), 2},
		{"wrapped post-publish", fmt.Errorf("step 6: %w", releasepipeline.ErrPostPublishFailed), 3},
		{"unclassified", errors.New("disk on fire"), 1},
		{"first class in a join wins", errors.Join(releasepipeline.ErrPrePublishFailed, releasepipeline.ErrShipFailed), 1},
		{"ship before post in a join", errors.Join(releasepipeline.ErrShipFailed, releasepipeline.ErrPostPublishFailed), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var errb bytes.Buffer

			got := releasePipelineExitCode(tc.err, &errb)

			if got != tc.want || (tc.err == nil) != (errb.Len() == 0) || (tc.err != nil && !strings.HasPrefix(errb.String(), "[release-pipeline] FAIL: ")) {
				t.Errorf("exit %d stderr %q, want %d", got, errb.String(), tc.want)
			}
		})
	}
}

func TestRollbackExitCode_MapsEachFailureClassToItsExitCodeF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		err        error
		want       int
		wantStderr string
	}{
		{"success", nil, 0, ""},
		{"journal missing", rollback.ErrJournalNotFound, 2, "[rollback] FAIL: rollback: journal not found"},
		{"journal malformed", rollback.ErrJournalMalformed, 2, "[rollback] FAIL: rollback: journal malformed"},
		{"wrapped missing journal", fmt.Errorf("open: %w", rollback.ErrJournalNotFound), 2, "FAIL: open:"},
		{"partial is quiet", rollback.ErrPartial, 1, ""},
		{"wrapped partial is quiet", fmt.Errorf("step 2: %w", rollback.ErrPartial), 1, ""},
		{"unclassified", errors.New("disk on fire"), 1, "[rollback] FAIL: disk on fire"},
		{"malformed wins over partial in a join", errors.Join(rollback.ErrPartial, rollback.ErrJournalMalformed), 2, "journal malformed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var errb bytes.Buffer

			got := rollbackExitCode(tc.err, &errb)

			if got != tc.want || (tc.wantStderr == "") != (errb.Len() == 0) || !strings.Contains(errb.String(), tc.wantStderr) {
				t.Errorf("exit %d stderr %q, want %d containing %q", got, errb.String(), tc.want, tc.wantStderr)
			}
		})
	}
}

func TestParseReleasePreflightArgs_RecognisesEveryFlagAndRefusesTheRestF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		want     releasePreflightFlags
		wantCode int
		wantDone bool
		wantErr  string
		wantOut  string
	}{
		{"all flags", []string{"1.2.3", "--dry-run", "--skip-tests", "--strict-pass", "--allow-red-ci"}, releasePreflightFlags{target: "1.2.3", dryRun: true, skipTests: true, strictPass: true, allowRedCI: true}, 0, false, "", ""},
		{"strict pass only", []string{"--strict-pass", "1.2.3"}, releasePreflightFlags{target: "1.2.3", strictPass: true}, 0, false, "", ""},
		{"allow red ci only", []string{"1.2.3", "--allow-red-ci"}, releasePreflightFlags{target: "1.2.3", allowRedCI: true}, 0, false, "", ""},
		{"help", []string{"--help"}, releasePreflightFlags{}, 0, true, "", "Usage: evolve release-preflight"},
		{"short help", []string{"1.2.3", "-h"}, releasePreflightFlags{target: "1.2.3"}, 0, true, "", "--allow-red-ci"},
		{"unknown flag", []string{"--bogus"}, releasePreflightFlags{}, 10, true, "unknown flag: --bogus", ""},
		{"extra positional", []string{"1.2.3", "1.2.4"}, releasePreflightFlags{target: "1.2.3"}, 10, true, "extra positional arg: 1.2.4", ""},
		{"no target", nil, releasePreflightFlags{}, 10, true, "usage: release-preflight", ""},
		{"only flags", []string{"--dry-run"}, releasePreflightFlags{dryRun: true}, 10, true, "usage: release-preflight", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, errb bytes.Buffer

			f, code, done := parseReleasePreflightArgs(tc.args, &out, &errb)

			if f != tc.want || code != tc.wantCode || done != tc.wantDone || !strings.Contains(errb.String(), tc.wantErr) || !strings.Contains(out.String(), tc.wantOut) {
				t.Errorf("got %+v code=%d done=%v err=%q out=%q", f, code, done, errb.String(), out.String())
			}
		})
	}
}
