package gaterun

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gotool"
	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

type realLane struct{}

func (realLane) TrackedFiles(root, relDir string) ([]string, error) {
	return repostate.TrackedFiles(root, relDir)
}

func (realLane) TrackedTree(root, relDir string) ([]string, error) {
	return repostate.TrackedTree(root, relDir)
}

func (realLane) TrackedSet(root, relDir, ext string) (map[string]bool, error) {
	return repostate.TrackedSet(root, relDir, ext)
}

type realToolchain struct{}

func (realToolchain) List(ctx context.Context, call gatecontract.Call) (gatecontract.Output, error) {
	return capture(gotool.List(ctx, gotool.Call(call)))
}

func (realToolchain) Test(ctx context.Context, call gatecontract.Call) (gatecontract.Output, error) {
	return capture(gotool.Test(ctx, gotool.Call(call)))
}

func capture(cmd *exec.Cmd) (gatecontract.Output, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return gatecontract.Output{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: exit.ExitCode()}, nil
	}
	if err != nil {
		return gatecontract.Output{}, err
	}
	return gatecontract.Output{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, nil
}

func RealPorts() gatecontract.Ports {
	return gatecontract.Ports{Lane: realLane{}, Toolchain: realToolchain{}}
}

type conn struct {
	mu   sync.Mutex
	r    *bufio.Reader
	w    io.Writer
	next int
}

func (c *conn) call(method string, params, result any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("encode the params of %s: %w", method, err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next++
	if err := WriteFrame(c.w, Frame{Kind: KindRequest, ID: c.next, Method: method, Params: raw}); err != nil {
		return err
	}
	resp, err := ReadFrame(c.r)
	switch {
	case err != nil:
		return fmt.Errorf("%s: no response: %w", method, err)
	case resp.Kind != KindResponse || resp.ID != c.next:
		return fmt.Errorf("%s: expected response %d, got a %s frame with id %d", method, c.next, resp.Kind, resp.ID)
	case resp.Message != "":
		return errors.New(resp.Message)
	}
	if err := json.Unmarshal(resp.Result, result); err != nil {
		return fmt.Errorf("decode the result of %s: %w", method, err)
	}
	return nil
}

type clientLane struct{ c *conn }

func (l clientLane) TrackedFiles(root, relDir string) ([]string, error) {
	var out []string
	err := l.c.call(MethodTrackedFiles, treeParams{Root: root, RelDir: relDir}, &out)
	return out, err
}

func (l clientLane) TrackedTree(root, relDir string) ([]string, error) {
	var out []string
	err := l.c.call(MethodTrackedTree, treeParams{Root: root, RelDir: relDir}, &out)
	return out, err
}

func (l clientLane) TrackedSet(root, relDir, ext string) (map[string]bool, error) {
	var out map[string]bool
	err := l.c.call(MethodTrackedSet, setParams{Root: root, RelDir: relDir, Ext: ext}, &out)
	return out, err
}

type clientToolchain struct{ c *conn }

func (t clientToolchain) List(_ context.Context, call gatecontract.Call) (gatecontract.Output, error) {
	var out gatecontract.Output
	err := t.c.call(MethodList, toolParams{Call: call}, &out)
	return out, err
}

func (t clientToolchain) Test(_ context.Context, call gatecontract.Call) (gatecontract.Output, error) {
	var out gatecontract.Output
	err := t.c.call(MethodTest, toolParams{Call: call}, &out)
	return out, err
}

func clientPorts(r *bufio.Reader, w io.Writer) gatecontract.Ports {
	c := &conn{r: r, w: w}
	return gatecontract.Ports{Lane: clientLane{c}, Toolchain: clientToolchain{c}}
}
