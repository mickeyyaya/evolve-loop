package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var errItemUnchanged = errors.New("lifecycle: item unchanged")

func (m *Mover) parkPrebuild(d drained, cycle int, q *Policy) (bool, string) {
	cycles, err := recordPrebuild(d.src, cycle, q)
	if err != nil {
		m.warn(fault{code: CodeItemRewriteFailed, origin: "Mover.Release", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("release-cycle: prebuild failure record failed for %s (%v) — the breaker is skipped, releasing to inbox root", d.base, err),
			fields: map[string]string{"task_id": d.taskID, "path": d.src, "err": err.Error(), "step": "prebuild_record"}})
		return false, ""
	}
	if q.PrebuildCeiling <= 0 || len(cycles) < q.PrebuildCeiling {
		return false, ""
	}
	list, ceiling := joinCycles(cycles), strconv.Itoa(q.PrebuildCeiling)
	fields := map[string]string{"task_id": d.taskID, "cycles": list, "ceiling": ceiling, "step": "prebuild_breaker"}
	parked, at := m.quarantine(d, cycle, fmt.Sprintf("failed before build in cycles %s >= ceiling %s", list, ceiling), fields)
	if parked {
		m.warn(fault{code: CodeItemPrebuildBreaker, origin: "Mover.Release", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("prebuild breaker: '%s' failed before build in cycles %s (ceiling %s) — parked in quarantine/, no lane claims it again", d.taskID, list, ceiling),
			fields: map[string]string{"task_id": d.taskID, "cycles": list, "ceiling": ceiling, "path": at, "step": "prebuild_breaker"}})
	}
	return parked, at
}

func recordPrebuild(path string, cycle int, q *Policy) ([]int, error) {
	var cycles []int
	err := rewriteItemJSON(path, admitEvery, func(item map[string]json.RawMessage) error {
		raw, had := item[inboxbatch.PrebuildFailureCyclesField]
		if !q.Prebuild {
			delete(item, inboxbatch.PrebuildFailureCyclesField)
			return unchangedUnless(had && q.PastBuild)
		}
		_ = json.Unmarshal(raw, &cycles)
		if slices.Contains(cycles, cycle) {
			return errItemUnchanged
		}
		cycles = append(cycles, cycle)
		item[inboxbatch.PrebuildFailureCyclesField], _ = json.Marshal(cycles)
		return nil
	})
	if errors.Is(err, errItemUnchanged) {
		err = nil
	}
	return cycles, err
}

func unchangedUnless(changed bool) error {
	if changed {
		return nil
	}
	return errItemUnchanged
}

func joinCycles(cycles []int) string {
	parts := make([]string, len(cycles))
	for i, c := range cycles {
		parts[i] = strconv.Itoa(c)
	}
	return strings.Join(parts, ",")
}
