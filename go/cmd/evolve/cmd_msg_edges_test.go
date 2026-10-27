package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/outbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
)

type msgArmFailWaker struct {
	msgFakeWaker
	closed bool
}

func (w *msgArmFailWaker) Arm(wake.Targets) error { return errors.New("arm refused") }

func (w *msgArmFailWaker) Close() error {
	w.closed = true
	return nil
}

type msgEdgeRun struct {
	code   int
	out    string
	errOut string
	layout outbox.Layout
}

func msgEdge(t *testing.T, mutate func(*msgEnv, outbox.Layout), stdin string, stdout io.Writer, args ...string) msgEdgeRun {
	t.Helper()
	layout := outbox.Layout{Root: t.TempDir(), Cycle: msgTestCycle}
	env := msgEnvFor(layout, msgScenario{waker: &msgFakeWaker{}})
	if mutate != nil {
		mutate(&env, layout)
	}
	var out, errOut bytes.Buffer
	if stdout == nil {
		stdout = &out
	}
	code := runMsgWith(env, args, strings.NewReader(stdin), stdout, &errOut)
	return msgEdgeRun{code: code, out: out.String(), errOut: errOut.String(), layout: layout}
}

func msgSeedFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMsgSend_ReportsTheReceiptOfTheService(t *testing.T) {
	t.Parallel()
	sendArgs := []string{"send", "--to", "role:build", "--kind", "note", "--id", msgTestID}
	withJSON := append([]string{"send", "--json"}, sendArgs[1:]...)
	okReceipt := `{"Receipt":{"ID":"` + msgTestID + `","Cursor":3}}`
	rows := []struct {
		name    string
		receipt string
		args    []string
		stdout  io.Writer
		want    int
		stderr  string
		out     string
	}{
		{"denied", `{"Error":"broker: denied"}`, sendArgs, nil, 1, "broker: denied", ""},
		{"not found", `{"Error":"broker: not found: unknown recipient"}`, sendArgs, nil, 4, "unknown recipient", ""},
		{"partial refusal", `{"Receipt":{"ID":"x","Refused":[{"Rule":"queue_cap","Reason":"full"}]}}`, sendArgs, nil, 0, "refused by queue_cap: full", "sent " + msgTestID},
		{"neither receipt nor error", `{}`, sendArgs, nil, 2, "neither a receipt nor an error", ""},
		{"malformed receipt", `not json`, sendArgs, nil, 2, "invalid character", ""},
		{"text hangup", okReceipt, sendArgs, msgBrokenWriter{err: syscall.EPIPE}, 3, "", ""},
		{"text write failure", okReceipt, sendArgs, msgBrokenWriter{err: errors.New("disk full")}, 2, "disk full", ""},
		{"json receipt", okReceipt, withJSON, nil, 0, "", `"Cursor":3`},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			seed := func(_ *msgEnv, layout outbox.Layout) {
				msgSeedFile(t, filepath.Join(layout.BoxDir(msgTestDispatch), "receipts", msgTestID+".json"), row.receipt)
			}
			got := msgEdge(t, seed, "", row.stdout, row.args...)
			if got.code != row.want {
				t.Fatalf("exit %d, want %d: %s", got.code, row.want, got.errOut)
			}
			if !strings.Contains(got.errOut, row.stderr) || !strings.Contains(got.out, row.out) {
				t.Errorf("stdout %q stderr %q, want %q and %q", got.out, got.errOut, row.out, row.stderr)
			}
		})
	}
}

func TestMsgSend_FailsLoudlyOnEachLocalFault(t *testing.T) {
	t.Parallel()
	sendArgs := []string{"send", "--to", "role:build", "--kind", "note"}
	t.Run("stdin cannot be read", func(t *testing.T) {
		layout := outbox.Layout{Root: t.TempDir(), Cycle: msgTestCycle}
		env := msgEnvFor(layout, msgScenario{waker: &msgFakeWaker{}})
		var errOut bytes.Buffer
		code := runMsgWith(env, sendArgs, iotest.ErrReader(errors.New("stdin gone")), io.Discard, &errOut)
		if code != 2 || !strings.Contains(errOut.String(), "stdin gone") {
			t.Fatalf("exit %d: %s", code, errOut.String())
		}
	})
	t.Run("entropy fails while minting the id", func(t *testing.T) {
		got := msgEdge(t, func(env *msgEnv, _ outbox.Layout) { env.entropy = iotest.ErrReader(errors.New("no entropy")) }, "", nil, sendArgs...)
		if got.code != 2 || !strings.Contains(got.errOut, "no entropy") {
			t.Fatalf("exit %d: %s", got.code, got.errOut)
		}
	})
	t.Run("the draft cannot be written", func(t *testing.T) {
		block := func(_ *msgEnv, layout outbox.Layout) { msgSeedFile(t, layout.OutDir(msgTestDispatch), "x") }
		got := msgEdge(t, block, "", nil, append(sendArgs, "--id", msgTestID)...)
		if got.code != 2 {
			t.Fatalf("exit %d: %s", got.code, got.errOut)
		}
	})
	t.Run("the kernel refuses the watch", func(t *testing.T) {
		waker := &msgArmFailWaker{}
		got := msgEdge(t, func(env *msgEnv, _ outbox.Layout) {
			env.waker = func() (msgWaker, error) { return waker, nil }
		}, "", nil, sendArgs...)
		if got.code != 2 || !strings.Contains(got.errOut, "arm refused") || !waker.closed {
			t.Fatalf("exit %d closed=%v: %s", got.code, waker.closed, got.errOut)
		}
	})
	t.Run("the reader of stdout is gone while waiting for the receipt", func(t *testing.T) {
		got := msgEdge(t, func(env *msgEnv, _ outbox.Layout) {
			env.waker = func() (msgWaker, error) { return &msgFakeWaker{result: wake.Wake{Hangup: true}}, nil }
		}, "", nil, sendArgs...)
		if got.code != 3 {
			t.Fatalf("exit %d, want 3: %s", got.code, got.errOut)
		}
	})
}

func TestMsgWait_FailsLoudlyOnEachLocalFault(t *testing.T) {
	t.Parallel()
	t.Run("the caller has no identity", func(t *testing.T) {
		got := msgEdge(t, func(env *msgEnv, _ outbox.Layout) {
			env.caller = func() (msgCaller, error) { return msgCaller{}, errors.New("no token") }
		}, "", nil, "wait")
		if got.code != 1 || !strings.Contains(got.errOut, "no token") {
			t.Fatalf("exit %d: %s", got.code, got.errOut)
		}
	})
	t.Run("the mailbox cannot be read", func(t *testing.T) {
		block := func(_ *msgEnv, layout outbox.Layout) {
			if err := os.MkdirAll(filepath.Join(layout.BoxDir(msgTestDispatch), msgMailboxFile), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if got := msgEdge(t, block, "", nil, "wait"); got.code != 2 {
			t.Fatalf("exit %d: %s", got.code, got.errOut)
		}
	})
	t.Run("the mailbox is malformed", func(t *testing.T) {
		seed := func(_ *msgEnv, layout outbox.Layout) {
			msgSeedFile(t, filepath.Join(layout.BoxDir(msgTestDispatch), msgMailboxFile), "{broken\n")
		}
		got := msgEdge(t, seed, "", nil, "wait")
		if got.code != 2 || !strings.Contains(got.errOut, "the mailbox is malformed") {
			t.Fatalf("exit %d: %s", got.code, got.errOut)
		}
	})
	t.Run("a failed write on a text print", func(t *testing.T) {
		seed := func(_ *msgEnv, layout outbox.Layout) {
			msgWriteMailbox(t, layout, []envelope.Envelope{msgMessage(msgTestID, "")})
		}
		got := msgEdge(t, seed, "", msgBrokenWriter{err: errors.New("disk full")}, "wait")
		if got.code != 2 || !strings.Contains(got.errOut, "disk full") {
			t.Fatalf("exit %d: %s", got.code, got.errOut)
		}
	})
}

func TestMsgReply_FailsLoudlyOnEachLocalFault(t *testing.T) {
	t.Parallel()
	t.Run("the caller has no identity", func(t *testing.T) {
		got := msgEdge(t, func(env *msgEnv, _ outbox.Layout) {
			env.caller = func() (msgCaller, error) { return msgCaller{}, errors.New("no token") }
		}, "x", nil, "reply", msgTestID)
		if got.code != 1 {
			t.Fatalf("exit %d: %s", got.code, got.errOut)
		}
	})
	t.Run("the mailbox is malformed", func(t *testing.T) {
		seed := func(_ *msgEnv, layout outbox.Layout) {
			msgSeedFile(t, filepath.Join(layout.BoxDir(msgTestDispatch), msgMailboxFile), "{broken\n")
		}
		if got := msgEdge(t, seed, "x", nil, "reply", msgTestID); got.code != 2 {
			t.Fatalf("exit %d: %s", got.code, got.errOut)
		}
	})
}

func TestMsgRefs_CollectsEachFlag(t *testing.T) {
	t.Parallel()
	var refs msgRefs
	for _, v := range []string{"a", "b"} {
		if err := refs.Set(v); err != nil {
			t.Fatal(err)
		}
	}
	if refs.String() != "a,b" {
		t.Errorf("refs %q, want a,b", refs.String())
	}
}

func TestMsgCallerFromEnv_ReadsTheDispatchTheTokenAndTheCycle(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", "")
	t.Setenv("EVOLVE_DISPATCH_ID", "")
	t.Setenv("EVOLVE_MSG_TOKEN", "")
	if _, err := msgCallerFromEnv(); err == nil {
		t.Fatal("a caller with no dispatch and no token must be refused")
	}
	t.Setenv("EVOLVE_DISPATCH_ID", msgTestDispatch)
	t.Setenv("EVOLVE_MSG_TOKEN", msgTestToken)
	if _, err := msgCallerFromEnv(); err == nil {
		t.Fatal("a missing cycle state must be refused")
	}
	state := filepath.Join(root, ".evolve", "cycle-state.json")
	msgSeedFile(t, state, `{"cycle_id": 0}`)
	if _, err := msgCallerFromEnv(); err == nil || !strings.Contains(err.Error(), "no cycle_id") {
		t.Fatalf("a state with no cycle must be refused, got %v", err)
	}
	msgSeedFile(t, state, `{"cycle_id": 2058}`)
	got, err := msgCallerFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	want := msgCaller{Layout: outbox.Layout{Root: root, Cycle: 2058}, Dispatch: msgTestDispatch, Token: msgTestToken}
	if got != want {
		t.Errorf("caller %+v, want %+v", got, want)
	}
}

func TestMsgWakerOnStdout_StartsTheKernelWaker(t *testing.T) {
	t.Parallel()
	w, err := msgWakerOnStdout()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
