package inputindex

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

const (
	LayerUpstream   = "L1"
	LayerRecovery   = "L2"
	tokenEstimate   = "ceil(bytes/3)"
	stageShadow     = "shadow"
	kindHandoffAsks = "handoff-asks"
	defaultL1Tokens = 8000
	defaultL2Tokens = 6000
)

type Budget struct {
	L1Tokens int
	L2Tokens int
}

type ContextPolicy struct {
	Stage    string
	Budget   Budget
	Required []string
	Inputs   []string
}

type LayerBudget struct {
	LimitTokens    int   `json:"limit_tokens"`
	RequiredTokens int64 `json:"required_tokens"`
	Over           int   `json:"over,omitempty"`
}

var layerOfClass = map[string]string{ClassDeliverable: LayerUpstream, ClassLineage: LayerRecovery}

func tokensOf(bytes int64) int64 {
	return (bytes + 2) / 3
}

func handoffWhy(name, phase string) string {
	pair := strings.TrimSuffix(strings.TrimPrefix(name, "handoff-"), ".json")
	if from, ok := strings.CutSuffix(pair, "-"+phase); ok && from != "" {
		return "asks of " + from + " for " + phase
	}
	return "asks of the pair " + pair
}

func annotate(idx Index, req Request) Index {
	limits := map[string]int{LayerUpstream: cmp.Or(req.Context.Budget.L1Tokens, defaultL1Tokens), LayerRecovery: cmp.Or(req.Context.Budget.L2Tokens, defaultL2Tokens)}
	used, over := map[string]int64{}, map[string]int{}
	isRequired := requiredRule(req, idx.Lineage)
	out := idx
	out.Stage, out.Estimate, out.Required = cmp.Or(req.Context.Stage, stageShadow), tokenEstimate, []string{}
	out.Entries, out.Notes = make([]Entry, len(idx.Entries)), slices.Clone(idx.Notes)
	for i, e := range idx.Entries {
		e.Layer, e.Tokens = layerOfClass[e.Class], tokensOf(e.Bytes)
		switch {
		case !isRequired(e):
		case used[e.Layer]+e.Tokens > int64(limits[e.Layer]):
			e.Why, over[e.Layer] = e.Why+"; not required: over the "+e.Layer+" budget", over[e.Layer]+1
			out.Notes = append(out.Notes, fmt.Sprintf("%s (%s, %d tokens) is not required: over the %s budget of %d tokens", e.ID, e.Kind, e.Tokens, e.Layer, limits[e.Layer]))
		default:
			e.Required, used[e.Layer] = true, used[e.Layer]+e.Tokens
			e.Why += "; required by the " + req.Phase + " inputs"
			out.Required = append(out.Required, e.ID)
		}
		out.Entries[i] = e
	}
	out.Budgets = map[string]LayerBudget{}
	for layer, limit := range limits {
		out.Budgets[layer] = LayerBudget{LimitTokens: limit, RequiredTokens: used[layer], Over: over[layer]}
	}
	return out
}

func requiredRule(req Request, lineage []int) func(Entry) bool {
	kinds, fromEdges := requiredKinds(req.Context)
	upstreams, _ := phaseio.EdgeUpstreams(req.Phase)
	newest := 0
	if len(lineage) > 0 {
		newest = lineage[len(lineage)-1]
	}
	return func(e Entry) bool {
		switch {
		case e.Kind == kindHandoffAsks:
			return strings.HasSuffix(e.Path, "-"+req.Phase+".json")
		case e.Class == ClassLineage:
			return e.Cycle == newest && (e.Kind == "retrospective" || e.Kind == "failure-digest")
		case fromEdges:
			return slices.Contains(upstreams, e.Phase)
		default:
			return slices.Contains(kinds, e.Kind)
		}
	}
}

func requiredKinds(c ContextPolicy) ([]string, bool) {
	if c.Required != nil {
		return c.Required, false
	}
	if len(c.Inputs) == 0 {
		return nil, true
	}
	kinds := make([]string, len(c.Inputs))
	for i, in := range c.Inputs {
		kinds[i] = kindOf(filepath.Base(in))
	}
	return kinds, false
}

var (
	ErrUnknownEntry = errors.New("no entry with this id")
	ErrOutsideRoot  = errors.New("the path of the entry is outside the project root")
	ErrChangedBody  = errors.New("the body differs from the sha256 of the index")
)

func (idx Index) Open(projectRoot, id string) (Entry, []byte, error) {
	i := slices.IndexFunc(idx.Entries, func(e Entry) bool { return e.ID == id })
	if i < 0 {
		return Entry{}, nil, fmt.Errorf("inputindex: %q: %w", id, ErrUnknownEntry)
	}
	e := idx.Entries[i]
	path, ok := underRoot(projectRoot, e.Path)
	if !ok {
		return e, nil, fmt.Errorf("inputindex: %s %q: %w", id, e.Path, ErrOutsideRoot)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return e, nil, fmt.Errorf("inputindex: open %s: %w", id, err)
	}
	if got := digest(body); got != e.SHA256 {
		return e, nil, fmt.Errorf("inputindex: %s %s has sha256 %s, the index has %s: %w", id, e.Path, got, e.SHA256, ErrChangedBody)
	}
	return e, body, nil
}
