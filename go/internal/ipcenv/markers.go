package ipcenv

import (
	"path"
	"slices"
	"strings"
)

type marker struct {
	name   string
	inCell bool
}

var markers = []marker{
	{"EVOLVE_PROJECT_ROOT", true}, {"EVOLVE_PLUGIN_ROOT", false},
	{DispatchIDKey, false}, {DispatchDepthKey, true}, {MsgTokenKey, false}, {PhaseKey, true},
	{FleetKey, true}, {FleetScopeKey, true}, {FleetWidthKey, true}, {FleetReachHolderKey, true},
	{WorktreeRootKey, true}, {CycleStateFileKey, true}, {TmuxSocketKey, true},
	{"EVOLVE_FANOUT_WORKER_TOKEN", false}, {"EVOLVE_FANOUT_WORKER_NAME", false}, {"EVOLVE_FANOUT_CYCLE", false},
	{"EVOLVE_FANOUT_WORKSPACE", false}, {"EVOLVE_FANOUT_PARENT_AGENT", false}, {"EVOLVE_FANOUT_WORKER_ARTIFACT", false},
	{"EVOLVE_FANOUT_CACHE_PREFIX_FILE", false},
	{"EVOLVE_BRIDGE_PIDFILE", false}, {"EVOLVE_PHASE_RECOVERY_STAGE", false}, {"EVOLVE_SHIP_RELEASE_NOTES", false},
	{"EVOLVE_CLI", false},
	{"BRIDGE_CLI", false}, {"BRIDGE_RUN_ID", false}, {"BRIDGE_COMPLETION", false}, {"BRIDGE_PERMISSION_MODE", false},
	{"BRIDGE_SESSION_NAME", false}, {"BRIDGE_STREAM_OUTPUT", false}, {"BRIDGE_REQUIRE_FULL", false},
	{"BRIDGE_REQUIRE_SANDBOX", false}, {"BRIDGE_HUMAN_INPUT_FLAG", false},
}

var unmovedControls = []string{
	"EVOLVE_SANDBOX", "EVOLVE_USE_PHASE_REGISTRY", "EVOLVE_OPERATOR_CONFIRM", "EVOLVE_SHIP_AUTO_CONFIRM",
	"EVOLVE_PHASE_*_BIN", "BRIDGE_ALLOW_BYPASS", "BRIDGE_DRY_RUN", "BRIDGE_ALLOW_ANTHROPIC_BASE_URL",
	"BRIDGE_ALLOW_OPENAI_API_KEY", "BRIDGE_HUMAN_SIMULATION",
}

type RetiredControl struct{ Name, Use string }

var retiredControls = []RetiredControl{
	{"EVOLVE_DYNAMIC_ROUTING", "policy.json router.dynamic_routing"},
	{"EVOLVE_ROUTING_MODE", "policy.json router.routing_mode"},
	{"EVOLVE_MANDATORY_PHASES", "policy.json mandatory_phases"},
	{"EVOLVE_CONDITIONAL_MANDATORY", "policy.json router.conditional_mandatory"},
	{"EVOLVE_MAX_OPTIONAL_INSERTIONS", "policy.json router.max_optional_insertions"},
	{"EVOLVE_COMMIT_EVIDENCE", "nothing: no code consumed the commit-evidence stage, so it has no replacement"},
	{"EVOLVE_PHASE_IO", "policy.json gates.phase_io"},
	{"EVOLVE_CLI_HEALTH", "policy.json cli_health.enabled"},
	{"EVOLVE_CLI_MAX_CONCURRENT_*", "policy.json bridge.max_concurrent.<driver>"},
	{"EVOLVE_LANE", "the --lane parameter"},
	{"EVOLVE_INTENT_DELTA", "policy.json workflow.intent_delta"},
	{"EVOLVE_*_CLI", "the --cli <agent>=<driver> parameter or policy.json pins.<phase>.cli"},
	{"EVOLVE_*_MODEL", "the --model <agent>=<tier> parameter or policy.json pins.<phase>.model"},
	{"EVOLVE_*_SYSTEM_PROMPT", "the system_prompt of the agent profile"},
	{"EVOLVE_SYSTEM_PROMPT", "the system_prompt of the agent profile"},
	{"EVOLVE_*_PERMISSION_MODE", "the permission_mode of the agent profile"},
	{"EVOLVE_PROFILE_DIR", "the --profile-dir parameter of evolve phases"},
	{"EVOLVE_PERSONA_OVERRIDE", "the --persona-override parameter of evolve phases"},
	{"EVOLVE_GO_BIN", "go/bin/evolve or evolve on PATH"},
}

func Markers() []string {
	names := make([]string, 0, len(markers))
	for _, m := range markers {
		names = append(names, m.name)
	}
	return names
}

func CellMarkers() []string {
	names := make([]string, 0, len(markers))
	for _, m := range markers {
		if m.inCell {
			names = append(names, m.name)
		}
	}
	return names
}

func UnmovedControls() []string {
	return slices.Clone(unmovedControls)
}

func RetiredControls() []RetiredControl {
	return slices.Clone(retiredControls)
}

func RetiredInUse(environ []string) []RetiredControl {
	out := make([]RetiredControl, 0, len(retiredControls))
	for _, r := range retiredControls {
		if slices.ContainsFunc(environ, func(kv string) bool {
			name, value, _ := strings.Cut(kv, "=")
			matched, _ := path.Match(r.Name, name)
			return matched && value != ""
		}) {
			out = append(out, r)
		}
	}
	return out
}

func Carried(name string) bool {
	if slices.ContainsFunc(markers, func(m marker) bool { return m.name == name }) {
		return true
	}
	return slices.ContainsFunc(unmovedControls, func(pattern string) bool {
		matched, _ := path.Match(pattern, name)
		return matched
	})
}
