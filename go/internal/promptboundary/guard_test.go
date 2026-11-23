package promptboundary_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	adapter "github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/debugger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/intent"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/scout"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/internal/promptboundary"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

type captureBridge struct {
	got    []core.BridgeRequest
	cancel context.CancelFunc
}

func (c *captureBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	c.got = append(c.got, req)
	c.cancel()
	return core.BridgeResponse{}, errors.New("captured")
}

func (c *captureBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

type site struct {
	name string
	make func(br core.Bridge, p *prompts.Loader) core.PhaseRunner
}

var sites = []site{
	{"audit", func(br core.Bridge, p *prompts.Loader) core.PhaseRunner {
		return audit.New(audit.Config{Bridge: br, Prompts: p})
	}},
	{"build", func(br core.Bridge, p *prompts.Loader) core.PhaseRunner {
		return build.New(build.Config{Bridge: br, Prompts: p})
	}},
	{"debugger", func(br core.Bridge, p *prompts.Loader) core.PhaseRunner {
		return debugger.New(debugger.Config{Bridge: br, Prompts: p})
	}},
	{"intent", func(br core.Bridge, p *prompts.Loader) core.PhaseRunner {
		return intent.New(intent.Config{Bridge: br, Prompts: p})
	}},
	{"retro", func(br core.Bridge, p *prompts.Loader) core.PhaseRunner {
		return retro.New(retro.Config{Bridge: br, Prompts: p})
	}},
	{"scout", func(br core.Bridge, p *prompts.Loader) core.PhaseRunner {
		return scout.New(scout.Config{Bridge: br, Prompts: p})
	}},
	{"tdd", func(br core.Bridge, p *prompts.Loader) core.PhaseRunner {
		return tdd.New(tdd.Config{Bridge: br, Prompts: p})
	}},
	{"triage", func(br core.Bridge, p *prompts.Loader) core.PhaseRunner {
		return triage.New(triage.Config{Bridge: br, Prompts: p})
	}},
}

var exemptPhases = map[string]string{
	"ship": "deterministic phase: it composes no LLM prompt",
}

var staticPhaseFields = map[string]string{
	"ProjectRoot": "selects the project; constant for the life of a project",
}

var unfillablePhaseFields = map[string]string{
	"Fence": "pointer, not text", "WorktreeVerified": "bool flag", "BuildExplanation": "pointer, not text",
	"BuildExplanationState": "enum string with a closed value set", "Input": "interface, not text",
	"BypassPolicy": "bool flag", "ComposePhases": "bool flag", "Spec": "pointer, not text",
	"UpstreamSignals": "map of any", "Assignment": "struct of tier and effort", "Driver": "struct, not text",
	"BudgetScale": "float, not text", "Ladder": "pointer, not text", "BeforeClose": "func, not text",
	"PermissionMode": "closed value set that selects the CLI mode",
}

var staticContextValues = map[string]string{
	"previous_verdict": core.VerdictFAIL,
}

const exemptOperatorDirectives = "OperatorDirectives"

var bridgeRequestFields = map[string]string{
	"CLI": "set by dispatch chain after compose", "Profile": "derived: profile path per phase",
	"Model": "derived: routing", "Effort": "derived: routing", "Prompt": "derived: composed prompt",
	"Workspace": "same as PhaseRequest.Workspace", "Worktree": "same as PhaseRequest.Worktree",
	"RunID": "same as PhaseRequest.RunID", "ProjectRoot": "same as PhaseRequest.ProjectRoot",
	"StdoutLog": "derived: log path", "StderrLog": "derived: log path",
	"ArtifactPath": "derived: workspace plus artifact name", "SecondaryArtifacts": "derived: hook",
	"Completion": "derived: request", "Agent": "derived: phase name", "Contract": "derived: contract id",
	"Cycle": "same as PhaseRequest.Cycle", "Attempt": "same as PhaseRequest.Attempt",
	"ChainAttempt": "derived: constant", "FreshSession": "derived: retry policy",
	"BudgetScale": "same as PhaseRequest.BudgetScale", "RequireSandbox": "derived: explanation flag",
	"SeatTokenFile": "derived: seat", "Env": "same as PhaseRequest.Env", "ExtraFlags": "derived: routing",
	"PermissionMode": "derived: profile", "InteractivePolicy": "derived: profile",
	"SystemPrompt": "derived: profile rules, static per phase", "Skills": "derived: policy overlays",
	"CorrectionDirective": "same as PhaseRequest.CorrectionDirective",
	"OperatorDirectives":  "same as PhaseRequest.OperatorDirectives",
	"SessionName":         "derived: session", "Ladder": "same as PhaseRequest.Ladder",
	"BeforeClose": "same as PhaseRequest.BeforeClose",
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

var keyCache sync.Map

func sourceKeys(t *testing.T, root string, re *regexp.Regexp) []string {
	t.Helper()
	if cached, ok := keyCache.Load(re); ok {
		return cached.([]string)
	}
	seen := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(root, "go", "internal"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			seen[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk sources: %v", err)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	keyCache.Store(re, out)
	return out
}

var (
	contextKeyRE = regexp.MustCompile(`Context\["([a-z_]+)"\]`)
	constKeyRE   = regexp.MustCompile(`CtxKey\w+\s*=\s*"([a-z_]+)"`)
	signalKeyRE  = regexp.MustCompile(`Signals\["([a-z_A-Z.]+)"\]`)
)

func fillRequest(t *testing.T, root string, tag string, base int, operator string) core.PhaseRequest {
	t.Helper()
	var req core.PhaseRequest
	v := reflect.ValueOf(&req).Elem()
	ty := v.Type()
	for i := 0; i < ty.NumField(); i++ {
		f := ty.Field(i)
		if _, ok := staticPhaseFields[f.Name]; ok {
			continue
		}
		if _, ok := unfillablePhaseFields[f.Name]; ok {
			continue
		}
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(tag + ":" + f.Name)
		case reflect.Int:
			fv.SetInt(int64(base + i))
		case reflect.Map:
			if fv.Type().Key().Kind() != reflect.String || fv.Type().Elem().Kind() != reflect.String {
				t.Fatalf("PhaseRequest.%s: map of %s is not fillable; classify it in unfillablePhaseFields", f.Name, fv.Type())
			}
			fv.Set(reflect.MakeMap(fv.Type()))
		default:
			t.Fatalf("PhaseRequest.%s (%s) is neither fillable by a sentinel nor classified; add it to staticPhaseFields or unfillablePhaseFields with a reason", f.Name, fv.Kind())
		}
	}
	req.ProjectRoot = t.TempDir()
	req.Workspace = filepath.Join(t.TempDir(), tag+"-ws")
	req.Worktree = filepath.Join(t.TempDir(), tag+"-wt")
	req.ExplanationDocumentationVersion = 0
	req.Attempt = 0
	req.AuditRound = 0
	req.RunID = ""
	req.OperatorDirectives = operator
	keys := append(sourceKeys(t, root, contextKeyRE), sourceKeys(t, root, constKeyRE)...)
	for _, k := range keys {
		req.Context[k] = tag + ":ctx:" + k
	}
	for k, val := range staticContextValues {
		req.Context[k] = val
	}
	for _, k := range sourceKeys(t, root, signalKeyRE) {
		req.Signals[k] = tag + ":sig:" + k
	}
	req.Env["EVOLVE_SENTINEL"] = tag + ":env"
	return req
}

func sentinelsOf(req core.PhaseRequest, tag string) []string {
	var out []string
	v := reflect.ValueOf(req)
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		if name == exemptOperatorDirectives {
			continue
		}
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.String:
			if _, static := staticPhaseFields[name]; !static && strings.HasPrefix(fv.String(), tag+":") {
				out = append(out, fv.String())
			}
		case reflect.Map:
			for _, k := range fv.MapKeys() {
				if val := fv.MapIndex(k).String(); strings.HasPrefix(val, tag+":") {
					out = append(out, val)
				}
			}
		}
	}
	return out
}

func composeThroughRealPath(t *testing.T, s site, req core.PhaseRequest, loader *prompts.Loader) []string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cb := &captureBridge{cancel: cancel}
	_, _ = s.make(cb, loader).Run(ctx, req)
	if len(cb.got) == 0 {
		t.Fatalf("site %s: the runner never reached the bridge; the guard has no prompt to check", s.name)
	}
	ad := adapter.New()
	var out []string
	for _, br := range cb.got {
		if br.CLI == "" {
			br.CLI = "claude-tmux"
		}
		if br.Profile == "" {
			br.Profile = "/profile"
		}
		composed, err := ad.ComposePrompt(br)
		if err != nil {
			t.Fatalf("site %s: real ComposePrompt: %v", s.name, err)
		}
		out = append(out, composed.Prompt)
	}
	return out
}

func TestSites_NoDynamicBytesAboveBoundary(t *testing.T) {
	root := repoRoot(t)
	loader := prompts.NewForProject(root)
	const directives = "## Operator Directives\n\nFIXED-PER-CYCLE"
	for _, s := range sites {
		t.Run(s.name, func(t *testing.T) {
			reqA := fillRequest(t, root, "A", 100000, directives)
			reqB := fillRequest(t, root, "B", 200000, directives)
			a := composeThroughRealPath(t, s, reqA, loader)
			b := composeThroughRealPath(t, s, reqB, loader)
			prefixA := promptboundary.StaticPrefix(a[0])
			prefixB := promptboundary.StaticPrefix(b[0])
			if prefixA != prefixB {
				t.Fatalf("static prefix differs between two cycles (first diff at byte %d)", firstDiff(prefixA, prefixB))
			}
			for _, tok := range sentinelsOf(reqA, "A") {
				if strings.Contains(prefixA, tok) {
					t.Errorf("dynamic value %q is above the boundary", tok)
				}
			}
			if !strings.Contains(prefixA, "FIXED-PER-CYCLE") {
				t.Errorf("named exemption broken: OperatorDirectives must stay above the boundary")
			}
		})
	}
}

func firstDiff(a, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func TestSites_BoundaryExactlyOnce(t *testing.T) {
	root := repoRoot(t)
	loader := prompts.NewForProject(root)
	for _, s := range sites {
		t.Run(s.name, func(t *testing.T) {
			req := fillRequest(t, root, "A", 100000, "")
			for _, p := range composeThroughRealPath(t, s, req, loader) {
				if err := boundaryOnce(p); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func boundaryOnce(prompt string) error {
	if n := strings.Count(prompt, promptboundary.Boundary); n != 1 {
		return fmt.Errorf("composed prompt holds the boundary %d times, want exactly 1", n)
	}
	return nil
}

func TestBoundaryOnce_RejectsPersonaQuotingHeader(t *testing.T) {
	clean := "persona" + promptboundary.Boundary + "- cycle: 1\n"
	quoting := "persona" + promptboundary.Boundary + "quoted" + promptboundary.Boundary + "- cycle: 1\n"
	if err := boundaryOnce(clean); err != nil {
		t.Errorf("clean prompt rejected: %v", err)
	}
	if err := boundaryOnce(quoting); err == nil {
		t.Error("a body that quotes the header must fail the exactly-once check")
	}
	if err := boundaryOnce("persona only"); err == nil {
		t.Error("a prompt with no boundary must fail the exactly-once check")
	}
}

func TestSites_CoverAllRegistered(t *testing.T) {
	covered := map[string]bool{}
	for _, s := range sites {
		covered[s.name] = true
	}
	for _, name := range registry.Names() {
		if covered[name] {
			continue
		}
		if reason, ok := exemptPhases[name]; !ok || reason == "" {
			t.Errorf("registered phase %q is neither a guarded site nor exempt with a reason", name)
		}
	}
}

func TestRequestFields_AllClassified(t *testing.T) {
	bt := reflect.TypeOf(core.BridgeRequest{})
	for i := 0; i < bt.NumField(); i++ {
		if bridgeRequestFields[bt.Field(i).Name] == "" {
			t.Errorf("BridgeRequest.%s has no class in bridgeRequestFields", bt.Field(i).Name)
		}
	}
	for name := range bridgeRequestFields {
		if _, ok := bt.FieldByName(name); !ok {
			t.Errorf("bridgeRequestFields lists %s, which BridgeRequest no longer has", name)
		}
	}
	pt := reflect.TypeOf(core.PhaseRequest{})
	for name := range staticPhaseFields {
		if _, ok := pt.FieldByName(name); !ok {
			t.Errorf("staticPhaseFields lists %s, which PhaseRequest no longer has", name)
		}
	}
	for name := range unfillablePhaseFields {
		if _, ok := pt.FieldByName(name); !ok {
			t.Errorf("unfillablePhaseFields lists %s, which PhaseRequest no longer has", name)
		}
	}
	root := repoRoot(t)
	fillRequest(t, root, "A", 1, "")
}

func TestBoundaryLiteral_SingleSource(t *testing.T) {
	root := repoRoot(t)
	var offenders []string
	err := filepath.WalkDir(filepath.Join(root, "go"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		if strings.Contains(p, string(filepath.Separator)+"promptboundary"+string(filepath.Separator)) {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(data), "## Cycle Context") {
			offenders = append(offenders, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("the boundary heading is spelled outside promptboundary: %v", offenders)
	}
}
