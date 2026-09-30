package subagentrun

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

var workerNameRE = regexp.MustCompile(`^([a-z][a-z-]+)-worker-([a-z][a-z0-9-]+)$`)

func parseAgentName(agent string) (role, worker string) {
	if m := workerNameRE.FindStringSubmatch(agent); len(m) == 3 {
		return m[1], m[2]
	}
	return agent, ""
}

func enforceBridgeOnly(legacyRequested bool) error {
	if legacyRequested {
		return ErrInProcessDispatchBanned
	}
	return nil
}

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
	if err := enforceBridgeOnly(req.LegacyAgentDispatch); err != nil {
		return d.reject(id, req.Cycle, "legacy_dispatch", err, nil)
	}
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
