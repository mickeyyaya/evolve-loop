// Package cliroute compiles the policy's cli_routing table and resolves every launch's CLI chain from it.
// See docs/architecture/packages/internal-cliroute.md.
package cliroute

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
)

type Finding struct {
	Severity Severity
	Key      string
	Message  string
}

type Catalog interface {
	Get(name string) (phasespec.PhaseSpec, bool)
	Names() []string
}

type ProfileSource interface {
	Get(name string) (profiles.Profile, error)
	List() ([]string, error)
}

type Host struct {
	LookPath func(string) (string, error)
	Discover func() []string
	Bench    func(projectRoot, phase string, plan llmroute.Plan, env map[string]string) llmroute.Plan
	Logf     func(format string, args ...any)
}

type Launch string

const (
	LaunchAdvisor    Launch = "advisor"
	LaunchClassifier Launch = "classifier"
)

const ClassifierAgent = "model-classifier"

type Request struct {
	Agent        string
	Phase        string
	ProjectRoot  string
	DefaultModel string
	Env          map[string]string
	Overlay      llmroute.Overlay
	CallerCLI    string
	Expand       llmroute.AutoModel
	BypassPolicy bool
	Launch       Launch
}

type Decision struct {
	Plan    llmroute.Plan
	Rule    string
	Allowed []string
	Trace   []string
}

func (d Decision) Allows(cli string) bool {
	return d.Allowed == nil || slices.Contains(d.Allowed, familyOf(cli))
}

func (d Decision) Legacy() bool {
	return strings.HasPrefix(d.Rule, legacyRulePrefix)
}

type Router struct {
	tables      atomic.Pointer[tablePair]
	recompiling sync.Mutex
	host        Host
	setup       *Setup
}

type tablePair struct {
	declared Table
	bypass   Table
}

type resolver struct {
	table Table
	host  Host
}

func New(t Table, h Host) (*Router, error) {
	if err := refusal(t.findings); err != nil {
		return nil, err
	}
	r := &Router{host: h}
	r.tables.Store(&tablePair{declared: t, bypass: t})
	return r, nil
}

func refusal(findings []Finding) error {
	var refused []string
	for _, f := range findings {
		if f.Severity == SeverityError {
			refused = append(refused, f.Key+": "+f.Message)
		}
	}
	if len(refused) > 0 {
		return fmt.Errorf("cliroute: the routing table has %d error finding(s): %s", len(refused), strings.Join(refused, "; "))
	}
	return nil
}

func (r *Router) Resolve(req Request) (Decision, error) {
	pair := r.tables.Load()
	v := resolver{table: pair.declared, host: r.host}
	if req.BypassPolicy {
		v.table = pair.bypass
	}
	if !v.table.declared {
		return v.resolveLegacy(req)
	}
	return v.resolveDeclared(req)
}

func (v resolver) logf(format string, args ...any) {
	if v.host.Logf != nil {
		v.host.Logf(format, args...)
	}
}

func (v resolver) bench(req Request, plan llmroute.Plan) llmroute.Plan {
	if v.host.Bench == nil {
		return plan
	}
	label := req.Phase
	if label == "" {
		label = req.Agent
	}
	return v.host.Bench(req.ProjectRoot, label, plan, req.Env)
}

func familyOf(cli string) string {
	return llmroute.Family(llmroute.DefaultDriverForFamily(cli))
}

func (d Decision) WalkAt(tier string, benched func(cli string) bool) (llmroute.Plan, bool, error) {
	var permitted []string
	for _, cli := range d.Plan.Candidates {
		if d.Plan.Permits(cli, tier) {
			permitted = append(permitted, cli)
		}
	}
	if len(permitted) == 0 {
		return llmroute.Plan{}, false, fmt.Errorf("%w: no candidate of %v is permitted at tier %s", ErrRefused, d.Plan.Candidates, tier)
	}
	lead, healthy := permitted[0], false
	for _, cli := range permitted {
		if !benched(cli) {
			lead, healthy = cli, true
			break
		}
	}
	return llmroute.Plan{Candidates: leadWith(lead, permitted), Triggers: d.Plan.Triggers}, healthy, nil
}

func (d Decision) Walk() (llmroute.Plan, error) {
	walk, _, err := d.WalkAt(d.Plan.Model, func(string) bool { return false })
	return walk, err
}
