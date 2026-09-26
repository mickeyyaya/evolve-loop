//go:build acs

// Package cycle1698 materialises the acceptance criteria of the one
// fleet-scoped task pinned to this lane: `unify-auditor-ledger-readers`
// (.evolve/inbox/processing/cycle-1698/2026-08-27T06-00-00Z-unify-auditor-ledger-readers.json;
// triage top_n). Three readers each re-declare the auditor ledger row and
// their own scan — ship.findLatestAudit (internal/phases/ship/audit.go),
// cmd/evolve latestAuditEntry (cmd_composition_wiring.go) and
// releasepreflight.checkRecentAudit (a raw-line regex walk). The task folds
// them onto one leaf helper, internal/auditledger, with a typed
// ErrNoAuditorForRun sentinel each consumer maps onto its own vocabulary.
// redteamcheck is out of scope (cycle-scoped, all roles, no run scoping).
//
// PINNED HELPER SURFACE (the minimum every consumer needs; nothing more):
//
//	auditledger.LatestAuditorEntry(ledgerPath, runID string) (Entry|*Entry, error)
//	auditledger.ErrNoAuditorForRun   — an error value for errors.Is
//	Entry fields RunID, GitHEAD, ArtifactPath
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - POSITIVE : 001 — run-scoped binding, latest-any without a run, the
//     kind+role row identity, alien lines skipped, structured (not regex) parse.
//   - NEGATIVE : 002 — every "no bindable row" shape is the typed sentinel and
//     the foreign refusal names the refused entry; 008 — redteamcheck untouched
//     and the change set is non-vacuous.
//   - EDGE     : 003 — a missing ledger and an unreadable ledger stay
//     distinguishable from a miss; the release preflight keeps its advisory NONE.
//   - WIRING   : 005/006/007 — every consumer imports the helper, none re-declares
//     the row schema or regex, and the production caller releasepreflight.Run
//     binds exactly the row the helper binds.
//   - FLOOR    : 004 leaf, 009 stale TODO, 010 apicover graduation, 011 the
//     touched packages' own suites.
package cycle1698

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

const (
	modulePath   = "github.com/mickeyyaya/evolve-loop/go"
	helperImport = modulePath + "/internal/auditledger"
	helperDirRel = "go/internal/auditledger/"
)

// consumers are the three duplicate readers: the module-relative package the
// go tool resolves, and its directory under go/.
var consumers = []struct{ pkg, dir string }{
	{"./internal/phases/ship", "internal/phases/ship"},
	{"./cmd/evolve", "cmd/evolve"},
	{"./internal/releasepreflight", "internal/releasepreflight"},
}

// ledgerLineFragments are the raw-line spellings of auditor-row fields a
// regex/substring reader needs; a consumer that still carries one is still
// parsing the row itself.
var ledgerLineFragments = []string{
	`"role":"auditor"`,
	`"artifact_path":"`,
	`"git_head":"`,
	`"worktree_tree_sha":"`,
	`"ts":"`,
}

func mustWrite(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeLedger(t *testing.T, lines ...string) string {
	t.Helper()
	body := ""
	if len(lines) > 0 {
		body = strings.Join(lines, "\n") + "\n"
	}
	return mustWrite(t, filepath.Join(t.TempDir(), "ledger.jsonl"), body)
}

func writeArtifact(t *testing.T, body string) string {
	t.Helper()
	return mustWrite(t, filepath.Join(t.TempDir(), "audit-report.md"), body)
}

func jsonString(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// runGo runs the go tool from the module dir and returns stdout, stderr and
// the exit code; a tool that cannot start fails the predicate loudly.
func runGo(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	t.Fatalf("go %s could not start: %v", strings.Join(args, " "), err)
	return "", "", -1
}

// runPreflight drives the production entry point releasepreflight.Run with
// every non-ledger step stubbed green, so step 4 (the auditor-row read) is the
// only live input.
func runPreflight(t *testing.T, ledgerPath string) (releasepreflight.Result, error) {
	t.Helper()
	repo := t.TempDir()
	plugin := mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"name":"x","version":"1.0.0"}`)
	return releasepreflight.Run(releasepreflight.Options{
		Target:           "1.0.1",
		RepoRoot:         repo,
		SkipTests:        true,
		Stderr:           io.Discard,
		PluginJSONPath:   plugin,
		LedgerPath:       ledgerPath,
		Now:              time.Now,
		GitClean:         func(string) (bool, error) { return true, nil },
		CurrentBranch:    func(string) (string, error) { return "main", nil },
		GateTestRunner:   func(string, string) error { return nil },
		NameGuard:        func(string) ([]naminguard.Violation, error) { return nil, nil },
		SimulationRunner: func(string) error { return nil },
		CIConclusion:     func(string) (releasepreflight.CIRunStatus, error) { return releasepreflight.CIRunStatus{}, nil },
		HeadSHA:          func(string) (string, error) { return "", nil },
	})
}

// -----------------------------------------------------------------------------
// AC1 — the helper exists and owns the scan + run-scope predicate.
// -----------------------------------------------------------------------------

// TestC1698_001_LatestAuditorEntryBindsThisRunsAuditorRow pins the scan every
// consumer delegates to: newest-first, the row identity kind=agent_subprocess
// AND role=auditor, run-scoped when a run id is given, latest-any without one,
// forward-compatible over alien lines, and a structured JSON parse (the
// whitespace row is invisible to a raw-line regex reader).
func TestC1698_001_LatestAuditorEntryBindsThisRunsAuditorRow(t *testing.T) {
	cases := []struct {
		name, runID, wantRun, wantHead, wantArtifact string
		lines                                        []string
	}{
		{
			name: "own run bound over a newer sibling's row", runID: "MINE",
			wantRun: "MINE", wantHead: "shaMine", wantArtifact: "/runs/mine/audit-report.md",
			lines: []string{
				`{"role":"auditor","kind":"agent_subprocess","run_id":"MINE","git_head":"shaMine","artifact_path":"/runs/mine/audit-report.md"}`,
				`{"role":"auditor","kind":"agent_subprocess","run_id":"SIBLING","git_head":"shaSibling","artifact_path":"/runs/sib/audit-report.md"}`,
			},
		},
		{
			name: "no run context binds the newest auditor row", runID: "",
			wantRun: "SIBLING", wantHead: "shaSibling", wantArtifact: "/runs/sib/audit-report.md",
			lines: []string{
				`{"role":"auditor","kind":"agent_subprocess","run_id":"MINE","git_head":"shaMine","artifact_path":"/runs/mine/audit-report.md"}`,
				`{"role":"auditor","kind":"agent_subprocess","run_id":"SIBLING","git_head":"shaSibling","artifact_path":"/runs/sib/audit-report.md"}`,
			},
		},
		{
			name: "the newest of this run's rows wins", runID: "MINE",
			wantRun: "MINE", wantHead: "shaNew",
			lines: []string{
				`{"role":"auditor","kind":"agent_subprocess","run_id":"MINE","git_head":"shaOld"}`,
				`{"role":"auditor","kind":"agent_subprocess","run_id":"MINE","git_head":"shaNew"}`,
			},
		},
		{
			name: "wrong kind, wrong role and unparseable lines are not auditor rows", runID: "MINE",
			wantRun: "MINE", wantHead: "shaReal",
			lines: []string{
				`{"role":"auditor","kind":"agent_subprocess","run_id":"MINE","git_head":"shaReal"}`,
				`{"role":"auditor","kind":"phase_verdict","run_id":"MINE","git_head":"shaWrongKind"}`,
				`{"role":"builder","kind":"agent_subprocess","run_id":"MINE","git_head":"shaWrongRole"}`,
				`{not json`,
				``,
			},
		},
		{
			name: "whitespace-formatted JSON row is parsed structurally", runID: "MINE",
			wantRun: "MINE", wantHead: "shaSpaced", wantArtifact: "/runs/spaced/audit-report.md",
			lines: []string{
				`{"role": "auditor", "kind": "agent_subprocess", "run_id": "MINE", "git_head": "shaSpaced", "artifact_path": "/runs/spaced/audit-report.md"}`,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, err := auditledger.LatestAuditorEntry(writeLedger(t, c.lines...), c.runID)
			if err != nil {
				t.Fatalf("LatestAuditorEntry(runID=%q): %v", c.runID, err)
			}
			if e.RunID != c.wantRun || e.GitHEAD != c.wantHead {
				t.Errorf("bound run_id=%q git_head=%q, want run_id=%q git_head=%q", e.RunID, e.GitHEAD, c.wantRun, c.wantHead)
			}
			if c.wantArtifact != "" && e.ArtifactPath != c.wantArtifact {
				t.Errorf("bound artifact_path=%q, want %q", e.ArtifactPath, c.wantArtifact)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// AC4 — negative: no bindable row is the typed sentinel, with forensics.
// -----------------------------------------------------------------------------

// TestC1698_002_NoBindableAuditorRowIsTheTypedSentinel covers every miss shape.
// Each must be errors.Is(ErrNoAuditorForRun) — the one value ship maps to
// AUDIT_BINDING_NO_AUDITOR and composition fails closed on — and never read as
// a missing ledger. The foreign-run refusal must carry the refused entry (its
// run id and git_head) so an operator sees what would have been bound
// (cycle-1571 H3).
func TestC1698_002_NoBindableAuditorRowIsTheTypedSentinel(t *testing.T) {
	cases := []struct {
		name, runID string
		lines       []string
		wantInMsg   []string
	}{
		{
			name: "foreign-run-only ledger names the refused entry", runID: "MINE",
			lines:     []string{`{"role":"auditor","kind":"agent_subprocess","run_id":"SIBLING","git_head":"shaForeign"}`},
			wantInMsg: []string{"SIBLING", "shaForeign"},
		},
		{
			name: "unstamped rows never satisfy a run-scoped bind", runID: "MINE",
			lines: []string{`{"role":"auditor","kind":"agent_subprocess","git_head":"shaUnstamped"}`},
		},
		{
			name: "own-run row of a non-subprocess kind is not an auditor row", runID: "MINE",
			lines: []string{`{"role":"auditor","kind":"phase_verdict","run_id":"MINE","git_head":"shaWrongKind"}`},
		},
		{
			name: "ledger with no auditor row", runID: "",
			lines: []string{`{"role":"builder","kind":"agent_subprocess","run_id":"MINE","git_head":"shaBuilder"}`},
		},
		{
			name: "empty ledger file", runID: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, err := auditledger.LatestAuditorEntry(writeLedger(t, c.lines...), c.runID)
			if err == nil {
				t.Fatalf("runID=%q bound %+v — a miss must refuse, never bind", c.runID, e)
			}
			if !errors.Is(err, auditledger.ErrNoAuditorForRun) {
				t.Errorf("miss error %q is not errors.Is(ErrNoAuditorForRun) — consumers cannot map it onto their own vocabulary", err)
			}
			if errors.Is(err, fs.ErrNotExist) {
				t.Errorf("miss error %q reads as a missing ledger — ship would report NO_LEDGER for a present ledger", err)
			}
			for _, w := range c.wantInMsg {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("refusal %q does not name %q — the refused foreign entry is lost for forensics", err, w)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// AC5 — edge: an absent/unreadable ledger keeps each consumer's semantics.
// -----------------------------------------------------------------------------

// TestC1698_003_MissingLedgerStaysDistinguishableFromAMiss: ship maps an absent
// ledger to AUDIT_BINDING_NO_LEDGER and an unreadable one to a transient
// STATE_IO, while releasepreflight treats absence as advisory NONE. The helper
// must keep both distinguishable from the miss sentinel, and the preflight's
// production path must keep its advisory verdict.
func TestC1698_003_MissingLedgerStaysDistinguishableFromAMiss(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "nonexistent", "ledger.jsonl")
	for _, runID := range []string{"", "MINE"} {
		_, err := auditledger.LatestAuditorEntry(absent, runID)
		switch {
		case err == nil:
			t.Errorf("runID=%q: absent ledger returned no error", runID)
		case !errors.Is(err, fs.ErrNotExist):
			t.Errorf("runID=%q: absent-ledger error %q is not errors.Is(fs.ErrNotExist) — ship can no longer tell NO_LEDGER apart", runID, err)
		case errors.Is(err, auditledger.ErrNoAuditorForRun):
			t.Errorf("runID=%q: absent-ledger error %q is the miss sentinel — a missing ledger is not a missing auditor row", runID, err)
		}
	}

	unreadable := t.TempDir() // a directory: present, but ReadFile fails
	_, err := auditledger.LatestAuditorEntry(unreadable, "MINE")
	switch {
	case err == nil:
		t.Errorf("unreadable ledger returned no error")
	case errors.Is(err, auditledger.ErrNoAuditorForRun), errors.Is(err, fs.ErrNotExist):
		t.Errorf("unreadable-ledger error %q is misclassified as a miss or an absence — ship's transient STATE_IO path is lost", err)
	}

	res, err := runPreflight(t, absent)
	if err != nil {
		t.Errorf("releasepreflight.Run with no ledger must stay advisory, got error: %v", err)
	}
	if res.AuditVerdict != "NONE" {
		t.Errorf("releasepreflight.Run with no ledger: AuditVerdict=%q, want the advisory NONE", res.AuditVerdict)
	}
}

// TestC1698_004_HelperIsALeafOwningNoConsumerVocabulary: the helper may not
// depend on any consumer or on core (whose ship error codes are ship's
// vocabulary) — the leaf shape of internal/treestate. A helper that did would
// either cycle with its consumers or return consumer-specific errors.
func TestC1698_004_HelperIsALeafOwningNoConsumerVocabulary(t *testing.T) {
	out, errOut, code := runGo(t, "list", "-deps", "-f", "{{.ImportPath}}", "./internal/auditledger")
	if code != 0 {
		t.Fatalf("go list -deps ./internal/auditledger exited %d — the helper package does not build:\n%s", code, errOut)
	}
	forbidden := []string{
		modulePath + "/internal/core",
		modulePath + "/internal/phases",
		modulePath + "/internal/releasepreflight",
		modulePath + "/cmd",
	}
	for _, dep := range strings.Split(strings.TrimSpace(out), "\n") {
		for _, f := range forbidden {
			if dep == f || strings.HasPrefix(dep, f+"/") {
				t.Errorf("internal/auditledger depends on %s — the helper must stay a leaf below its consumers", dep)
			}
		}
	}
}

// -----------------------------------------------------------------------------
// AC2 — all three consumers delegate.
// -----------------------------------------------------------------------------

// TestC1698_005_EveryConsumerImportsTheHelper asks the go tool for each
// consumer's non-test imports. The compiler rejects unused imports, so an
// import of the helper is a use of it.
func TestC1698_005_EveryConsumerImportsTheHelper(t *testing.T) {
	for _, c := range consumers {
		out, errOut, code := runGo(t, "list", "-f", `{{join .Imports "\n"}}`, c.pkg)
		if code != 0 {
			t.Errorf("go list %s exited %d:\n%s", c.pkg, code, errOut)
			continue
		}
		imported := false
		for _, imp := range strings.Split(out, "\n") {
			if strings.TrimSpace(imp) == helperImport {
				imported = true
			}
		}
		if !imported {
			t.Errorf("%s does not import %s — it still reads the auditor row on its own", c.pkg, helperImport)
		}
	}
}

// jsonTagNames returns the json field names a struct type declares.
func jsonTagNames(st *ast.StructType) map[string]bool {
	names := map[string]bool{}
	for _, f := range st.Fields.List {
		if f.Tag == nil {
			continue
		}
		raw, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			continue
		}
		name, _, _ := strings.Cut(reflect.StructTag(raw).Get("json"), ",")
		names[name] = true
	}
	return names
}

// TestC1698_006_ConsumersRedeclareNoLedgerRowSchema parses every non-test file
// of the three consumers: no struct may re-declare the ledger row (json tags
// role AND kind), and no string literal may carry a raw-line field spelling a
// regex/substring reader needs. Either one is a second copy of the row schema.
func TestC1698_006_ConsumersRedeclareNoLedgerRowSchema(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, c := range consumers {
		pkgDir := filepath.Join(root, "go", c.dir)
		files, err := filepath.Glob(filepath.Join(pkgDir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no Go files in %s (err=%v)", pkgDir, err)
		}
		fset := token.NewFileSet()
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			af, err := parser.ParseFile(fset, file, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", file, err)
			}
			ast.Inspect(af, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.StructType:
					if tags := jsonTagNames(x); tags["role"] && tags["kind"] {
						t.Errorf("%s: struct re-declares the auditor ledger row (json role+kind) — use auditledger's row type", fset.Position(x.Pos()))
					}
				case *ast.BasicLit:
					if x.Kind != token.STRING {
						return true
					}
					s, err := strconv.Unquote(x.Value)
					if err != nil {
						return true
					}
					for _, frag := range ledgerLineFragments {
						if strings.Contains(s, frag) {
							t.Errorf("%s: literal %s still parses the auditor row from the raw line (%s)", fset.Position(x.Pos()), x.Value, frag)
						}
					}
				}
				return true
			})
		}
	}
}

// assertPreflightAgrees drives releasepreflight.Run over ledger and requires
// it to bind exactly the row auditledger binds, with a PASS verdict.
func assertPreflightAgrees(t *testing.T, ledger, wantArtifact string) {
	t.Helper()
	e, err := auditledger.LatestAuditorEntry(ledger, "")
	if err != nil {
		t.Fatalf("LatestAuditorEntry: %v", err)
	}
	if e.ArtifactPath != wantArtifact {
		t.Fatalf("fixture premise: helper bound %q, want %q", e.ArtifactPath, wantArtifact)
	}
	res, err := runPreflight(t, ledger)
	if err != nil {
		t.Errorf("releasepreflight.Run: %v", err)
	}
	if res.AuditArtifact != e.ArtifactPath || res.AuditVerdict != "PASS" {
		t.Errorf("releasepreflight bound artifact=%q verdict=%q; the shared helper binds %q (PASS) — releasepreflight still applies its own row definition",
			res.AuditArtifact, res.AuditVerdict, e.ArtifactPath)
	}
}

// TestC1698_007_ReleasePreflightBindsTheSameAuditorRowAsTheHelper is the
// behavioral delegation proof through the production caller: for the same
// ledger bytes, releasepreflight.Run must pick the row the helper picks. The
// two shapes are exactly where a private raw-line reader diverges — it misses
// a whitespace-formatted row and it takes any line mentioning the auditor role.
func TestC1698_007_ReleasePreflightBindsTheSameAuditorRowAsTheHelper(t *testing.T) {
	now := jsonString(t, time.Now().UTC().Format(time.RFC3339))

	t.Run("whitespace-formatted auditor row", func(t *testing.T) {
		art := writeArtifact(t, "# Audit\n\nVerdict: PASS\n")
		ledger := writeLedger(t, fmt.Sprintf(
			`{"ts": %s, "role": "auditor", "kind": "agent_subprocess", "run_id": "R1", "exit_code": 0, "artifact_path": %s, "git_head": "shaSpaced"}`,
			now, jsonString(t, art)))
		assertPreflightAgrees(t, ledger, art)
	})

	t.Run("auditor-role row of a non-subprocess kind", func(t *testing.T) {
		real := writeArtifact(t, "# Audit\n\nVerdict: PASS\n")
		alien := writeArtifact(t, "# Audit\n\nVerdict: PASS\n")
		ledger := writeLedger(t,
			fmt.Sprintf(`{"ts":%s,"role":"auditor","kind":"agent_subprocess","run_id":"R1","exit_code":0,"artifact_path":%s,"git_head":"shaReal"}`,
				now, jsonString(t, real)),
			fmt.Sprintf(`{"ts":%s,"role":"auditor","kind":"phase_verdict","run_id":"R1","artifact_path":%s,"git_head":"shaAlien"}`,
				now, jsonString(t, alien)))
		assertPreflightAgrees(t, ledger, real)
	})
}

// -----------------------------------------------------------------------------
// AC3 — redteamcheck is out of scope; the lane is non-vacuous.
// -----------------------------------------------------------------------------

// changeSet maps each path the lane changed to a git status letter: committed
// since the nearest fork point with main / origin/main, overlaid with the
// working tree — so it holds whether or not the Builder has committed.
func changeSet(t *testing.T, root string) map[string]string {
	t.Helper()
	base := forkPoint(t, root)
	changes := map[string]string{}
	out, errOut, code, _ := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-status", "--no-renames", base, "HEAD")
	if code != 0 {
		t.Fatalf("git diff --name-status %s HEAD: exit %d: %s", base, code, errOut)
	}
	for _, line := range strings.Split(out, "\n") {
		if st, p, ok := strings.Cut(line, "\t"); ok && st != "" && p != "" {
			changes[p] = st[:1]
		}
	}
	status, errOut, code, _ := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--untracked-files=all", "--no-renames")
	if code != 0 {
		t.Fatalf("git status: exit %d: %s", code, errOut)
	}
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		xy, p := line[:2], line[3:]
		switch {
		case strings.Contains(xy, "D"):
			changes[p] = "D"
		case xy == "??":
			changes[p] = "A"
		default:
			if _, seen := changes[p]; !seen {
				changes[p] = "M"
			}
		}
	}
	return changes
}

// forkPoint is the nearest merge-base of HEAD with main / origin/main.
func forkPoint(t *testing.T, root string) string {
	t.Helper()
	var bases []string
	for _, ref := range []string{"main", "origin/main"} {
		if mb, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", ref); code == 0 {
			bases = append(bases, strings.TrimSpace(mb))
		}
	}
	if len(bases) == 0 {
		t.Fatalf("no merge-base with main or origin/main in %s", root)
	}
	base := bases[0]
	for _, cand := range bases[1:] {
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "--is-ancestor", base, cand); code == 0 {
			base = cand
		}
	}
	return base
}

// TestC1698_008_RedteamcheckIsOutOfScopeAndUntouched: redteamcheck answers a
// different question (did every role report this cycle), so the lane must not
// edit it or route it through the run-scoped helper — and the lane must
// actually have added the helper package.
func TestC1698_008_RedteamcheckIsOutOfScopeAndUntouched(t *testing.T) {
	root := acsassert.RepoRoot(t)
	changes := changeSet(t, root)
	addedHelper := false
	for p := range changes {
		if strings.HasPrefix(p, "go/internal/redteamcheck/") {
			t.Errorf("lane changed %s — redteamcheck is out of scope (cycle-scoped, all roles, no run scoping)", p)
		}
		if strings.HasPrefix(p, helperDirRel) {
			addedHelper = true
		}
	}
	if !addedHelper {
		t.Errorf("the change set touches nothing under %s — no helper package was added", helperDirRel)
	}
	out, errOut, code := runGo(t, "list", "-deps", "-f", "{{.ImportPath}}", "./internal/redteamcheck")
	if code != 0 {
		t.Fatalf("go list -deps ./internal/redteamcheck exited %d:\n%s", code, errOut)
	}
	for _, dep := range strings.Split(out, "\n") {
		if strings.TrimSpace(dep) == helperImport {
			t.Errorf("redteamcheck now depends on %s — its cycle-scoped completeness check must not be folded into the run-scoped reader", helperImport)
		}
	}
}

// -----------------------------------------------------------------------------
// AC6 — the stale TODO is gone.
// -----------------------------------------------------------------------------

// TestC1698_009_StaleMergeConcurrencyTODOIsGone: the TODO asked to fold the
// readers "if a third consumer appears"; once folded it is false. It must not
// survive in the file it annotated nor move anywhere else in the Go tree.
//
// acs-predicate: config-check — a code comment has no behavior to exercise;
// its absence is the criterion.
func TestC1698_009_StaleMergeConcurrencyTODOIsGone(t *testing.T) {
	root := acsassert.RepoRoot(t)
	marker := "TODO(merge-" + "concurrency-2026)"
	wiring := filepath.Join(root, "go", "cmd", "evolve", "cmd_composition_wiring.go")
	if !acsassert.FileExists(t, wiring) {
		t.Fatalf("%s is gone — the composition consumer must be migrated, not deleted", wiring)
	}
	acsassert.FileNotContains(t, wiring, marker)
	err := filepath.WalkDir(filepath.Join(root, "go"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "acs" && filepath.Dir(p) == filepath.Join(root, "go") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.Contains(string(raw), marker) {
			t.Errorf("%s still carries %s", p, marker)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk go/: %v", err)
	}
}

// -----------------------------------------------------------------------------
// House rule 1 — new-package graduation into the repo-wide apicover gate.
// -----------------------------------------------------------------------------

// TestC1698_010_HelperGraduatesIntoTheApicoverGate: a new internal package must
// be enrolled in go/.apicover-enforce with every export documented, named by a
// package test and executed. It runs apicover's own detectors over the
// integration-tagged coverage profile (the CI recipe) rather than
// re-implementing them, and checks the package files are not gitignored.
func TestC1698_010_HelperGraduatesIntoTheApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	pkgDir := filepath.Join(goDir, "internal", "auditledger")
	enrolled := false
	for _, line := range strings.Split(readFile(t, filepath.Join(goDir, ".apicover-enforce")), "\n") {
		if strings.TrimSpace(line) == "./internal/auditledger" {
			enrolled = true
		}
	}
	if !enrolled {
		t.Errorf("./internal/auditledger is not a line of go/.apicover-enforce — the new package is outside the repo-wide API gate")
	}

	ctx := context.Background()
	syms, err := apicover.Enumerate(ctx, pkgDir)
	if err != nil {
		t.Fatalf("apicover.Enumerate(auditledger): %v", err)
	}
	have := map[string]bool{}
	for _, s := range syms {
		have[s.Name] = true
	}
	for _, want := range []string{"LatestAuditorEntry", "ErrNoAuditorForRun", "Entry"} {
		if !have[want] {
			t.Errorf("internal/auditledger does not export %s", want)
		}
	}
	for _, s := range apicover.MissingDoc(syms) {
		t.Errorf("export %s (%s:%d) has no godoc", s.Name, s.File, s.Line)
	}
	named, err := apicover.NamesReferencedInTests(ctx, pkgDir)
	if err != nil {
		t.Fatalf("apicover.NamesReferencedInTests(auditledger): %v", err)
	}
	for _, s := range syms {
		if !s.Ignored && !named[s.Name] {
			t.Errorf("no _test.go in internal/auditledger names %s — an enrolled package's unnamed export hard-fails `make apicover-enforce`", s.Name)
		}
	}

	cov := filepath.Join(t.TempDir(), "coverage.txt")
	if _, errOut, code := runGo(t, "test", "-count=1", "-tags", "integration", "-coverprofile="+cov, "./internal/auditledger"); code != 0 {
		t.Fatalf("go test -coverprofile ./internal/auditledger exited %d:\n%s", code, errOut)
	}
	funcOut, errOut, code := runGo(t, "tool", "cover", "-func="+cov)
	if code != 0 {
		t.Fatalf("go tool cover -func exited %d: %s", code, errOut)
	}
	funcFile := mustWrite(t, filepath.Join(t.TempDir(), "coverage.func.txt"), funcOut)
	var report strings.Builder
	if rc, err := apicover.Run(ctx, apicover.Config{Dirs: []string{pkgDir}, CoverPath: funcFile, Enforce: true}, &report); err != nil || rc != 0 {
		t.Errorf("apicover enforce over internal/auditledger: rc=%d err=%v\n%s", rc, err, report.String())
	}

	files, _ := filepath.Glob(filepath.Join(pkgDir, "*.go"))
	for _, f := range files {
		rel, _ := filepath.Rel(root, f)
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code == 0 {
			t.Errorf("%s is gitignored — it would be dropped at ship", rel)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// -----------------------------------------------------------------------------
// AC7 — the touched packages' own suites stay green.
// -----------------------------------------------------------------------------

// TestC1698_011_TouchedPackageSuitesStayGreen runs each touched package's own
// tests, one package per invocation, narrowed where the package is a heavy
// suite. `-v` output must show at least one PASS: a -run pattern matching
// nothing exits 0 and proves nothing.
func TestC1698_011_TouchedPackageSuitesStayGreen(t *testing.T) {
	runs := []struct {
		name string
		cmd  *exec.Cmd
	}{
		{"auditledger", exec.Command("go", "test", "-count=1", "-v", "-tags", "integration", "./internal/auditledger")},
		{"releasepreflight", exec.Command("go", "test", "-count=1", "-v", "-tags", "integration", "./internal/releasepreflight")},
		{"ship audit binding", exec.Command("go", "test", "-count=1", "-v", "-tags", "integration", "-run", "Audit", "./internal/phases/ship")},
		{"composition snapshot", exec.Command("go", "test", "-count=1", "-v", "-run", "LatestAuditEntry|Composition|RequireReusableAudit", "./cmd/evolve")},
	}
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	for _, r := range runs {
		t.Run(r.name, func(t *testing.T) {
			r.cmd.Dir = goDir
			var stdout, stderr strings.Builder
			r.cmd.Stdout, r.cmd.Stderr = &stdout, &stderr
			argv := strings.Join(r.cmd.Args, " ")
			if err := r.cmd.Run(); err != nil {
				t.Fatalf("%s: %v\n%s\n%s", argv, err, tail(stdout.String(), 60), stderr.String())
			}
			if !strings.Contains(stdout.String(), "--- PASS: ") {
				t.Errorf("%s ran no passing test — a vacuous run proves nothing", argv)
			}
		})
	}
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
