package gaterun

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

type stubLane struct{ err error }

func (s stubLane) TrackedFiles(root, relDir string) ([]string, error) {
	return []string{"files:" + root + ":" + relDir}, s.err
}

func (s stubLane) TrackedTree(root, relDir string) ([]string, error) {
	return []string{"tree:" + root + ":" + relDir}, s.err
}

func (s stubLane) TrackedSet(root, relDir, ext string) (map[string]bool, error) {
	return map[string]bool{"set:" + root + ":" + relDir + ":" + ext: true}, s.err
}

type stubToolchain struct{ err error }

func (s stubToolchain) List(_ context.Context, call gatecontract.Call) (gatecontract.Output, error) {
	return gatecontract.Output{Stdout: []byte("list:" + call.Dir + ":" + strings.Join(call.Args, ",")), ExitCode: 0}, s.err
}

func (s stubToolchain) Test(_ context.Context, call gatecontract.Call) (gatecontract.Output, error) {
	var leaked []string
	for _, kv := range call.Env {
		if strings.HasPrefix(kv, "A=") || strings.HasPrefix(kv, "E=") {
			leaked = append(leaked, kv)
		}
	}
	return gatecontract.Output{Stderr: []byte("test:" + strings.Join(leaked, ",")), ExitCode: 7}, s.err
}

func request(t *testing.T, method string, params any) Frame {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return Frame{Kind: KindRequest, ID: 9, Method: method, Params: raw}
}

func answer[R any](t *testing.T, broker Broker, method string, params any) R {
	t.Helper()
	resp := broker.Handle(context.Background(), request(t, method, params))
	if resp.Kind != KindResponse || resp.ID != 9 || resp.Message != "" {
		t.Fatalf("%s: Handle = %+v", method, resp)
	}
	var out R
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBroker_ServesEachFunctionOfTheTwoInterfaces(t *testing.T) {
	t.Parallel()
	broker := Broker{Root: "r", Ports: gatecontract.Ports{Lane: stubLane{}, Toolchain: stubToolchain{}}}
	tree := treeParams{Root: "r", RelDir: "d"}
	if got := answer[[]string](t, broker, MethodTrackedFiles, tree); len(got) != 1 || got[0] != "files:r:d" {
		t.Errorf("TrackedFiles = %v", got)
	}
	if got := answer[[]string](t, broker, MethodTrackedTree, tree); len(got) != 1 || got[0] != "tree:r:d" {
		t.Errorf("TrackedTree = %v", got)
	}
	if got := answer[map[string]bool](t, broker, MethodTrackedSet, setParams{Root: "r", RelDir: "d", Ext: ".go"}); !got["set:r:d:.go"] {
		t.Errorf("TrackedSet = %v", got)
	}
	listed := answer[gatecontract.Output](t, broker, MethodList, toolParams{Call: gatecontract.Call{Dir: "r/x", Args: []string{"./a", "./b"}}})
	if string(listed.Stdout) != "list:r/x:./a,./b" {
		t.Errorf("List = %+v", listed)
	}
	tested := answer[gatecontract.Output](t, broker, MethodTest, toolParams{Call: gatecontract.Call{Env: []string{"E=1"}}})
	if string(tested.Stderr) != "test:" || tested.ExitCode != 7 {
		t.Errorf("Test = %+v", tested)
	}
}

func TestBroker_AnUnknownFunctionIsAnErrorResponse(t *testing.T) {
	t.Parallel()
	broker := Broker{Ports: gatecontract.Ports{Lane: stubLane{}, Toolchain: stubToolchain{}}}
	for _, method := range []string{"Lane.Bogus", "os.Remove", "", "Toolchain.Run"} {
		resp := broker.Handle(context.Background(), request(t, method, treeParams{}))
		if resp.Kind != KindResponse || resp.ID != 9 || !strings.Contains(resp.Message, "unknown function") {
			t.Errorf("method %q: Handle = %+v, want an unknown-function error", method, resp)
		}
	}
}

func TestBroker_AFailedCallBadParamsAndMissingPortsAreErrorResponses(t *testing.T) {
	t.Parallel()
	cause := errors.New("git failed")
	failing := Broker{Ports: gatecontract.Ports{Lane: stubLane{err: cause}, Toolchain: stubToolchain{err: cause}}}
	for _, method := range []string{MethodTrackedFiles, MethodTrackedTree, MethodTrackedSet, MethodList, MethodTest} {
		resp := failing.Handle(context.Background(), request(t, method, map[string]any{}))
		if resp.Message != "git failed" {
			t.Errorf("%s: Message = %q, want the cause", method, resp.Message)
		}
		bad := failing.Handle(context.Background(), Frame{Kind: KindRequest, ID: 1, Method: method, Params: []byte(`[1]`)})
		if !strings.Contains(bad.Message, "bad params") {
			t.Errorf("%s: bad params Message = %q", method, bad.Message)
		}
		none := Broker{}.Handle(context.Background(), request(t, method, map[string]any{}))
		if !strings.Contains(none.Message, "no port") {
			t.Errorf("%s: missing port Message = %q", method, none.Message)
		}
	}
}

func TestBroker_AResultThatCannotBeEncodedIsAnErrorResponse(t *testing.T) {
	t.Parallel()
	resp := encodeResponse(Frame{ID: 4}, make(chan int), nil)
	if resp.ID != 4 || resp.Message == "" {
		t.Errorf("encodeResponse = %+v, want an error response", resp)
	}
}
