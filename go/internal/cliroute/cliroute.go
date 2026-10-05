// Package cliroute compiles the policy's cli_routing table and resolves every launch's CLI chain from it.
// See docs/architecture/packages/internal-cliroute.md.
package cliroute

import (
	"fmt"
	"slices"
	"strings"

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

type Request struct {
	Agent        string
	Phase        string
	ProjectRoot  string
	DefaultModel string
	Env          map[string]string
	Overlay      llmroute.Overlay
	CallerCLI    string
	Expand       llmroute.AutoModel
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

type Router struct {
	table Table
	host  Host
}

func New(t Table, h Host) (*Router, error) {
	var refused []string
	for _, f := range t.findings {
		if f.Severity == SeverityError {
			refused = append(refused, f.Key+": "+f.Message)
		}
	}
	if len(refused) > 0 {
		return nil, fmt.Errorf("cliroute: the routing table has %d error finding(s): %s", len(refused), strings.Join(refused, "; "))
	}
	return &Router{table: t, host: h}, nil
}

func (r *Router) Resolve(req Request) (Decision, error) {
	if !r.table.declared {
		return r.resolveLegacy(req)
	}
	return r.resolveDeclared(req)
}

func (r *Router) logf(format string, args ...any) {
	if r.host.Logf != nil {
		r.host.Logf(format, args...)
	}
}

func (r *Router) bench(req Request, plan llmroute.Plan) llmroute.Plan {
	if r.host.Bench == nil {
		return plan
	}
	label := req.Phase
	if label == "" {
		label = req.Agent
	}
	return r.host.Bench(req.ProjectRoot, label, plan, req.Env)
}

func familyOf(cli string) string {
	return llmroute.Family(llmroute.DefaultDriverForFamily(cli))
}
