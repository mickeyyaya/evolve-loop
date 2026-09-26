package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strconv"
)

// registryDoc is the subset of phase-registry.json this loader reads.
type registryDoc struct {
	Config registryConfig  `json:"config"`
	Phases []registryPhase `json:"phases"`
}

type registryConfig struct {
	DynamicRouting        string                         `json:"dynamic_routing"`
	RoutingMode           string                         `json:"routing_mode"`
	ModelRouting          string                         `json:"model_routing"`
	MandatoryPhases       []string                       `json:"mandatory_phases"`
	SpineOrder            []string                       `json:"spine_order"`
	LegalSuccessors       map[string][]string            `json:"legal_successors"`
	ConditionalMandatory  map[string]string              `json:"conditional_mandatory"`
	MaxOptionalInsertions *int                           `json:"max_optional_insertions"`
	GoalRecipes           map[string][]string            `json:"goal_recipes"`
	DeliverableKinds      map[string]DeliverableKindSpec `json:"deliverable_kinds"`
	Workflow              registryWorkflow               `json:"workflow"`
}

type registryWorkflow struct {
	CompactPrompts *bool `json:"compact_prompts,omitempty"`
}

type registryPhase struct {
	Name     string        `json:"name"`
	Optional bool          `json:"optional"`
	Enabled  string        `json:"enabled"`
	Routing  *RoutingBlock `json:"routing"`
}

const baselineNote = "running on the built-in baseline — no triage, no registry order"

// readRegistry is silent only on absence. An unreadable or malformed registry warns, because the
// loop would otherwise run on the triage-less baseline with no trace.
func (l *Loader) readRegistry(path string, ws *[]Warning) (registryDoc, bool) {
	raw, err := l.readFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return registryDoc{}, false
	}
	if err != nil {
		warn(ws, codeRegistryUnreadable, fmt.Sprintf("phase registry unreadable (%s): read %s: %v", baselineNote, path, err),
			map[string]string{"path": path, "err": err.Error()})
		return registryDoc{}, false
	}
	var doc registryDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		warn(ws, codeRegistryMalformed, fmt.Sprintf("phase registry malformed (%s): parse %s: %v", baselineNote, path, err),
			map[string]string{"path": path, "err": err.Error()})
		return registryDoc{}, false
	}
	return doc, true
}

func applyRegistry(cfg *RoutingConfig, doc registryDoc, ws *[]Warning) {
	applyRegistryDials(cfg, doc.Config, ws)
	applyDeliverableKinds(cfg, doc.Config.DeliverableKinds, ws)
	applyConditional(cfg, doc.Config.ConditionalMandatory, ws)
	applyPhases(cfg, doc.Phases, ws)
}

// applyRegistryDials keeps the compiled default for any value the registry omits; the pointer
// fields tell an explicit 0 or false from absence.
func applyRegistryDials(cfg *RoutingConfig, c registryConfig, ws *[]Warning) {
	if c.DynamicRouting != "" {
		cfg.Stage = parseStage(c.DynamicRouting, "dynamic_routing", ws)
	}
	if c.RoutingMode != "" {
		cfg.Mode = parseMode(c.RoutingMode, ws)
	}
	if c.ModelRouting != "" {
		cfg.ModelRouting = parseModelRouting(c.ModelRouting, "model_routing", ws)
	}
	if len(c.MandatoryPhases) > 0 {
		cfg.Mandatory = c.MandatoryPhases
	}
	if len(c.SpineOrder) > 0 {
		cfg.SpineOrder = c.SpineOrder
	}
	if len(c.LegalSuccessors) > 0 {
		cfg.LegalSuccessors = c.LegalSuccessors
	}
	if c.MaxOptionalInsertions != nil {
		cfg.MaxInsertions = *c.MaxOptionalInsertions
	}
	if len(c.GoalRecipes) > 0 {
		cfg.GoalRecipes = c.GoalRecipes
	}
	if c.Workflow.CompactPrompts != nil {
		cfg.CompactPrompts = *c.Workflow.CompactPrompts
	}
}

// applyDeliverableKinds installs a spec even when it warns: the registry owns root and
// min_options, so a consumer must see the hole rather than a silent default.
func applyDeliverableKinds(cfg *RoutingConfig, kinds map[string]DeliverableKindSpec, ws *[]Warning) {
	if len(kinds) == 0 {
		return
	}
	cfg.DeliverableKinds = kinds
	for _, kind := range slices.Sorted(maps.Keys(kinds)) {
		spec := kinds[kind]
		if spec.Root == "" || spec.MinOptions <= 0 {
			warn(ws, codeUnknownValue, fmt.Sprintf("deliverable_kinds[%s]: root and min_options must be set (root=%q, min_options=%d)", kind, spec.Root, spec.MinOptions),
				map[string]string{"key": "deliverable_kinds[" + kind + "]", "root": spec.Root, "min_options": strconv.Itoa(spec.MinOptions)})
		}
	}
}

// applyConditional merges over the compiled rules, so the tdd default survives a registry that omits it.
func applyConditional(cfg *RoutingConfig, rules map[string]string, ws *[]Warning) {
	for _, phase := range slices.Sorted(maps.Keys(rules)) {
		expr := rules[phase]
		rule, err := parseCondRule(expr)
		if err != nil {
			warn(ws, codeUnknownValue, fmt.Sprintf("conditional_mandatory[%s]=%q: %v", phase, expr, err),
				map[string]string{"key": "conditional_mandatory[" + phase + "]", "value": expr, "err": err.Error()})
			continue
		}
		cfg.Conditional[phase] = rule
	}
}

func applyPhases(cfg *RoutingConfig, phases []registryPhase, ws *[]Warning) {
	for _, p := range phases {
		if p.Name == "" {
			continue
		}
		cfg.Order = append(cfg.Order, p.Name)
		if p.Enabled != "" {
			from := len(*ws)
			cfg.PhaseEnable[p.Name] = parseEnable(p.Enabled, ws)
			stamp((*ws)[from:], "key", "phases["+p.Name+"].enabled")
		}
		if p.Routing != nil {
			cfg.Triggers[p.Name] = *p.Routing
		}
	}
}
