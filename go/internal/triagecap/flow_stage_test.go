package triagecap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func stagedReview(t *testing.T, policy, companion string) (core.ReviewInput, *signalcenter.Center) {
	t.Helper()
	ws := writeTriageWorkspace(t, readFixture(t, "triage-cycle283.md"))
	if policy != "" {
		if err := os.MkdirAll(filepath.Join(ws, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if companion != "" {
		if err := os.WriteFile(filepath.Join(ws, TriageDecisionName()), []byte(companion), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := signalcenter.New()
	in := reviewIn(ws)
	in.Cycle, in.Signals = 9, c
	return in, c
}

func flowViolations(c *signalcenter.Center) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range c.Recent() {
		if e.Code == isolation.ViolationCode(8) {
			out = append(out, e)
		}
	}
	return out
}

func TestCapReviewer_LegacyCountsTheProseFloorsAndEmitsAnI8Violation(t *testing.T) {
	t.Parallel()
	in, c := stagedReview(t, "", "")
	if rr := newTestReviewer(config.StageEnforce, nil, nil).Review(context.Background(), in); rr.Approve {
		t.Fatal("legacy keeps the prose count: the cycle-283 shape is still rejected")
	}
	v := flowViolations(c)
	if len(v) != 1 || v[0].Fields["action"] != "logged" || v[0].Fields["rule"] != phasecontract.FlowRuleProseFallback ||
		v[0].Fields["cell"] != phasecontract.FlowSeatTriage || v[0].Cycle != 9 || v[0].Origin != "CapReviewer.Review" {
		t.Fatalf("violations %+v", v)
	}
}

func TestCapReviewer_EnforceStageNeverCountsProseFloorsAndRefusesAMissingDecision(t *testing.T) {
	t.Parallel()
	in, c := stagedReview(t, `{"isolation":{"i8":{"stage":"enforce"}}}`, "")
	if rr := newTestReviewer(config.StageEnforce, nil, nil).Review(context.Background(), in); rr.Approve || !strings.Contains(rr.Reason, "committed_floors") {
		t.Fatalf("a missing decision file fails the contract at enforce: %+v", rr)
	}
	shadowIn, _ := stagedReview(t, `{"isolation":{"i8":{"stage":"enforce"}}}`, "")
	if rr := newTestReviewer(config.StageShadow, nil, nil).Review(context.Background(), shadowIn); !rr.Approve {
		t.Fatalf("a shadow capacity clamp only logs the refusal: %+v", rr)
	}
	v := flowViolations(c)
	if len(v) != 1 || v[0].Fields["action"] != "refused" || v[0].Fields["stage"] != "enforce" {
		t.Fatalf("violations %+v", v)
	}
}

func TestCapReviewer_EnforceReadsTheDeclaredFloorsOfAValidDecisionFile(t *testing.T) {
	t.Parallel()
	in, c := stagedReview(t, `{"isolation":{"i8":{"stage":"enforce"}}}`, `{"top_n":[],"committed_floors":["a","b","c","d","e","f","g","h"]}`)
	if rr := newTestReviewer(config.StageEnforce, nil, nil).Review(context.Background(), in); rr.Approve {
		t.Fatal("eight declared floors exceed the cap of seven")
	}
	if v := flowViolations(c); len(v) != 0 {
		t.Fatalf("a valid decision file is no violation: %+v", v)
	}
}

func TestCapReviewer_EnforceRefusesADecisionFileOutsideTheSchema(t *testing.T) {
	t.Parallel()
	in, c := stagedReview(t, `{"isolation":{"i8":{"stage":"enforce"}}}`, `{"committed_floors":["a","b","c","d","e","f","g","h"]}`)
	if rr := newTestReviewer(config.StageEnforce, nil, nil).Review(context.Background(), in); rr.Approve {
		t.Fatal("a decision file without top_n fails the contract at enforce")
	}
	if v := flowViolations(c); len(v) != 1 || v[0].Fields["action"] != "refused" {
		t.Fatalf("violations %+v", v)
	}
}

func TestCapReviewer_ADeclaredDecisionIsNoViolationAtLegacy(t *testing.T) {
	t.Parallel()
	in, c := stagedReview(t, "", `{"committed_floors":["a"]}`)
	if rr := newTestReviewer(config.StageEnforce, nil, nil).Review(context.Background(), in); !rr.Approve {
		t.Fatalf("one declared floor fits the cap: %s", rr.Reason)
	}
	if v := flowViolations(c); len(v) != 0 {
		t.Fatalf("violations %+v", v)
	}
}

func TestCapReviewer_EnforceRefusesAValidDecisionFileThatDeclaresNoFloors(t *testing.T) {
	t.Parallel()
	in, c := stagedReview(t, `{"isolation":{"i8":{"stage":"enforce"}}}`, `{"top_n":[]}`)
	if rr := newTestReviewer(config.StageEnforce, nil, nil).Review(context.Background(), in); rr.Approve {
		t.Fatal("no declared floors fails the contract at enforce")
	}
	v := flowViolations(c)
	if len(v) != 1 || v[0].Fields["action"] != "refused" || !strings.Contains(v[0].Reason, "declares no committed_floors") {
		t.Fatalf("violations %+v", v)
	}
}

func TestCapReviewer_EnforceRefusesCommittedFloorsThatAreNotAStringList(t *testing.T) {
	t.Parallel()
	in, c := stagedReview(t, `{"isolation":{"i8":{"stage":"enforce"}}}`, `{"top_n":[],"committed_floors":"a"}`)
	if rr := newTestReviewer(config.StageEnforce, nil, nil).Review(context.Background(), in); rr.Approve {
		t.Fatal("a malformed declaration fails the contract at enforce")
	}
	if v := flowViolations(c); len(v) != 1 || v[0].Fields["action"] != "refused" || !strings.Contains(v[0].Reason, "cannot unmarshal") {
		t.Fatalf("violations %+v", v)
	}
}
