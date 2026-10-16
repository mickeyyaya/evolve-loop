// Package capability parses adapter capability manifests and renders the WARN lines, dispatch plan and quality tier.
// See docs/architecture/packages/internal-capability.md.
package capability

import (
	"fmt"
	"os"
	"strings"
)

type Manifest struct {
	BudgetNative      bool
	PermissionScoping bool
}

type Inspection struct {
	Manifest Manifest
	Warns    []string
}

func Inspect(adaptersDir, cli string) (Inspection, error) {
	path := fmt.Sprintf("%s/%s.capabilities.json", adaptersDir, cli)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Inspection{
				Manifest: Manifest{BudgetNative: true, PermissionScoping: true},
			}, nil
		}
		return Inspection{}, fmt.Errorf("capability: read %s: %w", path, err)
	}
	return inspectBytes(body, cli), nil
}

func inspectBytes(body []byte, cli string) Inspection {
	man := Manifest{
		BudgetNative:      extractBool(string(body), "budget_cap_native", true),
		PermissionScoping: extractBool(string(body), "permission_scoping", true),
	}
	var warns []string
	if !man.BudgetNative {
		warns = append(warns, fmt.Sprintf(
			"[adapter-cap] WARN cli=%s missing=budget_cap_native substitute=wall_clock_timeout",
			cli,
		))
	}
	if !man.PermissionScoping {
		warns = append(warns, fmt.Sprintf(
			"[adapter-cap] WARN cli=%s missing=permission_scoping substitute=kernel_role_gate_only",
			cli,
		))
	}
	return Inspection{Manifest: man, Warns: warns}
}

func extractBool(body, field string, def bool) bool {
	supports, ok := extractObject(body, "supports")
	if !ok {
		return def
	}
	needle := fmt.Sprintf("\"%s\"", field)
	idx := strings.Index(supports, needle)
	if idx < 0 {
		return def
	}
	tail := strings.TrimSpace(supports[idx+len(needle):])
	if len(tail) == 0 || tail[0] != ':' {
		return def
	}
	tail = strings.TrimSpace(tail[1:])
	switch {
	case strings.HasPrefix(tail, "true"):
		return true
	case strings.HasPrefix(tail, "false"):
		return false
	default:
		return def
	}
}

func extractObject(body, name string) (string, bool) {
	needle := fmt.Sprintf("\"%s\"", name)
	idx := strings.Index(body, needle)
	if idx < 0 {
		return "", false
	}
	tail := strings.TrimSpace(body[idx+len(needle):])
	if len(tail) == 0 || tail[0] != ':' {
		return "", false
	}
	tail = strings.TrimSpace(tail[1:])
	if len(tail) == 0 || tail[0] != '{' {
		return "", false
	}
	depth := 0
	for i, r := range tail {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return tail[1:i], true
			}
		}
	}
	return "", false
}

type DispatchPlan struct {
	CLI                string
	Model              string
	CLIResolutionSrc   string
	CapBudgetNative    bool
	CapPermissionScope bool
	Warns              []string
}

func (p DispatchPlan) PlanJSON() string {
	return fmt.Sprintf(
		`{"cli":"%s","model":"%s","cli_resolution_source":"%s","cap_budget_native":%s,"cap_permission_scoping":%s,"capability_warns":%s}`,
		jsonEscape(p.CLI),
		jsonEscape(p.Model),
		jsonEscape(p.CLIResolutionSrc),
		boolToken(p.CapBudgetNative),
		boolToken(p.CapPermissionScope),
		warnsJSON(p.Warns),
	)
}

func boolToken(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func jsonEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func warnsJSON(warns []string) string {
	if len(warns) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, w := range warns {
		if i > 0 {
			b.WriteByte(',')
		}
		stripped := strings.TrimPrefix(w, "[adapter-cap] WARN ")
		b.WriteByte('"')
		b.WriteString(jsonEscape(stripped))
		b.WriteByte('"')
	}
	b.WriteByte(']')
	return b.String()
}
