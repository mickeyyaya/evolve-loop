package integration

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const RolloutNeed = 2

type RolloutError struct {
	Clean    []int
	Need     int
	Excluded map[int]string
	Fault    error
}

func (e *RolloutError) Error() string {
	msg := "integration: " + ReadyLine(e.Clean, e.Need) + ", so the driver does not run at enforce"
	if e.Fault != nil {
		msg += "; the evidence chain is unreadable: " + e.Fault.Error()
	}
	for _, pool := range slices.Sorted(maps.Keys(e.Excluded)) {
		msg += fmt.Sprintf("; pool %d is excluded: %s", pool, e.Excluded[pool])
	}
	return msg
}

func (e *RolloutError) Unwrap() error { return e.Fault }

func ReadyLine(clean []int, need int) string {
	return fmt.Sprintf("enforce ready: %d/%d clean shadow waves (pools %s)", len(clean), need, joinPools(clean))
}

type PoolHeldError struct {
	Pool int
	By   int
}

func (e *PoolHeldError) Error() string {
	return fmt.Sprintf("integration: pool %d waits, because the first enforce pool %d has not ended", e.Pool, e.By)
}

func joinPools(pools []int) string {
	if len(pools) == 0 {
		return "none"
	}
	parts := make([]string, len(pools))
	for i, p := range pools {
		parts[i] = fmt.Sprint(p)
	}
	return strings.Join(parts, ", ")
}

type RolloutCount struct {
	Root   string
	Chain  ChainPort
	Gather func(pool int) PoolEvidence
}

type Readiness struct {
	Clean    []int
	Excluded map[int]string
	Fault    error
}

func (c RolloutCount) Count(ctx context.Context, pools []int) Readiness {
	ready := Readiness{Excluded: map[int]string{}}
	sealed := map[int]string{}
	if c.Chain != nil {
		var err error
		if sealed, err = c.Chain.SealedAll(ctx); err != nil {
			ready.Fault = err
			return ready
		}
	}
	for _, pool := range slices.Sorted(slices.Values(pools)) {
		ev := c.Gather(pool)
		if ev.Record.Stage == policy.FleetIntegrationEnforce {
			continue
		}
		if verdict, failed := PoolVerdict(ev); verdict != PoolClean {
			ready.Excluded[pool] = strings.Join(append([]string{verdict}, failed...), ": ")
			continue
		}
		chained, err := Chained(sealed, c.Root, pool)
		switch {
		case err != nil:
			ready.Excluded[pool] = "the record digest is unreadable: " + err.Error()
		case !chained:
			ready.Excluded[pool] = "the wave record is not chained"
		default:
			ready.Clean = append(ready.Clean, pool)
		}
	}
	return ready
}

func firstEnforcePool(store WaveStore) (pool int, ended bool, found bool, err error) {
	for _, p := range slices.Sorted(slices.Values(store.Pools())) {
		rec, ok, err := store.Read(p)
		if err != nil {
			return 0, false, false, fmt.Errorf("integration: read the record of pool %d: %w", p, err)
		}
		if !ok || rec.Stage != policy.FleetIntegrationEnforce {
			continue
		}
		return p, rec.Ended, true, nil
	}
	return 0, false, false, nil
}

func HeldBy(store WaveStore, pool int) (by int, held bool, err error) {
	first, ended, found, err := firstEnforcePool(store)
	if err != nil {
		return 0, false, err
	}
	return first, found && first != pool && !ended, nil
}

func (d *Driver) admitStage(stage string) error {
	if stage != policy.FleetIntegrationEnforce {
		return nil
	}
	ready := d.readiness()
	if len(ready.Clean) < RolloutNeed {
		return &RolloutError{Clean: ready.Clean, Need: RolloutNeed, Excluded: ready.Excluded, Fault: ready.Fault}
	}
	by, held, err := HeldBy(d.Store, d.Pool)
	if err != nil {
		return err
	}
	if held {
		return &PoolHeldError{Pool: d.Pool, By: by}
	}
	return nil
}

func (d *Driver) readiness() Readiness {
	if d.ready == nil {
		var ready Readiness
		if d.Clean != nil {
			ready = d.Clean()
		}
		d.ready = &ready
	}
	return *d.ready
}
