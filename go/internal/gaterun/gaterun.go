package gaterun

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Prefix func(*exec.Cmd) *exec.Cmd

type Options struct {
	Command func(ctx context.Context, entry string) *exec.Cmd
	Ports   gatecontract.Ports

	Strategy Strategy

	ScratchParent string
}

func EvolveCommand(binary string) func(context.Context, string) *exec.Cmd {
	return func(ctx context.Context, entry string) *exec.Cmd {
		return sysexec.Command(ctx, binary, "gate", "run", "--entry", entry)
	}
}

func Run(ctx context.Context, entry gatecontract.Entry, input gatecontract.Input, opts Options) (gatecontract.Verdict, error) {
	if opts.Command == nil {
		return gatecontract.Verdict{}, fmt.Errorf("gate %s: no command to start the child", entry.Name)
	}
	scratch, err := newScratch(opts.ScratchParent, canaries)
	if err != nil {
		return gatecontract.Verdict{}, fmt.Errorf("gate %s: %w", entry.Name, err)
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	target := Target{Entry: entry.Name, Lane: input.Root, Scratch: scratch}
	var prefix Prefix
	if opts.Strategy != nil {
		prefix = opts.Strategy.Prefix(target)
	}
	cmd := opts.Command(ctx, entry.Name)
	if prefix != nil {
		cmd = prefix(cmd)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return gatecontract.Verdict{}, fmt.Errorf("gate %s: %w", entry.Name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return gatecontract.Verdict{}, fmt.Errorf("gate %s: %w", entry.Name, err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	started := time.Now()
	if err := cmd.Start(); err != nil {
		return gatecontract.Verdict{}, fmt.Errorf("gate %s: start the child: %w", entry.Name, err)
	}
	verdict, converseErr := converse(ctx, Broker{Root: input.Root, Scratch: scratch, Ports: opts.Ports}, input, bufio.NewReader(stdout), stdin)
	_ = stdin.Close()
	if converseErr != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if finisher, ok := opts.Strategy.(Finisher); ok {
		finisher.Finish(ctx, target, started)
	}
	switch {
	case converseErr != nil:
		return gatecontract.Verdict{}, childError(entry.Name, converseErr, &stderr)
	case waitErr != nil:
		return gatecontract.Verdict{}, childError(entry.Name, waitErr, &stderr)
	}
	return verdict, nil
}

func childError(name string, err error, stderr *bytes.Buffer) error {
	if text := strings.TrimSpace(stderr.String()); text != "" {
		return fmt.Errorf("gate %s: %w: %s", name, err, text)
	}
	return fmt.Errorf("gate %s: %w", name, err)
}

func converse(ctx context.Context, broker Broker, input gatecontract.Input, r *bufio.Reader, w io.Writer) (gatecontract.Verdict, error) {
	if err := WriteFrame(w, Frame{Kind: KindInput, Input: &input}); err != nil {
		return gatecontract.Verdict{}, fmt.Errorf("the child ended its stream with no verdict: %w", err)
	}
	for {
		frame, err := ReadFrame(r)
		if errors.Is(err, io.EOF) {
			return gatecontract.Verdict{}, errors.New("the child ended its stream with no verdict")
		}
		if err != nil {
			return gatecontract.Verdict{}, err
		}
		switch frame.Kind {
		case KindVerdict:
			return *frame.Verdict, nil
		case KindError:
			return gatecontract.Verdict{}, errors.New(frame.Message)
		case KindRequest:
			if err := WriteFrame(w, broker.Handle(ctx, frame)); err != nil {
				return gatecontract.Verdict{}, err
			}
		default:
			return gatecontract.Verdict{}, fmt.Errorf("unexpected %s frame from the child", frame.Kind)
		}
	}
}

func Serve(ctx context.Context, entry gatecontract.Entry, stdin io.Reader, stdout io.Writer) error {
	return serveWith(ctx, entry, stdin, stdout, readCanary)
}

func serveWith(ctx context.Context, entry gatecontract.Entry, stdin io.Reader, stdout io.Writer, canary func(name string)) error {
	canary(sandbox.CanaryStart)
	verdict, err := serve(ctx, entry, bufio.NewReader(stdin), stdout)
	canary(sandbox.CanaryEnd)
	if err != nil {
		_ = WriteFrame(stdout, Frame{Kind: KindError, Message: err.Error()})
		return err
	}
	return WriteFrame(stdout, Frame{Kind: KindVerdict, Verdict: &verdict})
}

func serve(ctx context.Context, entry gatecontract.Entry, r *bufio.Reader, w io.Writer) (gatecontract.Verdict, error) {
	frame, err := ReadFrame(r)
	if err != nil {
		return gatecontract.Verdict{}, fmt.Errorf("read the input frame: %w", err)
	}
	if frame.Kind != KindInput {
		return gatecontract.Verdict{}, fmt.Errorf("expected an input frame, got a %s frame", frame.Kind)
	}
	return entry.Run(ctx, *frame.Input, clientPorts(r, w))
}

var canaries = []string{sandbox.CanaryStart, sandbox.CanaryEnd}

func newScratch(parent string, names []string) (string, error) {
	scratch, err := os.MkdirTemp(parent, "gaterun-")
	if err != nil {
		return "", err
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(scratch, name), nil, 0o600); err != nil {
			_ = os.RemoveAll(scratch)
			return "", fmt.Errorf("place the canary %s: %w", name, err)
		}
	}
	return scratch, nil
}

func readCanary(name string) {
	_, _ = os.ReadFile(filepath.Join(os.TempDir(), name))
}
