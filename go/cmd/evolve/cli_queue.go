package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

const (
	queueRequestSchema   = "evolve.queue.request/1"
	queueApplyVerb       = "queue-apply"
	queueApplyDesc       = "apply the CLI requests queued during the wave (.evolve/queue)"
	queueStageEnforce    = "enforce"
	queueTimeLayout      = "2006-01-02T15:04:05.000000000Z07:00"
	roleQueue            = "queue"
	codeCLIQueueFailed   = signalcenter.Code("CLI_QUEUE_FAILED")
	codeCLIQueueUnlanded = signalcenter.Code("CLI_QUEUE_UNLANDED")
	queueShelfProcessing = "processing"
	queueShelfApplied    = "applied"
	queueShelfFailed     = "failed"
	queueShelfRequeued   = "requeued"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleCLI, codeCLIQueueFailed, "the boundary could not apply a queued CLI request (cli-contract.md §4 rule 3); the request moved to .evolve/queue/failed/; fields verb, request, rc, shelf")
	signalcenter.RegisterCode(signalcenter.ModuleCLI, codeCLIQueueUnlanded, "the applied queue left tracked changes beyond the loop stamps; the next sync-main refuses them until an operator lands them (cli-contract.md §4.6); fields paths")
}

type queuedRequest struct {
	Schema   string   `json:"schema"`
	Verb     string   `json:"verb"`
	Args     []string `json:"args"`
	QueuedAt string   `json:"queued_at"`
}

type queueReceipt struct {
	Verb string `json:"verb"`
	Path string `json:"path"`
}

func queueDir(root string) string { return filepath.Join(paths.EvolveDirOf(root), "queue") }

func queueFileName(r queuedRequest) string {
	return strings.ReplaceAll(r.Verb, " ", "-") + "-" + strings.ReplaceAll(r.QueuedAt, ":", "") + "-" + strconv.Itoa(os.Getpid()) + ".json"
}

func writeQueuedRequest(root string, r queuedRequest) (string, error) {
	path := filepath.Join(queueDir(root), queueFileName(r))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, atomicwrite.JSON(path, r)
}

type midWaveCall struct {
	root, verb, prefix string
	args               []string
	asJSON             bool
	stdout, stderr     io.Writer
}

func inboxQueueGate(root, verb string, args []string, asJSON bool, stdout, stderr io.Writer) (int, bool) {
	return midWaveCall{root: root, verb: "inbox " + verb, prefix: "inbox " + verb, args: args, asJSON: asJSON, stdout: stdout, stderr: stderr}.queueOrRefuse()
}

func liveRunsOf(root string) []runlease.LiveRun {
	return runlease.LiveRuns(filepath.Join(paths.EvolveDirOf(root), "runs"), time.Now())
}

func (c midWaveCall) queueOrRefuse() (int, bool) {
	if subagent.ReadDispatchDepth(os.Getenv) > 0 {
		fmt.Fprintf(c.stderr, "%s: refused inside a phase: a phase agent never runs %s; it asks the pipeline\n", c.prefix, c.verb)
		return cliExitRefused, true
	}
	live := liveRunsOf(c.root)
	if len(live) == 0 {
		return cliExitOK, false
	}
	req := queuedRequest{Schema: queueRequestSchema, Verb: c.verb, Args: append([]string{}, c.args...), QueuedAt: time.Now().UTC().Format(queueTimeLayout)}
	if c.stage() != queueStageEnforce {
		fmt.Fprintf(c.stderr, "%s: refused — a loop lane is live (%s, pid %d); run it at a wave boundary (`evolve loop-stop --wait`)\n", c.prefix, live[0].Dir, live[0].Lease.OwnerPID)
		fmt.Fprintf(c.stderr, "%s: shadow: the stage enforce writes %s and exits 4 (policy isolation.i15.stage)\n", c.prefix, filepath.Join(queueDir(c.root), queueFileName(req)))
		return cliExitRefused, true
	}
	path, err := writeQueuedRequest(c.root, req)
	if err != nil {
		fmt.Fprintf(c.stderr, "%s: write the queue request: %v\n", c.prefix, err)
		return cliExitIO, true
	}
	fmt.Fprintf(c.stderr, "%s: a loop lane is live (%s), so the request waits for the next wave boundary\n", c.prefix, live[0].Dir)
	receipt := queueReceipt{Verb: c.verb, Path: path}
	if c.asJSON {
		if rc := printVerbJSON(c.verb, receipt, c.stdout, c.stderr); rc != cliExitOK {
			return rc, true
		}
		return cliExitQueued, true
	}
	fmt.Fprintf(c.stdout, "%s: queued %s\n", c.prefix, path)
	return cliExitQueued, true
}

func (c midWaveCall) stage() string {
	pol, err := policy.Load(paths.PolicyPath(paths.EvolveDirOf(c.root)))
	if err != nil {
		fmt.Fprintf(c.stderr, "%s: WARN: the policy is unreadable, so the queue stays at stage shadow: %v\n", c.prefix, err)
		return ""
	}
	return pol.IsolationQueueStage()
}

type queueEntry struct {
	path string
	req  queuedRequest
	err  error
}

func loadQueue(root string) ([]queueEntry, error) {
	files, err := os.ReadDir(queueDir(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries := []queueEntry{}
	for _, f := range files {
		if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
			continue
		}
		entries = append(entries, readQueueEntry(filepath.Join(queueDir(root), f.Name())))
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].req.QueuedAt != entries[j].req.QueuedAt {
			return entries[i].req.QueuedAt < entries[j].req.QueuedAt
		}
		return entries[i].path < entries[j].path
	})
	return entries, nil
}

func readQueueEntry(path string) queueEntry {
	e := queueEntry{path: path}
	raw, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(raw, &e.req)
	}
	if err == nil && e.req.Schema != queueRequestSchema {
		err = fmt.Errorf("schema %q is not %s", e.req.Schema, queueRequestSchema)
	}
	e.err = err
	return e
}

var queueHandlers = map[string]func([]string, io.Reader, io.Writer, io.Writer) int{
	"inbox":       runInbox,
	"cli-routing": runCLIRouting,
}

func applyQueuedRequest(root string, e queueEntry, stdout, stderr io.Writer) int {
	if e.err != nil {
		fmt.Fprintf(stderr, "%s: %s: %v\n", queueApplyVerb, filepath.Base(e.path), e.err)
		return cliExitInput
	}
	words := strings.Fields(e.req.Verb)
	if len(words) != 2 || queueHandlers[words[0]] == nil {
		fmt.Fprintf(stderr, "%s: %s: %q is not a queued verb\n", queueApplyVerb, filepath.Base(e.path), e.req.Verb)
		return cliExitInput
	}
	args := slices.Concat(words[1:], e.req.Args, []string{"--project-root", root})
	return queueHandlers[words[0]](args, strings.NewReader(""), stdout, stderr)
}

func runQueueApply(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var projectRoot string
	operands, err := cliFlags{values: map[string]*string{"--project-root": &projectRoot}}.parse(args)
	if err == nil && len(operands) > 0 {
		err = fmt.Errorf("unexpected argument %q", operands[0])
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\nusage: %s [--project-root P]\n", queueApplyVerb, err, queueApplyVerb)
		return cliExitUsage
	}
	root, err := loopStopRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cwd: %v\n", queueApplyVerb, err)
		return cliExitIO
	}
	entries, err := loadQueue(root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: list the queue: %v\n", queueApplyVerb, err)
		return cliExitIO
	}
	if len(entries) == 0 {
		fmt.Fprintf(stdout, "%s: no queued request\n", queueApplyVerb)
		return cliExitOK
	}
	if live := liveRunsOf(root); len(live) > 0 {
		fmt.Fprintf(stdout, "%s: %d request(s) wait, because %s holds a live run lease\n", queueApplyVerb, len(entries), filepath.Base(live[0].Dir))
		return cliExitOK
	}
	applied := applyQueue(root, entries, stdout, stderr)
	fmt.Fprintf(stdout, "%s: applied %d of %d queued request(s)\n", queueApplyVerb, applied, len(entries))
	return cliExitOK
}

func applyQueue(root string, entries []queueEntry, stdout, stderr io.Writer) int {
	signals, closeSignals := newRoleSignalCenter(roleQueue, root, paths.EvolveDirOf(root), stderr)
	defer closeSignals()
	before := unlandedPaths(root)
	applied := 0
	for _, e := range entries {
		if applyOneQueued(signals, root, e, stdout, stderr) {
			applied++
		}
	}
	reportUnlanded(signals, root, before, stdout)
	return applied
}

func applyOneQueued(signals *signalcenter.Center, root string, e queueEntry, stdout, stderr io.Writer) bool {
	claimed, err := shelveQueuedRequest(root, e.path, queueShelfProcessing)
	if err != nil {
		fmt.Fprintf(stderr, "%s: claim %s: %v; it stays queued\n", queueApplyVerb, filepath.Base(e.path), err)
		return false
	}
	e.path = claimed
	return settleQueuedRequest(signals, root, e, applyQueuedRequest(root, e, stdout, stderr), stderr)
}

func settleQueuedRequest(signals *signalcenter.Center, root string, e queueEntry, rc int, stderr io.Writer) bool {
	name := filepath.Base(e.path)
	ev := signalcenter.Event{Module: signalcenter.ModuleCLI, Origin: "evolve.settleQueuedRequest", Kind: signalcenter.KindCLIQueueApplied,
		Severity: signalcenter.SeverityInfo, Reason: fmt.Sprintf("applied the queued request %s", name),
		Fields: map[string]string{"verb": e.req.Verb, "request": name, "rc": fmt.Sprint(rc)}}
	shelf := queueShelfApplied
	switch rc {
	case cliExitOK:
	case cliExitQueued:
		shelf, ev.Reason = queueShelfRequeued, fmt.Sprintf("a run lease became live, so the verb queued %s again", name)
	default:
		shelf = queueShelfFailed
		ev.Severity, ev.Code = signalcenter.SeverityWarn, codeCLIQueueFailed
		ev.Reason = fmt.Sprintf("the queued request %s exited %d; it moved to %s", name, rc, filepath.Join(queueDir(root), shelf))
	}
	ev.Fields["shelf"] = shelf
	if _, err := shelveQueuedRequest(root, e.path, shelf); err != nil {
		fmt.Fprintf(stderr, "%s: move %s to %s: %v; it stays in %s\n", queueApplyVerb, name, shelf, err, queueShelfProcessing)
	}
	signals.Emit(ev)
	return rc == cliExitOK
}

func shelveQueuedRequest(root, path, shelf string) (string, error) {
	dir := filepath.Join(queueDir(root), shelf)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	to := filepath.Join(dir, filepath.Base(path))
	return to, os.Rename(path, to)
}

func unlandedPaths(root string) map[string]bool {
	partition, err := inboxstamps.Classify(context.Background(), gitexec.Default(root))
	out := map[string]bool{}
	if err != nil {
		return out
	}
	for _, p := range partition.Other {
		out[p] = true
	}
	return out
}

func reportUnlanded(signals *signalcenter.Center, root string, before map[string]bool, stdout io.Writer) {
	var fresh []string
	for p := range unlandedPaths(root) {
		if !before[p] {
			fresh = append(fresh, p)
		}
	}
	if len(fresh) == 0 {
		return
	}
	sort.Strings(fresh)
	fmt.Fprintf(stdout, "%s: land these tracked changes before the next sync-main, which refuses them: %s\n", queueApplyVerb, strings.Join(fresh, ", "))
	signals.Emit(signalcenter.Event{Module: signalcenter.ModuleCLI, Origin: "evolve.reportUnlanded", Kind: signalcenter.KindCLIQueueApplied,
		Severity: signalcenter.SeverityWarn, Code: codeCLIQueueUnlanded,
		Reason: "the applied queue left tracked changes that the next sync-main refuses until they land",
		Fields: map[string]string{"paths": strings.Join(fresh, ",")}})
}
