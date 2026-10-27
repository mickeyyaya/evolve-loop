package config

import (
	"fmt"
	"strings"
)

type envDial struct {
	key   string
	apply func(cfg *RoutingConfig, v string, ws *[]Warning)
}

var envDials = []envDial{
	{key: "EVOLVE_SANDBOX", apply: envSandbox},
}

func applyEnv(cfg *RoutingConfig, env map[string]string, ws *[]Warning) {
	for _, d := range envDials {
		v := env[d.key]
		if v == "" {
			continue
		}
		from := len(*ws)
		d.apply(cfg, v, ws)
		stamp((*ws)[from:], "key", d.key)
	}
}

func envSandbox(cfg *RoutingConfig, v string, ws *[]Warning) {
	cfg.SandboxMode = parseSandboxMode(v, cfg.SandboxMode, ws)
}

func parseSandboxMode(v, current string, ws *[]Warning) string {
	switch t := strings.TrimSpace(v); t {
	case SandboxModeAuto, SandboxModeOn, SandboxModeOff:
		return t
	}
	warn(ws, codeUnknownValue, fmt.Sprintf("EVOLVE_SANDBOX=%q unknown (want auto|on|off), defaulting to %q", v, current),
		map[string]string{"key": "EVOLVE_SANDBOX", "value": v, "default": current})
	return current
}
