package gaterun

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

func brokeredPorts(t *testing.T, ports gatecontract.Ports) gatecontract.Ports {
	t.Helper()
	childIn, parentOut := io.Pipe()
	parentIn, childOut := io.Pipe()
	broker := Broker{Root: "r", Ports: ports}
	go func() {
		reader := bufio.NewReader(parentIn)
		for {
			req, err := ReadFrame(reader)
			if err != nil {
				parentOut.Close()
				return
			}
			if WriteFrame(parentOut, broker.Handle(context.Background(), req)) != nil {
				return
			}
		}
	}()
	t.Cleanup(func() { childOut.Close(); parentIn.Close(); childIn.Close() })
	return clientPorts(bufio.NewReader(childIn), childOut)
}

func TestClientPorts_CallTheParentAndReturnItsAnswers(t *testing.T) {
	t.Parallel()
	ports := brokeredPorts(t, gatecontract.Ports{Lane: stubLane{}, Toolchain: stubToolchain{}})
	files, err := ports.Lane.TrackedFiles("r", "d")
	if err != nil || len(files) != 1 || files[0] != "files:r:d" {
		t.Errorf("TrackedFiles = %v, %v", files, err)
	}
	tree, err := ports.Lane.TrackedTree("r", "d")
	if err != nil || tree[0] != "tree:r:d" {
		t.Errorf("TrackedTree = %v, %v", tree, err)
	}
	set, err := ports.Lane.TrackedSet("r", "d", ".go")
	if err != nil || !set["set:r:d:.go"] {
		t.Errorf("TrackedSet = %v, %v", set, err)
	}
	listed, err := ports.Toolchain.List(context.Background(), gatecontract.Call{Dir: "r/x", Args: []string{"-json"}})
	if err != nil || string(listed.Stdout) != "list:r/x:-json" {
		t.Errorf("List = %+v, %v", listed, err)
	}
	tested, err := ports.Toolchain.Test(context.Background(), gatecontract.Call{Env: []string{"A=1"}})
	if err != nil || string(tested.Stderr) != "test:" || tested.ExitCode != 7 {
		t.Errorf("Test = %+v, %v", tested, err)
	}
}

func TestClientPorts_AnErrorResponseIsTheErrorOfTheCall(t *testing.T) {
	t.Parallel()
	ports := brokeredPorts(t, gatecontract.Ports{Lane: stubLane{err: errors.New("git failed")}, Toolchain: stubToolchain{err: errors.New("go failed")}})
	if _, err := ports.Lane.TrackedFiles("r", "d"); err == nil || !strings.Contains(err.Error(), "git failed") {
		t.Errorf("TrackedFiles = %v, want the parent error", err)
	}
	if _, err := ports.Toolchain.List(context.Background(), gatecontract.Call{}); err == nil || !strings.Contains(err.Error(), "go failed") {
		t.Errorf("List = %v, want the parent error", err)
	}
}

func TestConn_AnUnknownFunctionIsAnErrorOfTheChildCall(t *testing.T) {
	t.Parallel()
	childIn, parentOut := io.Pipe()
	parentIn, childOut := io.Pipe()
	go func() {
		reader := bufio.NewReader(parentIn)
		req, _ := ReadFrame(reader)
		_ = WriteFrame(parentOut, Broker{}.Handle(context.Background(), req))
	}()
	c := &conn{r: bufio.NewReader(childIn), w: childOut}
	var out []string
	if err := c.call("os.Remove", treeParams{}, &out); err == nil || !strings.Contains(err.Error(), "unknown function") {
		t.Errorf("call = %v, want an unknown-function error", err)
	}
}

func TestConn_AWrongFrameACrossedIDAClosedStreamAndABadResultAreErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"wrong kind": `{"kind":"error","message":"m"}` + "\n",
		"crossed id": `{"kind":"response","id":99,"result":[]}` + "\n",
		"closed":     "",
		"bad result": `{"kind":"response","id":1,"result":"text"}` + "\n",
		"malformed":  "junk\n",
	}
	for name, reply := range cases {
		c := &conn{r: bufio.NewReader(strings.NewReader(reply)), w: io.Discard}
		var out []string
		if err := c.call(MethodTrackedFiles, treeParams{}, &out); err == nil {
			t.Errorf("%s: call = nil, want an error", name)
		}
	}
	c := &conn{r: bufio.NewReader(strings.NewReader("")), w: failWriter{}}
	if err := c.call(MethodTrackedFiles, treeParams{}, new([]string)); err == nil {
		t.Error("a failed write: call = nil, want an error")
	}
	if err := (&conn{r: readerOf(""), w: io.Discard}).call(MethodTrackedFiles, make(chan int), nil); err == nil {
		t.Error("params that cannot be encoded: call = nil, want an error")
	}
}

func TestRealPorts_ReadTheLaneThroughRepostateAndRunTheToolchainThroughGotool(t *testing.T) {
	t.Parallel()
	ports := RealPorts()
	files, err := ports.Lane.TrackedFiles("../../..", "go/internal/gotool")
	if err != nil || len(files) == 0 {
		t.Errorf("TrackedFiles = %v, %v, want the tracked files of gotool", files, err)
	}
	tree, err := ports.Lane.TrackedTree("../../..", "go/internal/gotool")
	if err != nil || len(tree) == 0 {
		t.Errorf("TrackedTree = %v, %v", tree, err)
	}
	set, err := ports.Lane.TrackedSet("../../..", "go/internal/gotool", ".go")
	if err != nil || !set["gotool"] {
		t.Errorf("TrackedSet = %v, %v", set, err)
	}
	listed, err := ports.Toolchain.List(context.Background(), gatecontract.Call{Dir: "..", Args: []string{"-json", "./gotool"}})
	if err != nil || listed.ExitCode != 0 || !strings.Contains(string(listed.Stdout), "evolve-loop/go") {
		t.Errorf("List = %+v, %v, want the module path", listed, err)
	}
	tested, err := ports.Toolchain.Test(context.Background(), gatecontract.Call{Dir: "..", Args: []string{"./no/such/package"}})
	if err != nil || tested.ExitCode == 0 || len(tested.Stderr) == 0 {
		t.Errorf("Test over a missing package = %+v, %v, want a non-zero exit code and no error", tested, err)
	}
	if _, err := ports.Toolchain.List(context.Background(), gatecontract.Call{Dir: "/no/such/dir"}); err == nil {
		t.Error("List in a missing directory = nil, want a start error")
	}
}
