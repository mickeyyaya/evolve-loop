// Package capability parses adapter capability manifests
// (<adapter>.capabilities.json) and emits the WARN messages + dispatch-plan
// JSON envelope used by cmd_validate_profile in
// legacy/scripts/dispatch/subagent-run.sh.
//
// The manifest defines which kernel capabilities the adapter natively
// supports. Missing capabilities trigger WARN-level substitutions
// (e.g. wall-clock timeout instead of native budget cap) that the runner
// surfaces to operators before the agent fires.
package capability

import (
	"fmt"
	"os"
	"strings"
)

// Manifest is the parsed shape of an adapter capabilities.json. Only the
// fields cmd_validate_profile inspects are typed — other keys are tolerated
// but ignored. The two booleans default to true when absent (matches bash
// `if . == null then "true" else tostring`).
type Manifest struct {
	BudgetNative      bool
	PermissionScoping bool
}

// Inspection is the output of Inspect: parsed manifest + the ordered list of
// WARN lines bash would print to stderr. CLI consumers print these verbatim;
// programmatic consumers serialize them into the dispatch-plan envelope.
type Inspection struct {
	Manifest Manifest
	Warns    []string // each element is the full "[adapter-cap] WARN cli=... missing=... substitute=..." line
}

// Inspect loads <adaptersDir>/<cli>.capabilities.json and returns the
// inspection. Missing file → both supports default to true, no warns (bash:
// the manifest is optional and silently absent for adapters that haven't
// declared a manifest yet). I/O errors other than ENOENT bubble up.
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

// inspectBytes is the pure-string entry point for tests. Same defaults: both
// supports true when the field is absent or null. Bash uses jq with
// `if . == null then "true" else tostring end`; we mirror that.
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

// extractBool locates `"<field>": <bool>` inside the supports block. Returns
// def when the field is absent, null, or not a literal bool. Defensive
// against minor whitespace + ordering differences across manifests.
func extractBool(body, field string, def bool) bool {
	// Walk to the supports block first so a stray match outside .supports
	// can't fool us. Bash uses `jq -r '.supports.<field>'`.
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

// extractObject returns the inner contents of `"<name>": { ... }` (without
// braces) or ("", false) on miss. Tolerates whitespace + nested objects.
// Pure string scanning — keeps the package free of encoding/json.
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

// DispatchPlan is the serialized envelope written to $EVOLVE_DISPATCH_PLAN_LOG.
// Field order matches bash printf so byte-for-byte file diff stays clean.
type DispatchPlan struct {
	CLI                string
	Model              string
	CLIResolutionSrc   string
	CapBudgetNative    bool
	CapPermissionScope bool
	Warns              []string
}

// PlanJSON renders a single-line JSON envelope identical to bash printf at
// subagent-run.sh:555 — same field order, same boolean rendering. The
// caller writes the result to EVOLVE_DISPATCH_PLAN_LOG.
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

// jsonEscape quotes the special characters JSON would. Bash uses sed
// 's/"/\\"/g' only for the warn entries; the top-level fields use printf
// %s without escaping, so values containing " would break bash too. Mirror
// that: only escape " in our top-level fields for consistency. Keeps the
// envelope a valid one-line JSON object.
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
		// Bash strips the leading "[adapter-cap] WARN " prefix and stores
		// just "cli=... missing=... substitute=...". Mirror that.
		stripped := strings.TrimPrefix(w, "[adapter-cap] WARN ")
		b.WriteByte('"')
		b.WriteString(jsonEscape(stripped))
		b.WriteByte('"')
	}
	b.WriteByte(']')
	return b.String()
}
