package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// guardLogTag maps the Go guard subcommand name to the audit-trail tag
// used in .evolve/guards.log by the bash counterparts. Preserves the
// log format byte-equivalent across the v11.4.0 bash→Go cutover so
// operators / parity audits / external tooling that grep guards.log
// see the same lines.
var guardLogTag = map[string]string{
	"ship":      "ship-gate",
	"phase":     "phase-gate-pre",
	"role":      "role-gate",
	"quota":     "research-quota-gate",
	"docdelete": "doc-deletion-guard",
	"chain":     "chain",
}

// appendGuardsLog writes one line to the given logPath mirroring the
// bash `echo "[ts] [<tag>] <line>" >> guards.log` pattern. Best-effort:
// failures are silent so hook latency isn't impacted by audit-log I/O.
func appendGuardsLog(logPath, guardName string, allow bool, reason string) {
	tag, ok := guardLogTag[guardName]
	if !ok {
		tag = guardName
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return
	}
	ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	decision := "ALLOW"
	if !allow {
		decision = "DENY"
	}
	line := fmt.Sprintf("[%s] [%s] %s", ts, tag, decision)
	if reason != "" {
		line += ": " + reason
	}
	line += "\n"
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write([]byte(line))
}

// runGuard implements `evolve guard <name>` — reads tool_input JSON from
// stdin, dispatches to the named guard, and exits 0 on Allow or 2 on Deny.
// Mirrors the bash hook contract (scripts/guards/*.sh).
func runGuard(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve guard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var evolveDir string
	var bypass bool
	fs.StringVar(&evolveDir, "evolve-dir", ".evolve", "path to .evolve/ state directory")
	fs.BoolVar(&bypass, "bypass", false, "emergency: bypass this guard")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "evolve guard: usage: evolve guard <name> [--evolve-dir DIR] < stdin.json")
		return 10
	}
	name := fs.Arg(0)
	// Read-only enumeration subcommand, distinct from the kernel-hook
	// guards (ship/phase/role/…). Doesn't take stdin, doesn't emit
	// allow/deny — just lists pending audit failures so an operator can
	// triage them (issue surfaced by cycle-107 retro: "16 non-expired
	// code-audit-fail entries (within 30d retention)" with no way to see
	// which entries).
	if name == "list-audit-fails" {
		return runListAuditFails(fs.Args()[1:], evolveDir, stdout, stderr)
	}
	if name == "triage-floors" {
		return runGuardTriageFloors(fs.Args()[1:], stdout, stderr)
	}
	guardFS := flag.NewFlagSet("evolve guard "+name, flag.ContinueOnError)
	guardFS.SetOutput(stderr)
	guardFS.StringVar(&evolveDir, "evolve-dir", evolveDir, "path to .evolve/ state directory")
	guardFS.BoolVar(&bypass, "bypass", bypass, "emergency: bypass this guard")
	if err := guardFS.Parse(fs.Args()[1:]); err != nil {
		return 10
	}
	if guardFS.NArg() != 0 {
		fmt.Fprintf(stderr, "evolve guard %s: unexpected arguments: %v\n", name, guardFS.Args())
		return 10
	}
	in, err := readGuardInput(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "evolve guard %s: stdin parse: %v\n", name, err)
		return 10
	}
	g, err := buildGuard(name, evolveDir, bypass)
	if err != nil {
		fmt.Fprintf(stderr, "evolve guard: %v\n", err)
		return 10
	}
	dec := g.Decide(context.Background(), in)
	logPath := filepath.Join(evolveDir, "guards.log")
	appendGuardsLog(logPath, name, dec.Allow, dec.Reason)
	payload := map[string]any{"guard": name, "allow": dec.Allow, "reason": dec.Reason}
	if buf, mErr := json.Marshal(payload); mErr == nil {
		fmt.Fprintf(stdout, "%s\n", buf)
	}
	if !dec.Allow {
		if dec.Reason != "" {
			fmt.Fprintf(stderr, "[guard:%s] DENY: %s\n", name, dec.Reason)
		}
		return 2
	}
	return 0
}

func readGuardInput(r io.Reader) (core.GuardInput, error) {
	var in core.GuardInput
	if r == nil {
		return in, nil
	}
	buf, err := io.ReadAll(r)
	if err != nil {
		return in, fmt.Errorf("read stdin: %w", err)
	}
	if len(buf) == 0 {
		return in, nil
	}
	var raw struct {
		ToolName  string         `json:"tool_name"`
		ToolInput map[string]any `json:"tool_input"`
		CWD       string         `json:"cwd"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return in, fmt.Errorf("unmarshal: %w", err)
	}
	in.ToolName = raw.ToolName
	in.ToolInput = raw.ToolInput
	in.CWD = raw.CWD
	return in, nil
}

// runListAuditFails enumerates the non-expired code-audit-fail entries
// from .evolve/state.json:failedApproaches[]. Flag surface:
//
//	--json   emit JSON array instead of the human table
//
// Exit code 0 even when zero entries match (an empty list is a valid
// answer, not an error).
func runListAuditFails(args []string, evolveDir string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve guard list-audit-fails", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	// Re-accept --evolve-dir here so it works whether the operator puts
	// it before or after the subcommand name. Default to the parent
	// runGuard's already-resolved value.
	fs.StringVar(&evolveDir, "evolve-dir", evolveDir, "path to .evolve/ state directory")
	fs.BoolVar(&asJSON, "json", false, "emit JSON array instead of human-readable table")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 10
	}

	statePath := filepath.Join(evolveDir, "state.json")
	raw, err := os.ReadFile(statePath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve guard list-audit-fails: read %s: %v\n", statePath, err)
		return 1
	}
	var st core.State
	if err := json.Unmarshal(raw, &st); err != nil {
		fmt.Fprintf(stderr, "evolve guard list-audit-fails: parse %s: %v\n", statePath, err)
		return 1
	}

	// Convert state.FailedAt ([]core.FailedRecord) → []failureadapter.Entry
	// so we can reuse the canonical retention filter. Field names match
	// byte-for-byte across the two structs (both mirror state.json).
	entries := make([]failureadapter.Entry, 0, len(st.FailedAt))
	for _, r := range st.FailedAt {
		entries = append(entries, failureadapter.Entry{
			TS:                r.TS,
			Cycle:             r.Cycle,
			Verdict:           r.Verdict,
			Classification:    failureadapter.Classification(r.Classification),
			RecordedAt:        r.RecordedAt,
			ExpiresAt:         r.ExpiresAt,
			AuditReportPath:   r.AuditReportPath,
			AuditReportSHA256: r.AuditReportSHA256,
			GitHead:           r.GitHead,
			TreeStateSHA:      r.TreeStateSHA,
			Defects:           r.Defects,
			Retrospected:      r.Retrospected,
			Summary:           r.Summary,
		})
	}
	pending := failureadapter.ListPendingByClass(entries, failureadapter.CodeAuditFail, time.Now())

	if asJSON {
		buf, mErr := json.MarshalIndent(pending, "", "  ")
		if mErr != nil {
			fmt.Fprintf(stderr, "evolve guard list-audit-fails: marshal: %v\n", mErr)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", buf)
		return 0
	}

	fmt.Fprintf(stdout, "%-6s | %-20s | %-7s | %-12s | %s\n", "cycle", "recorded_at", "verdict", "tree_state", "summary")
	fmt.Fprintln(stdout, "-------+----------------------+---------+--------------+--------")
	for _, e := range pending {
		summary := e.Summary
		if len(summary) > 80 {
			summary = summary[:77] + "..."
		}
		ts := e.RecordedAt
		if ts == "" {
			ts = e.TS
		}
		tree := e.TreeStateSHA
		if len(tree) > 12 {
			tree = tree[:12]
		}
		fmt.Fprintf(stdout, "%-6d | %-20s | %-7s | %-12s | %s\n", e.Cycle, ts, e.Verdict, tree, summary)
	}
	fmt.Fprintf(stdout, "\n(%d pending code-audit-fail entries within 30d retention)\n", len(pending))
	return 0
}

func buildGuard(name, evolveDir string, bypass bool) (core.Guard, error) {
	var workflow policy.WorkflowConfig
	if name == "docdelete" || name == "quota" {
		pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
		if err != nil {
			return nil, err
		}
		workflow = pol.WorkflowConfig()
	}
	switch name {
	case "ship":
		return guards.NewShip(bypass), nil
	case "phase":
		return guards.NewPhase(storage.New(evolveDir), bypass), nil
	case "role":
		return guards.NewRole(storage.New(evolveDir), bypass), nil
	case "docdelete":
		return guards.NewDocDelete(workflow.AllowDocDelete), nil
	case "quota":
		return guards.NewQuota(guards.QuotaConfig{AllowDeepResearch: workflow.AllowDeepResearch}), nil
	case "chain":
		return guards.NewChain(ledger.New(evolveDir)), nil
	default:
		return nil, fmt.Errorf("unknown guard %q (known: ship phase role docdelete quota chain triage-floors)", name)
	}
}

func runGuardTriageFloors(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve guard triage-floors", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: evolve guard triage-floors <workspace>")
		fmt.Fprintln(fs.Output(), "checks committed_floors and deferred_floors declarations against triage prose")
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 10
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 10
	}
	workspace := fs.Arg(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(workspace)))
	artifactPath := filepath.Join(workspace, triagecap.TriageArtifactName())
	companionPath := filepath.Join(workspace, triagecap.TriageDecisionName())
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve guard triage-floors: read %s: %v\n", artifactPath, err)
		return 1
	}
	knownPkgs := triagecap.KnownPackages(projectRoot)
	var messages []string
	if msg := triagecap.FloorDivergenceCorrective(string(artifact), companionPath, knownPkgs); msg != "" {
		messages = append(messages, msg)
	}
	if msg := triagecap.DeferredFloorDivergence(string(artifact), companionPath, knownPkgs); msg != "" {
		messages = append(messages, msg)
	}
	if len(messages) > 0 {
		for _, msg := range messages {
			fmt.Fprintln(stdout, msg)
		}
		return 1
	}
	fmt.Fprintln(stdout, "triage floor declarations match prose")
	return 0
}
