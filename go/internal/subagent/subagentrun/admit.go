package subagentrun

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// workerNameRE matches fan-out worker names: <role>-worker-<subtask>.
// Subtask names may include digits and hyphens after the first letter.
var workerNameRE = regexp.MustCompile(`^([a-z][a-z-]+)-worker-([a-z][a-z0-9-]+)$`)

func parseAgentName(agent string) (role, worker string) {
	if m := workerNameRE.FindStringSubmatch(agent); len(m) == 3 {
		return m[1], m[2]
	}
	return agent, ""
}

// enforceBridgeOnly is the single source of truth for the bridge-only dispatch
// invariant. It rejects any request for the in-process escape hatch. Every
// dispatch path funnels through Dispatch, so enforcing here covers single +
// fan-out + recursive invocations.
func enforceBridgeOnly(legacyRequested bool) error {
	if legacyRequested {
		return ErrInProcessDispatchBanned
	}
	return nil
}

// admit is step 1: the six admission checks in their fixed order — prompt
// reader, role, cycle, workspace, the retired escape hatch, the recursion
// depth — and, as the gate's last act, the run id, so the identity it returns
// is COMPLETE and every later step takes it by value. A rejection is ONE
// BRIDGE_SUBAGENT_REQUEST_REJECTED naming its class; no port is consulted
// after it and the run id is never resolved (the request never became a run).
// Every error text is the path's own.
func (d *Dispatcher) admit(req Request) (identity, error) {
	role, worker := parseAgentName(req.Agent)
	id := identity{agent: req.Agent, role: role, worker: worker}
	if req.Prompt == nil {
		return d.reject(id, req.Cycle, "prompt_reader", errors.New("subagent/run: PromptReader required (PROMPT_FILE_OVERRIDE or stdin)"), nil)
	}
	if !d.deps.KnownRole(role) {
		return d.reject(id, req.Cycle, "unknown_agent", fmt.Errorf("subagent/run: unknown agent: %s", req.Agent), nil)
	}
	if req.Cycle < 0 {
		return d.reject(id, req.Cycle, "cycle", fmt.Errorf("subagent/run: cycle must be >= 0, got %d", req.Cycle), nil)
	}
	if info, err := os.Stat(req.WorkspacePath); err != nil || !info.IsDir() {
		return d.reject(id, req.Cycle, "workspace", fmt.Errorf("subagent/run: workspace dir does not exist: %s", req.WorkspacePath),
			map[string]string{"workspace": req.WorkspacePath})
	}
	// Bridge-only invariant: reject the retired in-process escape hatch before
	// any resolution work — the single chokepoint all dispatch funnels through.
	if err := enforceBridgeOnly(req.LegacyAgentDispatch); err != nil {
		return d.reject(id, req.Cycle, "legacy_dispatch", err, nil)
	}
	// Recursion bound: a fan-out worker re-enters here via `subagent run`; the
	// host's cap keeps a fan-out loop from recursing unboundedly.
	if err := d.deps.GuardDepth(req.DispatchDepth); err != nil {
		return d.reject(id, req.Cycle, "depth", err, map[string]string{"depth": strconv.Itoa(req.DispatchDepth)})
	}
	id.runID = d.deps.RunID(req.WorkspacePath)
	return id, nil
}

func (d *Dispatcher) reject(id identity, cycle int, class string, err error, extra map[string]string) (identity, error) {
	fields := map[string]string{"step": "validate", "reason_class": class}
	for k, v := range extra {
		fields[k] = v
	}
	d.warn(id, cycle, CodeRequestRejected, err.Error(), fields)
	return identity{}, err
}
