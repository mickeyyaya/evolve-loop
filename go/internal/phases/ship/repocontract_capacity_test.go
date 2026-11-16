package ship

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func swapImporterBackstopTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	prior := importerBackstopTimeout
	importerBackstopTimeout = d
	t.Cleanup(func() { importerBackstopTimeout = prior })
}

func TestRepoContractGate_ADeadlineKillOnTheImporterBackstopIsACapacityFailure(t *testing.T) {
	repo, goDir := importerFixture(t)
	mustWrite(t, filepath.Join(goDir, "internal", "base", "base.go"), "package base\n\nconst Stop = \"new-stop\"\n")
	swapImporterBackstopTimeout(t, time.Nanosecond)

	err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), io.Discard)

	se, ok := shiperr.AsShipError(err)
	if !ok || se.Code != shiperr.CodeRepoContractCapacity || se.Class != shiperr.ShipClassTransient {
		t.Fatalf("error = %v, want REPO_CONTRACT_CAPACITY of class transient", err)
	}
	if !strings.Contains(err.Error(), "deadline") {
		t.Errorf("the error does not name the deadline: %v", err)
	}
}

func TestCapacityOnDeadline_OnlyAnInfraKillAtTheDeadlineBecomesCapacity(t *testing.T) {
	t.Parallel()

	expired, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-expired.Done()
	infra := shiperr.NewShipError(shiperr.CodeRepoContractInfra, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip, "died")
	gate := shiperr.NewShipError(shiperr.CodeRepoContractGate, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip, "red")
	cancelled, stop := context.WithCancel(context.Background())
	stop()

	cases := []struct {
		name   string
		parent context.Context
		bctx   context.Context
		err    error
		want   shiperr.ShipErrorCode
	}{
		{"infra at the deadline", context.Background(), expired, infra, shiperr.CodeRepoContractCapacity},
		{"real red at the deadline", context.Background(), expired, gate, shiperr.CodeRepoContractGate},
		{"infra with time left", context.Background(), context.Background(), infra, shiperr.CodeRepoContractInfra},
		{"infra after the caller gave up", cancelled, expired, infra, shiperr.CodeRepoContractInfra},
		{"a plain error", context.Background(), expired, errors.New("boom"), ""},
		{"no error", context.Background(), expired, nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := capacityOnDeadline(c.parent, c.bctx, c.err, 160)
			if c.err == nil {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			se, ok := shiperr.AsShipError(got)
			if c.want == "" {
				if ok || got.Error() != c.err.Error() {
					t.Fatalf("got %v, want the plain error unchanged", got)
				}
				return
			}
			if !ok || se.Code != c.want {
				t.Fatalf("got %v, want code %s", got, c.want)
			}
		})
	}
}
