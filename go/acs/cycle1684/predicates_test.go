//go:build acs

// Package cycle1684 materialises the acceptance criteria of the one
// fleet-scoped task pinned to this lane: `continuation-release-cli-authority-gate`.
//
// WHAT THE CONTRACT IS. The inbox record
// (.evolve/inbox/2026-08-18T17-30-00Z-continuation-release-cli-authority-gate.json)
// states the fix in one sentence:
//
//	"Gate the release subcommand like its siblings: require -operator (or
//	 EVOLVE_OPERATOR_CONFIRM=1), refuse when a live cycle lease exists for the
//	 scope's lane unless -force, and log the release into the ledger
//	 (who/when/why) so lineage erasure is itself evidenced."
//
// Today `evolve continuation release <scope-id>` (runContinuationRelease,
// go/cmd/evolve/cmd_continuation.go:104-144) has NO authority gate at all: its
// FlagSet declares only -project-root, and the function drops straight from
// arg-parsing to an unconditional inboxmover.ReleaseContinuationBinding with a
// hardcoded "operator-release" reason that names no actual caller. Any
// Bash-capable process — an in-cycle agent included — can drop a live scope's
// binding, which silently widens the registry's ORCHESTRATOR-side-only
// authority invariant (ADR-0085/0089, cycle-1285 anti-tamper) and erases the
// lineage the defect-ledger gate depends on.
//
// WHY THESE PREDICATES DRIVE THE BINARY. runContinuationRelease lives in
// package main, so no test can import it; the ONLY way to prove the gate is
// reached from the production entry point (rather than sitting in dead code a
// test calls directly) is to build ./cmd/evolve once in TestMain and drive the
// real CLI. 001-005 do exactly that; 006 additionally calls the authority
// helper in-process, because criterion 5 is specifically that a direct package
// caller cannot BYPASS the gate the CLI goes through.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - NEGATIVE : 001 (ungated refuses, binding intact, nothing recorded) and
//     004 (a LIVE lease refuses even for a fully authorized operator). 004 is
//     the load-bearing discriminator — an implementation that adds only the
//     -operator flag passes 001-003 and fails here.
//   - EDGE/OOD : 003b drives EVOLVE_OPERATOR_CONFIRM=0, which a sloppy
//     os.Getenv(...) != "" gate accepts; 005b drives a lease whose heartbeat
//     has aged past runlease.DefaultTTL, which must NOT block — otherwise every
//     dead cycle's leftover lease bricks its scope forever (over-correction is
//     as much a defect as the gap).
//   - SEMANTIC : 002/005a do not merely assert "a release happened"; they
//     demand the durable record answer WHO (a named authority field), WHEN (a
//     parseable RFC3339 stamp) and WHY (a reason), and that a -force release be
//     distinguishable from an ordinary one. A constant reason string for every
//     path fails 005a.
//
// The record's HOME is deliberately not pinned. The inbox record says "the
// ledger"; the run ledger (.evolve/ledger.jsonl, already reachable from
// inboxmover.Options.Ledger) and the released_continuations[] annotation on the
// scope's inbox item are both durable and both defensible, and fault
// localization named the second while the item text reads as the first. The
// criterion is about the record's CONTENT, so releaseRecords scans both homes
// and a hit in either satisfies it. test-report.md records this reading.
package cycle1684

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// operatorConfirmEnv is the env gate the inbox record names, spelled once.
const operatorConfirmEnv = "EVOLVE_OPERATOR_CONFIRM"

var (
	evolveBin      string
	evolveBuildErr error
)

// TestMain builds ./cmd/evolve ONCE. The binary under test must be the real
// one, built from this lane's worktree — a stub or the operator's installed
// evolve would prove nothing about the diff this cycle ships.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1684-")
	switch {
	case err != nil:
		evolveBuildErr = fmt.Errorf("tempdir: %w", err)
	default:
		evolveBin = filepath.Join(dir, "evolve")
		root, rerr := repoRootFromCwd()
		if rerr != nil {
			evolveBuildErr = rerr
		} else if out, berr := exec.Command("go", "-C", filepath.Join(root, "go"), "build", "-o", evolveBin, "./cmd/evolve").CombinedOutput(); berr != nil {
			evolveBuildErr = fmt.Errorf("go build ./cmd/evolve: %v\n%s", berr, out)
		}
	}
	code := m.Run()
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
	os.Exit(code)
}

// repoRootFromCwd mirrors acsassert.RepoRoot for TestMain, which has no
// *testing.T yet. git is invoked with -C so the repo resolves from the test's
// directory and not from whatever cwd a fleet lane happens to hold.
func repoRootFromCwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git -C %s rev-parse --show-toplevel: %v", cwd, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ---------------------------------------------------------------- predicates

// TestC1684_001_UngatedReleaseRefusesAndLeavesBindingIntact pins criterion 1.
// NEGATIVE: the invocation that must FAIL. A refusal is three things at once —
// a non-zero exit, an untouched binding, and guidance naming BOTH authority
// paths — and a refusal that still writes a release record would evidence a
// lineage erasure that never happened.
func TestC1684_001_UngatedReleaseRefusesAndLeavesBindingIntact(t *testing.T) {
	const scope = "acs-1684-ungated"
	root := seedScope(t, scope, 1684)

	stdout, stderr, code := runRelease(t, root, nil, scope)
	if code == 0 {
		t.Errorf("RED: ungated release exited 0 — want a non-zero refusal\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	if !bound(t, root, scope) {
		t.Errorf("RED: ungated release DELETED scope %q's binding — a refused release must leave the registry untouched", scope)
	}
	guidance := stdout + stderr
	for _, want := range []string{"-operator", operatorConfirmEnv} {
		if !strings.Contains(guidance, want) {
			t.Errorf("RED: refusal names no %s — criterion 1 requires guidance on how to authorize\ngot: %s", want, guidance)
		}
	}
	if recs := releaseRecords(t, root, scope); len(recs) != 0 {
		t.Errorf("RED: a REFUSED release wrote %d durable release record(s) — nothing was released\n%s", len(recs), dump(recs))
	}
}

// TestC1684_002_OperatorFlagReleasesAndRecordsWhoWhenWhy pins criterion 2 via
// the flag path: authority granted, binding gone, and the erasure evidenced by
// a record that answers who/when/why.
func TestC1684_002_OperatorFlagReleasesAndRecordsWhoWhenWhy(t *testing.T) {
	const scope = "acs-1684-operator-flag"
	root := seedScope(t, scope, 1684)

	stdout, stderr, code := runRelease(t, root, nil, "-operator", scope)
	if code != 0 {
		t.Fatalf("RED: -operator release exited %d — an authorized release must succeed\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if bound(t, root, scope) {
		t.Errorf("RED: -operator release exited 0 but scope %q is still bound — nothing was released", scope)
	}
	assertRecordAnswersWhoWhenWhy(t, root, scope)
}

// TestC1684_003_OperatorConfirmEnvGatesRelease pins criterion 2's env half and
// its OOD edge. Sub-case (a) is the documented affirmative value; sub-case (b)
// drives "0", which the record does NOT authorize — a gate written as
// os.Getenv(operatorConfirmEnv) != "" reads it as consent and fails here.
func TestC1684_003_OperatorConfirmEnvGatesRelease(t *testing.T) {
	t.Run("affirmative_value_releases", func(t *testing.T) {
		const scope = "acs-1684-env-on"
		root := seedScope(t, scope, 1684)

		stdout, stderr, code := runRelease(t, root, []string{operatorConfirmEnv + "=1"}, scope)
		if code != 0 {
			t.Fatalf("RED: %s=1 release exited %d — want success\nstdout: %s\nstderr: %s", operatorConfirmEnv, code, stdout, stderr)
		}
		if bound(t, root, scope) {
			t.Errorf("RED: %s=1 exited 0 but scope %q is still bound", operatorConfirmEnv, scope)
		}
		// The env path must evidence the erasure exactly as the flag path does;
		// without this the sub-case is trivially green against the ungated tree,
		// where every release "succeeds".
		assertRecordAnswersWhoWhenWhy(t, root, scope)
	})

	t.Run("non_affirmative_value_refuses", func(t *testing.T) {
		const scope = "acs-1684-env-off"
		root := seedScope(t, scope, 1684)

		stdout, stderr, code := runRelease(t, root, []string{operatorConfirmEnv + "=0"}, scope)
		if code == 0 {
			t.Errorf("RED: %s=0 released the binding — a set-but-negative value is not authority\nstdout: %s\nstderr: %s", operatorConfirmEnv, stdout, stderr)
		}
		if !bound(t, root, scope) {
			t.Errorf("RED: %s=0 DELETED scope %q's binding", operatorConfirmEnv, scope)
		}
	})
}

// TestC1684_004_LiveLeaseRefusesWithoutForce pins criterion 3's refusal half,
// and is this contract's load-bearing discriminator: the operator here is FULLY
// authorized (-operator is passed), so the only thing that may stop the release
// is the live lease the scope's own cycle holds. An implementation that adds
// the flag gate and stops passes 001-003 and fails exactly here.
func TestC1684_004_LiveLeaseRefusesWithoutForce(t *testing.T) {
	const (
		scope = "acs-1684-live-lease"
		cycle = 1684
	)
	root := seedScope(t, scope, cycle)
	writeLease(t, root, cycle, time.Now()) // fresh heartbeat: the lane is alive

	stdout, stderr, code := runRelease(t, root, nil, "-operator", scope)
	if code == 0 {
		t.Errorf("RED: authorized release dropped a binding whose cycle %d lease is LIVE — criterion 3 requires -force for that\nstdout: %s\nstderr: %s", cycle, stdout, stderr)
	}
	if !bound(t, root, scope) {
		t.Errorf("RED: scope %q's binding was deleted while cycle %d still holds a live lease", scope, cycle)
	}
	guidance := stdout + stderr
	if !strings.Contains(guidance, "-force") {
		t.Errorf("RED: live-lease refusal never names -force — the operator is told to stop with no way forward\ngot: %s", guidance)
	}
	// The refusal must be ABOUT the lease. Without this the predicate cannot
	// tell a principled live-lease refusal from an unrelated non-zero exit
	// (e.g. an unparsed flag), and would read green on the wrong behaviour.
	//
	// The markers are deliberately collision-free: a bare "lease" is a
	// SUBSTRING OF "release", so the subcommand's own usage banner ("Usage of
	// continuation release:") satisfies it and the check silently never fires —
	// verified against the live tree this phase, which is how this line came to
	// be written this way rather than the obvious way.
	named := strings.Contains(strings.ToLower(guidance), "live")
	for _, form := range []string{
		fmt.Sprintf("cycle %d", cycle),
		fmt.Sprintf("cycle=%d", cycle),
		fmt.Sprintf("cycle-%d", cycle),
	} {
		if strings.Contains(guidance, form) {
			named = true
		}
	}
	if !named {
		t.Errorf("RED: refusal names neither a LIVE lane nor the owning cycle %d — it is not a live-lease refusal\ngot: %s", cycle, guidance)
	}
}

// TestC1684_005_ForceOverridesLiveLeaseAndStaleLeaseDoesNotBlock pins criterion
// 3's override half plus the edge that keeps the gate from over-correcting.
// (a) -force proceeds through a live lease AND the record says so — a constant
// reason string for every path cannot tell an override from a routine release.
// (b) a lease whose heartbeat aged past runlease.DefaultTTL is NOT liveness
// (runlease.Lease documents freshness as the only liveness signal), so it must
// not block; if it did, every dead cycle's leftover .lease would brick its
// scope permanently and the gate would become the new stall.
func TestC1684_005_ForceOverridesLiveLeaseAndStaleLeaseDoesNotBlock(t *testing.T) {
	t.Run("force_overrides_live_lease", func(t *testing.T) {
		const (
			scope = "acs-1684-force"
			cycle = 1684
		)
		root := seedScope(t, scope, cycle)
		writeLease(t, root, cycle, time.Now())

		stdout, stderr, code := runRelease(t, root, nil, "-operator", "-force", scope)
		if code != 0 {
			t.Fatalf("RED: -operator -force exited %d — criterion 3 requires -force to proceed\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if bound(t, root, scope) {
			t.Errorf("RED: -force exited 0 but scope %q is still bound", scope)
		}
		assertRecordAnswersWhoWhenWhy(t, root, scope)

		recs := releaseRecords(t, root, scope)
		noted := false
		for _, r := range recs {
			blob := strings.ToLower(dump([]map[string]any{r}))
			if strings.Contains(blob, "force") || strings.Contains(blob, "override") {
				noted = true
			}
		}
		if !noted {
			t.Errorf("RED: no release record notes the -force override — an overridden live lease is indistinguishable from a routine release\n%s", dump(recs))
		}
	})

	t.Run("stale_lease_does_not_block", func(t *testing.T) {
		const (
			scope = "acs-1684-stale-lease"
			cycle = 1684
		)
		root := seedScope(t, scope, cycle)
		writeLease(t, root, cycle, staleHeartbeat)

		stdout, stderr, code := runRelease(t, root, nil, "-operator", scope)
		if code != 0 {
			t.Errorf("RED: an authorized release was blocked by a lease STALER than runlease.DefaultTTL (%s) with no -force — a dead cycle's leftover lease must not brick its scope\nstdout: %s\nstderr: %s", runlease.DefaultTTL, stdout, stderr)
		}
		if bound(t, root, scope) {
			t.Errorf("RED: stale-lease release did not remove scope %q's binding", scope)
		}
	})
}

// TestC1684_006_AuthorityHelperRefusesInProcessCallers pins criterion 5: the
// check lives in a HELPER both the CLI and any in-process caller reach, so a
// direct package call cannot route around the gate the CLI honours. The
// helper's four-way behaviour is the load-bearing assertion; the call-site
// grep that follows it is auxiliary wiring evidence only — predicates 001-005
// are what actually prove the CLI reaches it, by driving the real binary.
func TestC1684_006_AuthorityHelperRefusesInProcessCallers(t *testing.T) {
	t.Setenv(operatorConfirmEnv, "")
	if err := continuation.RequireOperatorAuthority(false); err == nil {
		t.Errorf("RED: RequireOperatorAuthority(false) with no %s returned nil — an unauthorized in-process caller would bypass the gate", operatorConfirmEnv)
	} else {
		for _, want := range []string{"-operator", operatorConfirmEnv} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("RED: refusal error names no %s — the helper's message is the CLI's guidance text\ngot: %v", want, err)
			}
		}
	}
	if err := continuation.RequireOperatorAuthority(true); err != nil {
		t.Errorf("RED: RequireOperatorAuthority(true) refused an explicitly authorized caller: %v", err)
	}

	t.Setenv(operatorConfirmEnv, "1")
	if err := continuation.RequireOperatorAuthority(false); err != nil {
		t.Errorf("RED: RequireOperatorAuthority(false) refused with %s=1 set: %v", operatorConfirmEnv, err)
	}

	t.Setenv(operatorConfirmEnv, "0")
	if err := continuation.RequireOperatorAuthority(false); err == nil {
		t.Errorf("RED: RequireOperatorAuthority(false) accepted %s=0 — a set-but-negative value is not authority", operatorConfirmEnv)
	}

	// Auxiliary wiring evidence: the production CLI must reach the helper
	// rather than inlining a second copy of the check (the drift that produced
	// audit cycle-1507's H2). Not load-bearing on its own.
	root := acsassert.RepoRoot(t)
	cli := filepath.Join(root, "go", "cmd", "evolve", "cmd_continuation.go")
	if !acsassert.FileContains(t, cli, "continuation.RequireOperatorAuthority(") {
		t.Errorf("RED: %s never calls continuation.RequireOperatorAuthority — the CLI is gated by a private copy, not the shared helper", cli)
	}
}

// TestC1684_007_EditedPackagesVetAndGofmtClean pins criterion 4.
//
// Deliberately NARROWED to the two packages this task edits. The criterion's
// literal text ("go vet ./... and go build ./... clean; full suite green") is a
// repo-wide sweep, which the flaky-predicate-shape rules ban from a cycle
// predicate: a whole-repo go test under fleet load is the false-RED generator
// that failed cycles 1173/1175/1178 on sound work. The repo-wide sweep is the
// build phase's own obligation and CI's `-count=1` job; what a predicate can
// hold honestly is that the packages the diff touches stay vet- and
// gofmt-clean. test-report.md states this narrowing.
func TestC1684_007_EditedPackagesVetAndGofmtClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")

	vet := exec.Command("go", "-C", goDir, "vet", "./cmd/evolve", "./internal/continuation")
	if out, err := vet.CombinedOutput(); err != nil {
		t.Errorf("RED: go vet ./cmd/evolve ./internal/continuation failed: %v\n%s", err, out)
	}

	fmtCmd := exec.Command("gofmt", "-l", filepath.Join(goDir, "cmd", "evolve"), filepath.Join(goDir, "internal", "continuation"))
	out, err := fmtCmd.CombinedOutput()
	if err != nil {
		t.Errorf("RED: gofmt -l failed to run: %v\n%s", err, out)
	}
	if listed := strings.TrimSpace(string(out)); listed != "" {
		t.Errorf("RED: gofmt -l lists unformatted file(s) in the edited packages:\n%s", listed)
	}
}

// ------------------------------------------------------------------- fixtures

// seedScope builds a throwaway project root holding ONE live continuation
// binding for scopeID plus the pending inbox item that binding's salvage
// pointer belongs to. The registry is written through the production writer
// (continuation.WriteRegistryEntry) and the item carries the `id` field
// lifecycle.FindFileByTaskID matches on, so the fixture is the shape the
// runtime actually produces rather than an invented one.
func seedScope(t *testing.T, scopeID string, cycle int) string {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("seed inbox dir: %v", err)
	}
	item, err := json.MarshalIndent(map[string]any{
		"id":         scopeID,
		"kind":       "bug",
		"title":      "cycle-1684 ACS fixture item",
		"created_at": "2026-01-01T00:00:00Z",
	}, "", "  ")
	if err != nil {
		t.Fatalf("seed item: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "2026-01-01T00-00-00Z-"+scopeID+".json"), item, 0o644); err != nil {
		t.Fatalf("seed item file: %v", err)
	}
	if err := continuation.WriteRegistryEntry(root, scopeID, continuation.Continuation{
		Branch:      fmt.Sprintf("cycle-%d", cycle),
		SnapshotSHA: "deadbeef01",
		BaseSHA:     "cafef00d02",
		Cycle:       cycle,
	}); err != nil {
		t.Fatalf("seed registry entry: %v", err)
	}
	return root
}

// staleHeartbeat is a FIXED past instant, not now-minus-DefaultTTL. Deriving a
// stale stamp by subtracting from the wall clock is the shape the
// flaky-predicate lint flags, and rightly: it stays stale only as long as the
// arithmetic holds. A fixed date is stale forever, under any host load and any
// future change to runlease.DefaultTTL.
var staleHeartbeat = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

// writeLease stamps the scope lane's lease with at. OwnerPID is left zero on
// purpose: runlease.Lease documents heartbeat freshness as the ONLY liveness
// signal, and a real PID in a fixture is the stale-artifact shape the
// flaky-predicate rules ban.
func writeLease(t *testing.T, root string, cycle int, at time.Time) {
	t.Helper()
	runDir := paths.RunWorkspace(root, cycle)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("seed run dir: %v", err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01ACSCYCLE1684FIXTURE000"}, at); err != nil {
		t.Fatalf("seed lease: %v", err)
	}
}

// runRelease drives the REAL `evolve continuation release` entry point. Flags
// precede the positional scope id because Go's flag package stops parsing at
// the first non-flag argument. The base environment has every inherited
// EVOLVE_OPERATOR_CONFIRM stripped, so an ambient operator-confirming value in
// a fleet lane's environment can never make the ungated predicates pass.
func runRelease(t *testing.T, root string, env []string, args ...string) (string, string, int) {
	t.Helper()
	if evolveBuildErr != nil {
		t.Fatalf("RED (harness): %v", evolveBuildErr)
	}
	argv := append([]string{"continuation", "release", "-project-root", root}, args...)
	cmd := exec.Command(evolveBin, argv...)
	cmd.Env = append(scrubbedEnv(), env...)
	var sout, serr strings.Builder
	cmd.Stdout, cmd.Stderr = &sout, &serr
	err := cmd.Run()
	switch e := err.(type) {
	case nil:
		return sout.String(), serr.String(), 0
	case *exec.ExitError:
		return sout.String(), serr.String(), e.ExitCode()
	default:
		t.Fatalf("run %s %v: %v", evolveBin, argv, err)
		return "", "", -1
	}
}

func scrubbedEnv() []string {
	base := os.Environ()
	out := make([]string, 0, len(base))
	for _, kv := range base {
		if strings.HasPrefix(kv, operatorConfirmEnv+"=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func bound(t *testing.T, root, scopeID string) bool {
	t.Helper()
	_, ok, err := continuation.ReadRegistryEntry(root, scopeID)
	if err != nil {
		t.Fatalf("registry unreadable at %s: %v", continuation.RegistryPath(root), err)
	}
	return ok
}

// ------------------------------------------------------------- record reading

var (
	whoKeys  = []string{"actor", "authorized_by", "released_by", "authority"}
	whenKeys = []string{"released_at", "ts", "timestamp"}
	whyKeys  = []string{"reason", "message"}
)

// releaseRecords collects every durable record of scopeID's release from BOTH
// homes the contract allows — the append-only run ledger and the
// released_continuations[] annotation on the scope's item. See the package doc
// for why the home is not pinned.
func releaseRecords(t *testing.T, root, scopeID string) []map[string]any {
	t.Helper()
	var out []map[string]any

	if raw, err := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl")); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || !strings.Contains(line, scopeID) {
				continue
			}
			var rec map[string]any
			if json.Unmarshal([]byte(line), &rec) == nil {
				out = append(out, rec)
			}
		}
	}

	_ = filepath.WalkDir(filepath.Join(root, ".evolve", "inbox"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		var doc struct {
			ID       string           `json:"id"`
			Released []map[string]any `json:"released_continuations"`
		}
		if json.Unmarshal(raw, &doc) != nil || doc.ID != scopeID {
			return nil
		}
		out = append(out, doc.Released...)
		return nil
	})
	return out
}

// assertRecordAnswersWhoWhenWhy is criterion 2's real content test: at least
// one durable record must name an authority (who), carry a parseable RFC3339
// stamp (when) and a reason (why). "A release happened" is not the criterion.
func assertRecordAnswersWhoWhenWhy(t *testing.T, root, scopeID string) {
	t.Helper()
	recs := releaseRecords(t, root, scopeID)
	if len(recs) == 0 {
		t.Errorf("RED: release of %q wrote NO durable record — lineage erasure is unevidenced (looked in .evolve/ledger.jsonl and the item's released_continuations[])", scopeID)
		return
	}
	for _, r := range recs {
		who, when, why := lookup(r, whoKeys), lookup(r, whenKeys), lookup(r, whyKeys)
		if who == "" || why == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, when); err != nil {
			continue
		}
		return
	}
	t.Errorf("RED: %d release record(s) for %q, none answering who/when/why (who one of %v, when a parseable RFC3339 in %v, why one of %v)\n%s",
		len(recs), scopeID, whoKeys, whenKeys, whyKeys, dump(recs))
}

// lookup returns the first non-empty value among keys, descending one nested
// object at a time so a ledger entry carrying its payload under "data" is read
// the same as a flat one.
func lookup(rec map[string]any, keys []string) string {
	for _, k := range keys {
		if v, ok := rec[k]; ok {
			if s := strings.TrimSpace(fmt.Sprintf("%v", v)); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	for _, v := range rec {
		if sub, ok := v.(map[string]any); ok {
			if s := lookup(sub, keys); s != "" {
				return s
			}
		}
	}
	return ""
}

func dump(recs []map[string]any) string {
	b, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", recs)
	}
	return string(b)
}

// TestC1684_008_ExplanationLimitationsMatchTheBoundTree pins the defect audit
// round 2 raised as H1: the cycle's REQUIRED explanation document is
// cryptographically bound (diff_sha256 over every changed path, test files
// included — go/internal/explanationdocs/gitio.go:48-53) to a tree its prose
// misdescribes. Its `## Limitations` says cycle 1515's predicate "asserts that
// an ungated release exits 0" and "is recorded for adjudication rather than
// edited here" — but that predicate was re-authored inside this same diff and
// now drives `-operator`. The durable record tells a future reader this cycle
// shipped leaving the superseded contract unadjudicated, which is backwards.
//
// WHY THIS IS NOT A GREP. The load-bearing half EXECUTES the system: it drives
// the real `evolve` binary with no authority and observes whether the gate this
// cycle ships is actually live, exactly as predicate 001 does. That executed
// observation — not a string in a file — is what establishes which of the two
// prose readings is the true one. The document check is then a CONSISTENCY
// assertion between an executed fact and the durable record of it.
//
// It is also not vacuous in either direction: both branches assert. If the
// tree is gated (the expected state, which 001-007 independently pin) the
// document must record the adjudication and must not claim the deferral; if the
// tree were somehow ungated the document must not claim an adjudication that
// did not happen. A document can only satisfy this predicate by describing the
// tree it is bound to. Adding a magic string cannot pass it — the vocabulary
// required is decided by what the binary does.
func TestC1684_008_ExplanationLimitationsMatchTheBoundTree(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath := soleExplanationDoc(t, root)
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("RED: required explanation document unreadable at %s: %v", docPath, err)
	}
	limitations := section(string(raw), "## Limitations")
	if strings.TrimSpace(limitations) == "" {
		t.Fatalf("RED: %s has no `## Limitations` section — the explanation contract requires one", docPath)
	}

	// ---------------------------------------------------------- ground truth
	// (1) Executed: does the superseding contract actually hold in this tree?
	const probeScope = "acs-1684-explain-groundtruth"
	stdout, stderr, code := runRelease(t, seedScope(t, probeScope, 1684), nil, probeScope)
	ungatedRefused := code != 0
	// (2) Auxiliary corroboration only: cycle 1515's release predicate now
	//     reaches that gate through -operator rather than asserting exit 0
	//     without it. Read from disk because running a second ACS package
	//     inside this one is the nested-suite shape the flaky-predicate rules
	//     ban; the EGPS suite already runs cycle1515 for real.
	p1515 := filepath.Join(root, "go", "acs", "regression", "cycle1515", "predicates_test.go")
	drivesOperator := acsassert.FileContains(t, p1515, `"continuation", "release", "-operator"`)
	adjudicated := ungatedRefused && drivesOperator

	deferred, deferralHit := firstMarker(limitations, deferralMarkers)
	adjudicatedClaim, adjudicationHit := firstMarker(limitations, adjudicationMarkers)

	if !adjudicated {
		// The inverse accuracy obligation. Kept so the predicate can never pass
		// by accident against a tree where the gate was reverted.
		if adjudicationHit {
			t.Errorf("RED: %s `## Limitations` claims cycle 1515's predicate was adjudicated here (%q), but this tree does not support it: ungated release exited %d (want non-zero) and cycle1515 drives -operator = %v\nstdout: %s\nstderr: %s",
				docPath, adjudicatedClaim, code, drivesOperator, stdout, stderr)
		}
		return
	}

	if deferralHit {
		t.Errorf("RED: %s `## Limitations` claims cycle 1515's predicate was left unfixed (%q), but this tree PROVES otherwise — the ungated release the document calls a live contract exited %d, and %s drives -operator. The document is SHA-bound to this tree and must describe it.\nsection:\n%s",
			docPath, deferred, code, p1515, limitations)
	}
	if strings.Contains(limitations, "1515") && !adjudicationHit {
		t.Errorf("RED: %s `## Limitations` still discusses cycle 1515's predicate but never records that it WAS adjudicated and re-authored in this cycle. Say so using one of %v.\nsection:\n%s",
			docPath, adjudicationMarkers, limitations)
	}
}

// -------------------------------------------- explanation-document vocabulary

// deferralMarkers assert "this cycle did NOT fix cycle 1515's predicate".
var deferralMarkers = []string{
	"rather than edited here",
	"recorded for adjudication",
	"left for adjudication",
	"not edited here",
	"deferred to a future cycle",
	"deferred to the next cycle",
}

// adjudicationMarkers assert "this cycle DID fix it". No entry here is a
// substring of any deferralMarker, which is checked below rather than assumed:
// a bare "edited here" is a substring of "rather than edited here" and would
// read the false claim as its own remedy — the same collision trap predicate
// 004 documents for "lease" inside "release".
var adjudicationMarkers = []string{
	"re-authored",
	"reauthored",
	"was adjudicated",
	"is adjudicated",
	"adjudicated in this cycle",
	"corrected in this cycle",
	"corrected here",
	"superseded in this cycle",
	"rewritten in this cycle",
	"re-written in this cycle",
}

// TestC1684_008b_MarkerVocabularyIsCollisionFree keeps predicate 008 honest.
// If any adjudication marker were a substring of a deferral marker, the exact
// false sentence H1 names would satisfy the positive check and 008 would read
// green on the defect it exists to catch.
func TestC1684_008b_MarkerVocabularyIsCollisionFree(t *testing.T) {
	for _, d := range deferralMarkers {
		for _, a := range adjudicationMarkers {
			if strings.Contains(d, a) {
				t.Errorf("RED: adjudication marker %q is a substring of deferral marker %q — the false claim would satisfy its own remedy", a, d)
			}
		}
	}
}

// soleExplanationDoc resolves this cycle's published explanation document by
// glob rather than by its ULID filename: the run id is stable across a
// republish, but hardcoding the full name is the stale-artifact shape the
// flaky-predicate rules ban. Exactly one must exist.
func soleExplanationDoc(t *testing.T, root string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1684-*.md"))
	if err != nil {
		t.Fatalf("glob explanation docs: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("RED: want exactly 1 published explanation document for cycle 1684, found %d: %v", len(matches), matches)
	}
	return matches[0]
}

// section returns the body of a markdown H2 section, from its header to the
// next H2 or EOF.
func section(doc, header string) string {
	i := strings.Index(doc, header)
	if i < 0 {
		return ""
	}
	body := doc[i+len(header):]
	if j := strings.Index(body, "\n## "); j >= 0 {
		body = body[:j]
	}
	return body
}

// firstMarker reports the first marker present in text, case-insensitively.
func firstMarker(text string, markers []string) (string, bool) {
	lower := strings.ToLower(text)
	for _, m := range markers {
		if strings.Contains(lower, strings.ToLower(m)) {
			return m, true
		}
	}
	return "", false
}
