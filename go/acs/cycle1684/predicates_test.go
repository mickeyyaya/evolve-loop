//go:build acs

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

const operatorConfirmEnv = "EVOLVE_OPERATOR_CONFIRM"

var (
	evolveBin      string
	evolveBuildErr error
)

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

func TestC1684_004_LiveLeaseRefusesWithoutForce(t *testing.T) {
	const (
		scope = "acs-1684-live-lease"
		cycle = 1684
	)
	root := seedScope(t, scope, cycle)
	writeLease(t, root, cycle, time.Now())

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

	root := acsassert.RepoRoot(t)
	cli := filepath.Join(root, "go", "cmd", "evolve", "cmd_continuation.go")
	if !acsassert.FileContains(t, cli, "continuation.RequireOperatorAuthority(") {
		t.Errorf("RED: %s never calls continuation.RequireOperatorAuthority — the CLI is gated by a private copy, not the shared helper", cli)
	}
}

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

var staleHeartbeat = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

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

var (
	whoKeys  = []string{"actor", "authorized_by", "released_by", "authority"}
	whenKeys = []string{"released_at", "ts", "timestamp"}
	whyKeys  = []string{"reason", "message"}
)

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

	const probeScope = "acs-1684-explain-groundtruth"
	stdout, stderr, code := runRelease(t, seedScope(t, probeScope, 1684), nil, probeScope)
	ungatedRefused := code != 0
	p1515 := filepath.Join(root, "go", "acs", "regression", "cycle1515", "predicates_test.go")
	drivesOperator := acsassert.FileContains(t, p1515, `"continuation", "release", "-operator"`)
	adjudicated := ungatedRefused && drivesOperator

	deferred, deferralHit := firstMarker(limitations, deferralMarkers)
	adjudicatedClaim, adjudicationHit := firstMarker(limitations, adjudicationMarkers)

	if !adjudicated {
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

var deferralMarkers = []string{
	"rather than edited here",
	"recorded for adjudication",
	"left for adjudication",
	"not edited here",
	"deferred to a future cycle",
	"deferred to the next cycle",
}

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

func TestC1684_008b_MarkerVocabularyIsCollisionFree(t *testing.T) {
	for _, d := range deferralMarkers {
		for _, a := range adjudicationMarkers {
			if strings.Contains(d, a) {
				t.Errorf("RED: adjudication marker %q is a substring of deferral marker %q — the false claim would satisfy its own remedy", a, d)
			}
		}
	}
}

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

func firstMarker(text string, markers []string) (string, bool) {
	lower := strings.ToLower(text)
	for _, m := range markers {
		if strings.Contains(lower, strings.ToLower(m)) {
			return m, true
		}
	}
	return "", false
}
