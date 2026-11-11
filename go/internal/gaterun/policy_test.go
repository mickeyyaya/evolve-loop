package gaterun

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

type recordingToolchain struct{ calls []gatecontract.Call }

func (r *recordingToolchain) List(_ context.Context, call gatecontract.Call) (gatecontract.Output, error) {
	r.calls = append(r.calls, call)
	return gatecontract.Output{}, nil
}

func (r *recordingToolchain) Test(_ context.Context, call gatecontract.Call) (gatecontract.Output, error) {
	r.calls = append(r.calls, call)
	return gatecontract.Output{}, nil
}

type recordingLane struct{ calls int }

func (r *recordingLane) TrackedFiles(string, string) ([]string, error) {
	r.calls++
	return nil, nil
}

func (r *recordingLane) TrackedTree(string, string) ([]string, error) {
	r.calls += 1
	return []string{}, nil
}

func (r *recordingLane) TrackedSet(string, string, string) (map[string]bool, error) {
	r.calls += 1
	return map[string]bool{}, nil
}

func pinned(tc gatecontract.Toolchain, lane gatecontract.Lane) Broker {
	return Broker{Root: "/repo", Scratch: "/scratch", Ports: gatecontract.Ports{Lane: lane, Toolchain: tc}}
}

func TestBroker_ARefusedToolCallStartsNoProcess(t *testing.T) {
	t.Parallel()
	refused := map[string]gatecontract.Call{
		"exec":               {Dir: "/repo", Args: []string{"-exec", "/bin/sh -c id", "./..."}},
		"exec equals":        {Dir: "/repo", Args: []string{"-exec=/bin/sh", "./..."}},
		"toolexec":           {Dir: "/repo", Args: []string{"-toolexec", "x", "./..."}},
		"o":                  {Dir: "/repo", Args: []string{"-o", "x", "./..."}},
		"c":                  {Dir: "/repo", Args: []string{"-c", "./..."}},
		"modfile":            {Dir: "/repo", Args: []string{"-modfile=x", "./..."}},
		"overlay":            {Dir: "/repo", Args: []string{"-overlay=x", "./..."}},
		"pkgdir":             {Dir: "/repo", Args: []string{"-pkgdir=x", "./..."}},
		"unknown flag":       {Dir: "/repo", Args: []string{"-bogus", "./..."}},
		"double dash":        {Dir: "/repo", Args: []string{"--run", "x", "./..."}},
		"args terminator":    {Dir: "/repo", Args: []string{"./...", "-args", "x"}},
		"absolute pkg":       {Dir: "/repo", Args: []string{"/etc"}},
		"dotdot pkg":         {Dir: "/repo", Args: []string{"../x/..."}},
		"dotdot in lane":     {Dir: "/repo", Args: []string{"./x/../../y"}},
		"module query":       {Dir: "/repo", Args: []string{"rsc.io/quote@latest"}},
		"module flag":        {Dir: "/repo", Args: []string{"-m", "rsc.io/quote@v1.5.2"}},
		"import path":        {Dir: "/repo", Args: []string{"example.com/evil"}},
		"all pattern":        {Dir: "/repo", Args: []string{"all"}},
		"version in lane":    {Dir: "/repo", Args: []string{"./x@v1"}},
		"profile is scratch": {Dir: "/repo", Args: []string{"-coverprofile=/scratch", "./..."}},
		"dotdot dir":         {Dir: "/repo/../etc", Args: []string{"./..."}},
		"foreign dir":        {Dir: "/etc", Args: []string{"./..."}},
		"missing value":      {Dir: "/repo", Args: []string{"-run"}},
		"profile outside":    {Dir: "/repo", Args: []string{"-coverprofile=/tmp/x", "./..."}},
		"profile relative":   {Dir: "/repo", Args: []string{"-coverprofile", "c.out", "./..."}},
		"profile escape":     {Dir: "/repo", Args: []string{"-coverprofile=/scratch/../x", "./..."}},
	}
	for name, call := range refused {
		for method, verb := range map[string]string{MethodList: "list", MethodTest: "test"} {
			if verb == "list" && strings.HasPrefix(name, "profile") {
				continue
			}
			tc := &recordingToolchain{}
			resp := pinned(tc, nil).Handle(context.Background(), request(t, method, toolParams{Call: call}))
			if resp.Message == "" || len(tc.calls) != 0 {
				t.Errorf("%s %s: Message = %q, runner calls = %d, want a refusal and no process", verb, name, resp.Message, len(tc.calls))
			}
		}
	}
}

func TestBroker_AnAllowedToolCallRunsWithTheEnvOfTheParentAndAPinnedDir(t *testing.T) {
	t.Parallel()
	good := map[string][]string{
		"test": {"-run", "TestX", "-count=1", "-json", "-cover", "-coverprofile", "/scratch/c.out", "-tags=a", "-timeout", "5m", "-race", "-p=2", "-parallel", "4", "-v", "-short", "./a/...", "."},
		"list": {"-json", "-deps", "-test", "-e", "-f", "{{.Dir}}", "-find", "-tags", "x", "."},
	}
	for verb, args := range good {
		method := map[string]string{"test": MethodTest, "list": MethodList}[verb]
		tc := &recordingToolchain{}
		call := gatecontract.Call{Dir: "", Env: []string{"GOFLAGS=-exec=/bin/sh"}, Args: args}
		resp := pinned(tc, nil).Handle(context.Background(), request(t, method, toolParams{Call: call}))
		if resp.Message != "" || len(tc.calls) != 1 {
			t.Fatalf("%s: Message = %q, runner calls = %d", verb, resp.Message, len(tc.calls))
		}
		got := tc.calls[0]
		if got.Dir != "/repo" || !slices.Equal(got.Args, args) {
			t.Errorf("%s: forwarded %+v, want dir /repo and the same args", verb, got)
		}
		for _, kv := range goOfflineEnv {
			if !slices.Contains(got.Env, kv) {
				t.Errorf("%s: the go env misses %s", verb, kv)
			}
		}
		if slices.Contains(got.Env, "GOFLAGS=-exec=/bin/sh") {
			t.Errorf("%s: the env of the child reached the go run", verb)
		}
	}
	tc := &recordingToolchain{}
	sub := gatecontract.Call{Dir: filepath.Join("/repo", "go"), Args: []string{"./..."}}
	if resp := pinned(tc, nil).Handle(context.Background(), request(t, MethodList, toolParams{Call: sub})); resp.Message != "" || tc.calls[0].Dir != "/repo/go" {
		t.Errorf("a dir inside the root: %+v, %+v", resp, tc.calls)
	}
}

func TestBroker_ALaneCallOutsideTheRootOrWithAnEscapingRelDirStartsNothing(t *testing.T) {
	t.Parallel()
	for name, p := range map[string]treeParams{
		"foreign root": {Root: "/etc", RelDir: "d"},
		"absolute rel": {Root: "/repo", RelDir: "/etc"},
		"dotdot rel":   {Root: "/repo", RelDir: "a/../../b"},
		"dotdot only":  {Root: "/repo", RelDir: ".."},
	} {
		for _, method := range []string{MethodTrackedFiles, MethodTrackedTree, MethodTrackedSet} {
			lane := &recordingLane{}
			resp := pinned(nil, lane).Handle(context.Background(), request(t, method, setParams{Root: p.Root, RelDir: p.RelDir, Ext: ".go"}))
			if resp.Message == "" || lane.calls != 0 {
				t.Errorf("%s %s: Message = %q, calls = %d, want a refusal", method, name, resp.Message, lane.calls)
			}
		}
	}
	lane := &recordingLane{}
	if resp := pinned(nil, lane).Handle(context.Background(), request(t, MethodTrackedFiles, treeParams{Root: "/repo", RelDir: "go/internal"})); resp.Message != "" || lane.calls != 1 {
		t.Errorf("a pinned lane call: %+v, calls %d", resp, lane.calls)
	}
}

func TestBroker_ANoScratchDirRefusesACoverProfile(t *testing.T) {
	t.Parallel()
	tc := &recordingToolchain{}
	b := Broker{Root: "/repo", Ports: gatecontract.Ports{Toolchain: tc}}
	resp := b.Handle(context.Background(), request(t, MethodTest, toolParams{Call: gatecontract.Call{Dir: "/repo", Args: []string{"-coverprofile=/x"}}}))
	if resp.Message == "" || len(tc.calls) != 0 {
		t.Errorf("Handle = %+v, calls %d", resp, len(tc.calls))
	}
}

func TestLockedGoEnv_TheOfflineValuesComeLastSoTheyWin(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOPROXY", "https://proxy.golang.org")
	last := map[string]string{}
	for _, kv := range lockedGoEnv() {
		k, v, _ := strings.Cut(kv, "=")
		last[k] = v
	}
	for _, kv := range goOfflineEnv {
		k, v, _ := strings.Cut(kv, "=")
		if last[k] != v {
			t.Errorf("the last %s is %q, want %q: os/exec keeps the last value, so the offline value must come after the parent", k, last[k], v)
		}
	}
}
