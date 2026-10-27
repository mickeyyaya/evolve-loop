package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/address"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/outbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const (
	msgUsage = `usage: evolve msg send --to ADDR[,ADDR...] --kind KIND [--subject S] [--ref PATH]... [--reply-to ID] [--correlation ID] [--expires RFC3339] [--id ULID] [--json]   (the body is stdin)
       evolve msg wait [--deadline DURATION] [--json]
       evolve msg reply ID [--kind reply|evidence] [--json]   (the body is stdin)
exit: 0 ok, 1 refused, 2 I/O error or no receipt, 3 stdout reader gone, 4 not found, 5 wait deadline, 10 usage`
	msgPrefix       = "evolve msg: "
	msgReceiptWait  = 30 * time.Second
	exitMsgHangup   = 3
	exitMsgNotFound = 4
	exitMsgDeadline = 5
	msgMailboxFile  = "messages.ndjson"
	msgReceiptsDir  = "receipts"
	msgDraftSend    = "send"
)

type msgWaker interface {
	Arm(t wake.Targets) error
	Wait(ctx context.Context, deadline time.Time) (wake.Wake, error)
	Close() error
}

type msgCaller struct {
	Layout   outbox.Layout
	Dispatch string
	Token    string
}

type msgEnv struct {
	caller      func() (msgCaller, error)
	waker       func() (msgWaker, error)
	now         func() time.Time
	entropy     io.Reader
	receiptWait time.Duration
}

type msgStreams struct {
	stdin          io.Reader
	stdout, stderr io.Writer
}

type msgDraftFile struct {
	Type     string            `json:"type"`
	Token    string            `json:"token"`
	Envelope envelope.Envelope `json:"envelope"`
}

type msgRefs []string

func (r *msgRefs) String() string { return strings.Join(*r, ",") }

func (r *msgRefs) Set(v string) error {
	*r = append(*r, v)
	return nil
}

func runMsg(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runMsgWith(msgProductionEnv(), args, stdin, stdout, stderr)
}

func msgProductionEnv() msgEnv {
	return msgEnv{caller: msgCallerFromEnv, waker: msgWakerOnStdout, now: time.Now, entropy: rand.Reader, receiptWait: msgReceiptWait}
}

func msgWakerOnStdout() (msgWaker, error) {
	return wake.New(os.Stdout)
}

func msgCallerFromEnv() (msgCaller, error) {
	dispatch, token := os.Getenv(ipcenv.DispatchIDKey), os.Getenv(ipcenv.MsgTokenKey)
	if dispatch == "" || token == "" {
		return msgCaller{}, fmt.Errorf("%s and %s must both be set", ipcenv.DispatchIDKey, ipcenv.MsgTokenKey)
	}
	layout := paths.ResolveFromEnv()
	cycle, err := msgCycleOf(layout.CycleStateFile)
	if err != nil {
		return msgCaller{}, err
	}
	return msgCaller{Layout: outbox.Layout{Root: layout.ProjectRoot, Cycle: cycle}, Dispatch: dispatch, Token: token}, nil
}

func msgCycleOf(stateFile string) (int, error) {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return 0, fmt.Errorf("read the cycle state: %w", err)
	}
	var state struct {
		CycleID int `json:"cycle_id"`
	}
	if err := json.Unmarshal(data, &state); err != nil || state.CycleID <= 0 {
		return 0, fmt.Errorf("%s holds no cycle_id", stateFile)
	}
	return state.CycleID, nil
}

func runMsgWith(env msgEnv, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return msgUsageError(stderr, "a sub-verb is required")
	}
	verbs := map[string]func(msgEnv, []string, msgStreams) int{"send": msgSend, "wait": msgWait, "reply": msgReply}
	verb, ok := verbs[args[0]]
	if !ok {
		return msgUsageError(stderr, fmt.Sprintf("unknown sub-verb %q", args[0]))
	}
	return verb(env, args[1:], msgStreams{stdin: stdin, stdout: stdout, stderr: stderr})
}

func msgUsageError(stderr io.Writer, text string) int {
	fmt.Fprintf(stderr, "%s%s\n%s\n", msgPrefix, text, msgUsage)
	return exitUsage
}

func msgFail(stderr io.Writer, code int, err error) int {
	fmt.Fprintf(stderr, "%s%v\n", msgPrefix, err)
	return code
}

func msgParse(set *flag.FlagSet, args []string, positional int) ([]string, error) {
	set.SetOutput(io.Discard)
	pos, err := cmdutil.ParseInterspersed(set, args)
	if err == nil && len(pos) != positional {
		err = fmt.Errorf("want %d positional arguments, got %d", positional, len(pos))
	}
	return pos, err
}

func msgSend(env msgEnv, args []string, s msgStreams) int {
	set := flag.NewFlagSet("send", flag.ContinueOnError)
	var to, kind, subject, replyTo, correlation, expires, id string
	var refs msgRefs
	var asJSON bool
	set.StringVar(&to, "to", "", "")
	set.StringVar(&kind, "kind", "", "")
	set.StringVar(&subject, "subject", "", "")
	set.Var(&refs, "ref", "")
	set.StringVar(&replyTo, "reply-to", "", "")
	set.StringVar(&correlation, "correlation", "", "")
	set.StringVar(&expires, "expires", "", "")
	set.StringVar(&id, "id", "", "")
	set.BoolVar(&asJSON, "json", false, "")
	if _, err := msgParse(set, args, 0); err != nil {
		return msgUsageError(s.stderr, err.Error())
	}
	draft := envelope.Envelope{Kind: envelope.Kind(kind), Subject: subject, Refs: refs, ReplyTo: replyTo, CorrelationID: correlation, ID: id}
	if err := msgFillDraft(&draft, to, expires); err != nil {
		return msgUsageError(s.stderr, err.Error())
	}
	caller, err := env.caller()
	if err != nil {
		return msgFail(s.stderr, exitRefused, err)
	}
	return msgDeliver(env, caller, draft, s, asJSON)
}

func msgFillDraft(draft *envelope.Envelope, to, expires string) error {
	for _, part := range strings.Split(to, ",") {
		a, err := address.Parse(part)
		if err != nil {
			return fmt.Errorf("--to: %w", err)
		}
		draft.To = append(draft.To, a)
	}
	if draft.Kind == "" {
		return errors.New("--kind is required")
	}
	if draft.ID != "" && !envelope.ValidID(draft.ID) {
		return fmt.Errorf("--id %q is not a ULID", draft.ID)
	}
	if expires != "" {
		at, err := time.Parse(time.RFC3339, expires)
		if err != nil {
			return fmt.Errorf("--expires: %w", err)
		}
		draft.ExpiresAt = &at
	}
	return nil
}

func msgReply(env msgEnv, args []string, s msgStreams) int {
	set := flag.NewFlagSet("reply", flag.ContinueOnError)
	var kind string
	var asJSON bool
	set.StringVar(&kind, "kind", string(envelope.KindReply), "")
	set.BoolVar(&asJSON, "json", false, "")
	pos, err := msgParse(set, args, 1)
	if err != nil {
		return msgUsageError(s.stderr, err.Error())
	}
	target := pos[0]
	replyKind := envelope.Kind(kind)
	if !envelope.ValidID(target) || (replyKind != envelope.KindReply && replyKind != envelope.KindEvidence) {
		return msgUsageError(s.stderr, "reply wants a ULID and a kind of reply or evidence")
	}
	caller, err := env.caller()
	if err != nil {
		return msgFail(s.stderr, exitRefused, err)
	}
	mailbox, err := msgReadMailbox(caller)
	if err != nil {
		return msgFail(s.stderr, exitIO, err)
	}
	at := slices.IndexFunc(mailbox, func(m envelope.Envelope) bool { return m.ID == target })
	if at < 0 {
		return msgFail(s.stderr, exitMsgNotFound, fmt.Errorf("%s is not in the mailbox of this dispatch", target))
	}
	request := mailbox[at]
	draft := envelope.Envelope{
		Kind:          replyKind,
		To:            []address.Address{{Selector: address.SelectDispatch, Value: request.From.Dispatch}},
		ReplyTo:       target,
		CorrelationID: cmp.Or(request.CorrelationID, target),
	}
	return msgDeliver(env, caller, draft, s, asJSON)
}

func msgDeliver(env msgEnv, caller msgCaller, draft envelope.Envelope, s msgStreams, asJSON bool) int {
	body, err := io.ReadAll(s.stdin)
	if err != nil {
		return msgFail(s.stderr, exitIO, err)
	}
	draft.Body = string(body)
	if draft.ID == "" {
		draft.ID, err = envelope.IDSource{Now: env.now, Entropy: env.entropy}.New()
		if err != nil {
			return msgFail(s.stderr, exitIO, err)
		}
	}
	receipt := filepath.Join(caller.Layout.BoxDir(caller.Dispatch), msgReceiptsDir, draft.ID+".json")
	waker, err := msgOpenWaker(env, filepath.Dir(receipt))
	if err != nil {
		return msgFail(s.stderr, exitIO, err)
	}
	defer func() { _ = waker.Close() }()
	deadline := env.now().Add(env.receiptWait)
	file := filepath.Join(caller.Layout.OutDir(caller.Dispatch), draft.ID+".json")
	if err := atomicwrite.JSON(file, msgDraftFile{Type: msgDraftSend, Token: caller.Token, Envelope: draft}); err != nil {
		return msgFail(s.stderr, exitIO, err)
	}
	answer, code := msgAwaitReceipt(waker, receipt, deadline, s.stderr)
	if code != 0 {
		return code
	}
	return msgReport(answer, draft.ID, s, asJSON)
}

func msgReport(answer outbox.Answer, id string, s msgStreams, asJSON bool) int {
	if answer.Error != "" {
		code := exitRefused
		if strings.Contains(answer.Error, broker.ErrNotFound.Error()) {
			code = exitMsgNotFound
		}
		return msgFail(s.stderr, code, errors.New(answer.Error))
	}
	if answer.Receipt == nil {
		return msgFail(s.stderr, exitIO, errors.New("the receipt holds neither a receipt nor an error"))
	}
	for _, r := range answer.Receipt.Refused {
		fmt.Fprintf(s.stderr, "%srefused by %s: %s\n", msgPrefix, r.Rule, r.Reason)
	}
	if asJSON {
		return msgWriteExit(s.stderr, json.NewEncoder(s.stdout).Encode(answer.Receipt))
	}
	_, err := fmt.Fprintf(s.stdout, "sent %s\n", id)
	return msgWriteExit(s.stderr, err)
}

func msgWriteExit(stderr io.Writer, err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, syscall.EPIPE):
		return exitMsgHangup
	}
	return msgFail(stderr, exitIO, err)
}

func msgOpenWaker(env msgEnv, dir string) (msgWaker, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	w, err := env.waker()
	if err != nil {
		return nil, err
	}
	if err := w.Arm(wake.Targets{Dirs: []string{dir}}); err != nil {
		return nil, errors.Join(err, w.Close())
	}
	return w, nil
}

func msgBlock(w msgWaker, deadline time.Time, deadlineCode int, deadlineWhy string, stderr io.Writer) (int, bool) {
	woke, err := w.Wait(context.Background(), deadline)
	switch {
	case err != nil:
		return msgFail(stderr, exitIO, err), true
	case woke.Hangup:
		return exitMsgHangup, true
	case woke.Deadline:
		return msgFail(stderr, deadlineCode, errors.New(deadlineWhy)), true
	}
	return 0, false
}

func msgAwaitReceipt(w msgWaker, path string, deadline time.Time, stderr io.Writer) (outbox.Answer, int) {
	for {
		answer, found, err := msgReadReceipt(path)
		if err != nil {
			return outbox.Answer{}, msgFail(stderr, exitIO, err)
		}
		if found {
			return answer, 0
		}
		if code, stop := msgBlock(w, deadline, exitIO, "no receipt by the deadline", stderr); stop {
			return outbox.Answer{}, code
		}
	}
}

func msgReadReceipt(path string) (outbox.Answer, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return outbox.Answer{}, false, nil
	}
	var answer outbox.Answer
	if err == nil {
		err = json.Unmarshal(data, &answer)
	}
	return answer, err == nil, err
}

func msgReadMailbox(caller msgCaller) ([]envelope.Envelope, error) {
	data, err := os.ReadFile(filepath.Join(caller.Layout.BoxDir(caller.Dispatch), msgMailboxFile))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var messages []envelope.Envelope
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var m envelope.Envelope
		err := dec.Decode(&m)
		if errors.Is(err, io.EOF) {
			return messages, nil
		}
		if err != nil {
			return nil, fmt.Errorf("the mailbox is malformed: %w", err)
		}
		messages = append(messages, m)
	}
}

func msgWait(env msgEnv, args []string, s msgStreams) int {
	set := flag.NewFlagSet("wait", flag.ContinueOnError)
	var within time.Duration
	var asJSON bool
	set.DurationVar(&within, "deadline", 0, "")
	set.BoolVar(&asJSON, "json", false, "")
	if _, err := msgParse(set, args, 0); err != nil {
		return msgUsageError(s.stderr, err.Error())
	}
	var deadline time.Time
	if within != 0 {
		deadline = env.now().Add(within)
	}
	caller, err := env.caller()
	if err != nil {
		return msgFail(s.stderr, exitRefused, err)
	}
	waker, err := msgOpenWaker(env, caller.Layout.BoxDir(caller.Dispatch))
	if err != nil {
		return msgFail(s.stderr, exitIO, err)
	}
	defer func() { _ = waker.Close() }()
	for {
		mailbox, err := msgReadMailbox(caller)
		if err != nil {
			return msgFail(s.stderr, exitIO, err)
		}
		if len(mailbox) > 0 {
			return msgWriteExit(s.stderr, msgPrintMessages(s.stdout, mailbox, asJSON))
		}
		if code, stop := msgBlock(waker, deadline, exitMsgDeadline, "no message by the deadline", s.stderr); stop {
			return code
		}
	}
}

func msgPrintMessages(w io.Writer, messages []envelope.Envelope, asJSON bool) error {
	enc := json.NewEncoder(w)
	for _, m := range messages {
		var err error
		if asJSON {
			err = enc.Encode(m)
		} else {
			_, err = fmt.Fprintf(w, "%s %s from %s\n%s\n\n%s\n\n", m.ID, m.Kind, m.From.Dispatch, m.Subject, m.Body)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
