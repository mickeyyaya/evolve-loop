package acsassert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const largestLaneSum = 187 * time.Second

func checkedInPolicyPath() string {
	return filepath.Join("..", "..", "..", ".evolve", "policy.json")
}

func capFitFailure(t *testing.T, policyPath string, subprocessBound time.Duration) string {
	t.Helper()
	loaded, err := policy.Load(policyPath)
	if err != nil {
		t.Fatalf("load %s: %v", policyPath, err)
	}
	goTimeoutS := loaded.ACSTimeoutConfig().GoTimeoutS
	if goTimeoutS <= 0 {
		return fmt.Sprintf("%s sets no acs.go_timeout_s, so the cap has no budget to fit inside", policyPath)
	}
	budget := time.Duration(goTimeoutS) * time.Second
	if need := subprocessBound + largestLaneSum; need > budget {
		return fmt.Sprintf("subprocess cap %s plus the largest lane sum %s is %s, above acs.go_timeout_s %s", subprocessBound, largestLaneSum, need, budget)
	}
	return ""
}

func TestSubprocessCapFitsGoTimeout(t *testing.T) {
	if failure := capFitFailure(t, checkedInPolicyPath(), subprocessCap); failure != "" {
		t.Error(failure)
	}
}

func TestSubprocessCapFitsGoTimeout_RejectsATightPolicy(t *testing.T) {
	for _, tc := range []struct {
		name       string
		goTimeoutS int
		wantFail   bool
	}{
		{"exactly_fits", int((subprocessCap + largestLaneSum) / time.Second), false},
		{"one_second_short", int((subprocessCap+largestLaneSum)/time.Second) - 1, true},
		{"four_hundred_seconds", 400, true},
		{"unset", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy.json")
			body := fmt.Sprintf(`{"acs":{"go_timeout_s":%d}}`, tc.goTimeoutS)
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if failed := capFitFailure(t, path, subprocessCap) != ""; failed != tc.wantFail {
				t.Errorf("go_timeout_s=%d: failed=%v, want %v", tc.goTimeoutS, failed, tc.wantFail)
			}
		})
	}
}

func boundMessageOfAHungGoTests(t *testing.T, tb interface {
	TB
	recorded() string
}) string {
	t.Helper()
	installFakeGo(t, "exec sleep 30")
	done := make(chan bool, 1)
	go func() {
		done <- GoTests(tb, GoTestSpec{Dir: t.TempDir(), Package: "./x", Pattern: "^TestA$", Names: []string{"TestA"}})
	}()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("GoTests reported success for a hung go test")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("GoTests did not return: no bound ended the hung go test")
	}
	return tb.recorded()
}

type recordedCaller struct{ boundCallerT }

func (r *recordedCaller) recorded() string { return strings.Join(r.messages, "\n") }

type recordedLane struct{ recordingT }

func (r *recordedLane) recorded() string { return strings.Join(r.messages, "\n") }

func TestBoundLineNamesCallerDeadline(t *testing.T) {
	t.Run("caller_deadline_earlier_than_the_lane_end", func(t *testing.T) {
		setLaneEnd(t, time.Now().Add(time.Hour))
		message := boundMessageOfAHungGoTests(t, &recordedCaller{boundCallerT{deadline: time.Now().Add(time.Second)}})
		if !strings.Contains(message, acsverdict.SubprocessBoundMarker) {
			t.Errorf("message %q lacks the bound marker the lane reads", message)
		}
		if !strings.Contains(message, "caller deadline") {
			t.Errorf("message %q does not name the caller deadline that ended the call", message)
		}
	})
	t.Run("lane_end_earlier_than_the_caller_deadline", func(t *testing.T) {
		setLaneEnd(t, time.Now().Add(time.Second))
		message := boundMessageOfAHungGoTests(t, &recordedCaller{boundCallerT{deadline: time.Now().Add(time.Hour)}})
		if !strings.Contains(message, acsverdict.SubprocessBoundMarker) {
			t.Errorf("message %q lacks the bound marker the lane reads", message)
		}
		if strings.Contains(message, "caller deadline") {
			t.Errorf("message %q names the caller deadline although the lane end came first", message)
		}
	})
	t.Run("no_caller_deadline", func(t *testing.T) {
		setLaneEnd(t, time.Now().Add(time.Second))
		message := boundMessageOfAHungGoTests(t, &recordedLane{})
		if strings.Contains(message, "caller deadline") {
			t.Errorf("message %q names a caller deadline although the caller set none", message)
		}
	})
}
