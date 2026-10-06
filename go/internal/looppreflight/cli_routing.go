package looppreflight

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type Routing struct {
	Drivers  []string
	Findings []cliroute.Finding
}

type routingOnce struct {
	once    sync.Once
	compile func() (Routing, error)
	routing Routing
	err     error
}

func onceRouting(compile func() (Routing, error)) *routingOnce {
	return &routingOnce{compile: compile}
}

func (r *routingOnce) get(list func() ([]string, error), get func(string) (profiles.Profile, error)) (Routing, error) {
	r.once.Do(func() {
		compile := r.compile
		if compile == nil {
			compile = profileRouting(list, get)
		}
		r.routing, r.err = compile()
	})
	return r.routing, r.err
}

func (o resolved) routed() (Routing, error) {
	return o.routing.get(o.profileLister, o.profileGetter)
}

func (o resolved) drivers() []string {
	routed, err := o.routed()
	if err != nil {
		return nil
	}
	return routed.Drivers
}

type seamProfiles struct {
	list func() ([]string, error)
	get  func(string) (profiles.Profile, error)
}

func (s seamProfiles) List() ([]string, error)                   { return s.list() }
func (s seamProfiles) Get(name string) (profiles.Profile, error) { return s.get(name) }

func profileRouting(list func() ([]string, error), get func(string) (profiles.Profile, error)) func() (Routing, error) {
	return func() (Routing, error) {
		return CompileRouting(func() (*cliroute.Router, []cliroute.Finding, error) {
			return cliroute.Build(cliroute.Setup{Profiles: seamProfiles{list: list, get: get}})
		}, list)
	}
}

func CompileRouting(compile func() (*cliroute.Router, []cliroute.Finding, error), list func() ([]string, error)) (Routing, error) {
	router, findings, err := compile()
	if err != nil {
		return Routing{Findings: findings}, err
	}
	agents, err := list()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Routing{Findings: findings}, fmt.Errorf("list the profiles that route the batch: %w", err)
	}
	drivers, err := RoutedDrivers(router, agents)
	return Routing{Drivers: drivers, Findings: findings}, err
}

func RoutedDrivers(router *cliroute.Router, agents []string) ([]string, error) {
	var drivers []string
	for _, agent := range agents {
		d, err := router.Resolve(cliroute.Request{Agent: agent})
		if err != nil {
			return nil, err
		}
		for _, cli := range d.Plan.Candidates {
			if !slices.Contains(drivers, cli) {
				drivers = append(drivers, cli)
			}
		}
	}
	sort.Strings(drivers)
	return drivers, nil
}

func checkCLIRouting(o resolved) CheckResult {
	const name = "cli-routing"
	routed, err := o.routed()
	detail := findingLines(routed.Findings)
	switch {
	case err != nil:
		return CheckResult{Name: name, Level: LevelHalt, Message: "the CLI routing table does not compile: " + err.Error(), Detail: detail}
	case detail != "":
		return CheckResult{Name: name, Level: LevelWarn, Message: fmt.Sprintf("the CLI routing table compiles with %d warning(s)", len(routed.Findings)), Detail: detail}
	}
	return CheckResult{Name: name, Level: LevelPass, Message: "the CLI routing table compiles; drivers " + strings.Join(routed.Drivers, ", ")}
}

func findingLines(findings []cliroute.Finding) string {
	lines := make([]string, 0, len(findings))
	for _, f := range findings {
		lines = append(lines, fmt.Sprintf("%s %s: %s", f.Severity, f.Key, f.Message))
	}
	return strings.Join(lines, "\n")
}
