//go:build acs

package cycle2496

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
	"github.com/mickeyyaya/evolve-loop/go/internal/redact"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	incidentID       = "2026-11-18-fixture-incident"
	githubToken      = "ghp_A1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q7r8"
	kvSecretValue    = "hunter2hunter2"
	randomToken      = "Zq8Xv3Lp0Rt7Ym2Kc9Wd4Hs6Jb1Nf5Ga8Ue3OiPx"
	commitSHA        = "3f786850e387550fdab836ed7e6dc881de23001b"
	sha256Digest     = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	templateUpstream = "ERROR cannot reach upstream host db-primary"
	templateLock     = "ERROR migration lock held by stale worker"
	templateQuota    = "ERROR disk quota exceeded on volume scratch"
	sourceBytes      = 2000
	tightBudgetBytes = 5600
)

var vocabulary = []string{
	"amber", "birch", "cedar", "delta", "ember", "fjord", "grove", "harbor", "island", "jungle",
	"kettle", "lantern", "meadow", "nectar", "orchid", "pebble", "quarry", "ridge", "summit", "timber",
	"umbra", "valley", "willow", "xenon", "yonder", "zephyr", "anchor", "bridge", "canyon", "dune",
	"estuary", "forest", "glacier", "hollow", "inlet", "juniper", "kelp", "lagoon", "mesa", "nimbus",
}

type project struct {
	t    *testing.T
	bin  string
	root string
}

func newProject(t *testing.T, budgetBytes int) project {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	_, buildErr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "build", "-o", bin, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go build ./cmd/evolve: code=%d err=%v\n%s", code, err, acsassert.OutputTail(buildErr, 30))
	}
	root := t.TempDir()
	if budgetBytes > 0 {
		policy := fmt.Sprintf(`{"incident":{"evidence":{"budget_bytes":%d}}}`, budgetBytes)
		writeFile(t, filepath.Join(root, ".evolve", "policy.json"), policy)
	}
	return project{t: t, bin: bin, root: root}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	parent := filepath.Dir(path)
	if mkErr := os.MkdirAll(parent, 0o750); mkErr != nil {
		t.Fatalf("create %s: %v", parent, mkErr)
	}
	if wrErr := os.WriteFile(path, []byte(content), 0o600); wrErr != nil {
		t.Fatalf("write %s: %v", path, wrErr)
	}
}

func (p project) issue(args ...string) (string, int) {
	p.t.Helper()
	full := append([]string{"issue"}, args...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(p.bin, full...)
	return stdout + stderr, code
}

func (p project) addSource(id, name, content string) {
	p.t.Helper()
	source := filepath.Join(p.t.TempDir(), name)
	writeFile(p.t, source, content)
	out, code := p.issue("evidence", "add", "--project-root", p.root, id, source)
	if code != 0 {
		p.t.Fatalf("evolve issue evidence add %s %s: exit %d\n%s", id, name, code, acsassert.OutputTail(out, 20))
	}
}

func (p project) check(id string) (string, int) {
	p.t.Helper()
	return p.issue("check", "--project-root", p.root, id)
}

func (p project) bundleDir(id string) string {
	return filepath.Join(p.root, "knowledge-base", "incidents", id)
}

func (p project) bundleFiles(id string) map[string]string {
	p.t.Helper()
	files := map[string]string{}
	dir := p.bundleDir(id)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(dir, path)
		files[filepath.ToSlash(rel)] = string(body)
		return nil
	})
	if err != nil {
		p.t.Fatalf("bundle %s is not readable: %v", dir, err)
	}
	if _, ok := files["manifest.json"]; !ok {
		p.t.Fatalf("bundle %s has no manifest.json; files: %v", dir, keys(files))
	}
	return files
}

func (p project) manifest(id string) any {
	p.t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(p.bundleFiles(id)["manifest.json"]), &doc); err != nil {
		p.t.Fatalf("manifest.json is not JSON: %v", err)
	}
	return doc
}

func keys(m map[string]string) []string {
	return slices.Sorted(maps.Keys(m))
}

func copiedContent(files map[string]string) (joined string, total int) {
	var parts []string
	for name, body := range files {
		if name == "manifest.json" {
			continue
		}
		parts = append(parts, body)
		total += len(body)
	}
	return strings.Join(parts, "\n"), total
}

func digest(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

func anyObject(v any, match func(map[string]any) bool) bool {
	switch x := v.(type) {
	case map[string]any:
		if match(x) {
			return true
		}
		for _, child := range x {
			if anyObject(child, match) {
				return true
			}
		}
	case []any:
		for _, child := range x {
			if anyObject(child, match) {
				return true
			}
		}
	}
	return false
}

func holdsString(obj map[string]any, match func(string) bool) bool {
	for _, v := range obj {
		if s, ok := v.(string); ok && match(s) {
			return true
		}
	}
	return false
}

func hasPointerEntryFor(manifest any, rawSource string) bool {
	want := digest(rawSource)
	return anyObject(manifest, func(obj map[string]any) bool {
		return holdsString(obj, func(s string) bool { return s == "pointer" }) &&
			holdsString(obj, func(s string) bool { return strings.HasSuffix(s, want) })
	})
}

func hasPointerEntry(manifest any) bool {
	return anyObject(manifest, func(obj map[string]any) bool {
		return holdsString(obj, func(s string) bool { return s == "pointer" })
	})
}

func recordsRedactions(manifest any) bool {
	return anyObject(manifest, func(obj map[string]any) bool {
		switch r := obj["redactions"].(type) {
		case float64:
			return r > 0
		case []any:
			return len(r) > 0
		case map[string]any:
			return len(r) > 0
		}
		return false
	})
}

func logSource(marker string, seed int, errorStems ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "2026-11-18T01:00:00Z INFO marker %s\n", marker)
	for i, stem := range errorStems {
		fmt.Fprintf(&b, "2026-11-18T01:00:%02dZ %s attempt %d\n", i+1, stem, i+1)
	}
	seedWord := vocabulary[(seed*9)%len(vocabulary)]
	for i := 0; b.Len() < sourceBytes; i++ {
		n := i + seed*13
		fmt.Fprintf(&b, "2026-11-18T02:%02d:%02dZ INFO %s %s %s %s\n", i%60, n%60,
			seedWord, vocabulary[n%len(vocabulary)], vocabulary[(n*7+3)%len(vocabulary)], vocabulary[(n*11+5)%len(vocabulary)])
	}
	return b.String()
}

var (
	repeatSourceOldest = logSource("S1-ONLY-MARKER", 1, templateUpstream)
	repeatSourceMiddle = logSource("S2-ONLY-MARKER", 2, templateUpstream, templateLock)
	newestSource       = logSource("S3-ONLY-MARKER", 3, templateQuota)
)

func TestC2496_001_AChangedBundleFileFailsIssueCheck(t *testing.T) {
	p := newProject(t, 0)
	clean := logSource("S1-ONLY-MARKER", 1, templateUpstream) + "2026-11-18T03:00:00Z INFO deployed commit " + commitSHA + "\n"
	p.addSource(incidentID, "run.log", clean)
	if out, code := p.check(incidentID); code != 0 {
		t.Fatalf("a freshly built bundle must pass issue check: exit %d\n%s", code, acsassert.OutputTail(out, 20))
	}
	changed := 0
	for name := range p.bundleFiles(incidentID) {
		if name == "manifest.json" {
			continue
		}
		path := filepath.Join(p.bundleDir(incidentID), filepath.FromSlash(name))
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString("\n2026-11-18T09:99:99Z INFO a line nobody captured\n"); err != nil {
			t.Fatal(err)
		}
		f.Close()
		changed++
	}
	if changed == 0 {
		t.Fatalf("the bundle holds no copied file besides manifest.json: %v", keys(p.bundleFiles(incidentID)))
	}
	if out, code := p.check(incidentID); code == 0 {
		t.Errorf("issue check passed a bundle whose copied file changed after capture\n%s", acsassert.OutputTail(out, 20))
	}
}

func TestC2496_002_ATokenInASourceIsRedactedInTheCopyAndTheManifestKeepsTheRawDigest(t *testing.T) {
	p := newProject(t, 0)
	raw := strings.Join([]string{
		"2026-11-18T01:02:03Z INFO deploy started for build " + commitSHA,
		"2026-11-18T01:02:04Z ERROR upstream rejected the request: connection refused",
		"2026-11-18T01:02:05Z DEBUG using key " + githubToken + " for the push",
		"2026-11-18T01:02:06Z DEBUG api_key=" + kvSecretValue + " loaded",
		"",
	}, "\n")
	p.addSource(incidentID, "deploy.log", raw)
	files := p.bundleFiles(incidentID)
	everything := strings.Join(keysValues(files), "\n")
	for _, secret := range []string{githubToken, kvSecretValue} {
		if strings.Contains(everything, secret) {
			t.Errorf("the bundle still holds the secret %q", secret)
		}
	}
	copied, _ := copiedContent(files)
	for _, kept := range []string{"[REDACTED]", "connection refused", commitSHA} {
		if !strings.Contains(copied, kept) {
			t.Errorf("the redacted copy lost %q; redaction must only replace the secrets", kept)
		}
	}
	manifest := p.manifest(incidentID)
	if !anyObject(manifest, func(obj map[string]any) bool {
		return holdsString(obj, func(s string) bool { return strings.HasSuffix(s, digest(raw)) })
	}) {
		t.Errorf("the manifest holds no digest of the raw source (%s)", digest(raw))
	}
	if !recordsRedactions(manifest) {
		t.Errorf("the manifest does not record a non-zero redactions field")
	}
	if out, code := p.check(incidentID); code != 0 {
		t.Errorf("a redacted bundle must pass issue check: exit %d\n%s", code, acsassert.OutputTail(out, 20))
	}
}

func keysValues(m map[string]string) []string {
	return slices.Collect(maps.Values(m))
}

func TestC2496_003_AnOverBudgetBundleKeepsEveryErrorTemplateAndPointersTheOldestRepeatSource(t *testing.T) {
	p := newProject(t, tightBudgetBytes)
	p.addSource(incidentID, "s1.log", repeatSourceOldest)
	p.addSource(incidentID, "s2.log", repeatSourceMiddle)
	p.addSource(incidentID, "s3.log", newestSource)
	files := p.bundleFiles(incidentID)
	copied, total := copiedContent(files)
	if total > tightBudgetBytes {
		t.Errorf("the copied sources hold %d bytes, over the budget of %d", total, tightBudgetBytes)
	}
	for _, template := range []string{templateUpstream, templateLock, templateQuota} {
		if !strings.Contains(copied, template) {
			t.Errorf("the over-budget bundle lost the error template %q", template)
		}
	}
	manifest := p.manifest(incidentID)
	if !hasPointerEntryFor(manifest, repeatSourceOldest) {
		t.Errorf("the oldest repeat source (digest %s) is not a pointer entry in the manifest", digest(repeatSourceOldest))
	}
	if strings.Contains(copied, "S1-ONLY-MARKER") {
		t.Errorf("the oldest repeat source still has its content in the bundle")
	}
	for _, marker := range []string{"S2-ONLY-MARKER", "S3-ONLY-MARKER"} {
		if !strings.Contains(copied, marker) {
			t.Errorf("the source with %s must stay in full", marker)
		}
	}
	for _, kept := range []string{repeatSourceMiddle, newestSource} {
		if hasPointerEntryFor(manifest, kept) {
			t.Errorf("a source that fits must not become a pointer (digest %s)", digest(kept))
		}
	}
	if out, code := p.check(incidentID); code != 0 {
		t.Errorf("a bundle with a pointer entry must pass issue check: exit %d\n%s", code, acsassert.OutputTail(out, 20))
	}
}

func TestC2496_004_ABundleWithinTheBudgetKeepsEverySourceInFull(t *testing.T) {
	p := newProject(t, 0)
	p.addSource(incidentID, "s1.log", repeatSourceOldest)
	p.addSource(incidentID, "s2.log", repeatSourceMiddle)
	p.addSource(incidentID, "s3.log", newestSource)
	copied, _ := copiedContent(p.bundleFiles(incidentID))
	for _, marker := range []string{"S1-ONLY-MARKER", "S2-ONLY-MARKER", "S3-ONLY-MARKER"} {
		if !strings.Contains(copied, marker) {
			t.Errorf("a bundle within budget lost the source with %s", marker)
		}
	}
	if hasPointerEntry(p.manifest(incidentID)) {
		t.Errorf("a bundle within budget must hold no pointer entry")
	}
}

func TestC2496_005_AValidBundlePassesTheEntropyRuleAndARandomTokenInALogLineFailsIt(t *testing.T) {
	p := newProject(t, 0)
	validID, tokenID := incidentID+"-valid", incidentID+"-token"
	valid := "2026-11-18T01:00:00Z INFO built " + commitSHA + " digest " + sha256Digest + "\n" + logSource("V-MARKER", 4, templateLock)
	p.addSource(validID, "valid.log", valid)
	if out, code := p.check(validID); code != 0 {
		t.Errorf("a bundle with a 40-hex commit and a 64-hex digest must pass the entropy rule: exit %d\n%s", code, acsassert.OutputTail(out, 20))
	}
	withToken := "2026-11-18T01:00:00Z INFO session handle " + randomToken + " accepted\n" + logSource("T-MARKER", 5, templateLock)
	p.addSource(tokenID, "token.log", withToken)
	tokenKept := strings.Contains(strings.Join(keysValues(p.bundleFiles(tokenID)), "\n"), randomToken)
	out, code := p.check(tokenID)
	if tokenKept && code == 0 {
		t.Errorf("issue check passed a bundle that holds a random 40-character token\n%s", acsassert.OutputTail(out, 20))
	}
	if !tokenKept && code != 0 {
		t.Errorf("the token is gone from the bundle, yet issue check failed: exit %d\n%s", code, acsassert.OutputTail(out, 20))
	}
}

func TestC2496_006_TheRedactLeafRedactsSecretsAndPanetrustSharesItsPatternList(t *testing.T) {
	corpus := []string{
		"push with " + githubToken + " done",
		"api_key=" + kvSecretValue + " loaded",
		"Authorization: Bearer abc123def456",
		"key sk-livesecret0123456789ABCDEF end",
		"-----BEGIN RSA PRIVATE KEY-----",
		"no secrets here at all",
		"commit " + commitSHA + " built",
	}
	for _, line := range corpus {
		if got, want := panetrust.RedactSecrets(line), redact.Secrets(line); got != want {
			t.Errorf("panetrust and redact disagree on %q:\n panetrust: %q\n redact:    %q", line, got, want)
		}
	}
	for _, secret := range []string{githubToken, kvSecretValue, "sk-livesecret0123456789ABCDEF"} {
		if got := redact.Secrets(strings.Join(corpus, "\n")); strings.Contains(got, secret) {
			t.Errorf("redact.Secrets left %q in place", secret)
		}
	}
	if got := redact.Secrets("no secrets here at all"); got != "no secrets here at all" {
		t.Errorf("redact.Secrets changed clean text: %q", got)
	}
	if got := redact.Secrets("push with " + githubToken + " done"); !strings.Contains(got, "[REDACTED]") {
		t.Errorf("redact.Secrets must mark the redaction, got %q", got)
	}
}

func TestC2496_007_TheEntropyRuleFlagsARandomBase64RunAndExemptsHexDigestsAndShortOrFlatRuns(t *testing.T) {
	line := func(run string) string { return "2026-11-18T01:00:00Z INFO session handle " + run + " accepted" }
	flagged := redact.EntropyRuns(line(randomToken))
	if len(flagged) != 1 || flagged[0] != randomToken {
		t.Errorf("a random 40-character token must be the one flagged run, got %q", flagged)
	}
	for name, run := range map[string]string{
		"40-hex commit":      commitSHA,
		"40-hex uniform":     "0123456789abcdef0123456789abcdef01234567",
		"64-hex digest":      sha256Digest,
		"31-character token": randomToken[:31],
		"flat run":           strings.Repeat("a", 40),
		"two-symbol run":     strings.Repeat("ab", 20),
	} {
		if got := redact.EntropyRuns(line(run)); len(got) != 0 {
			t.Errorf("%s must pass the entropy rule, got %q", name, got)
		}
	}
	if got := redact.EntropyRuns(strings.Join([]string{"plain log line", "ERROR cannot reach upstream host db-primary"}, "\n")); len(got) != 0 {
		t.Errorf("ordinary log text must pass the entropy rule, got %q", got)
	}
}

func TestC2496_008_RedactIsAnInternalFreeLeafThatPanetrustImports(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	list := func(pkg string) string {
		out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "list", "-f", `{{join .Imports "\n"}}`, pkg)
		if err != nil || code != 0 {
			t.Fatalf("go list %s: code=%d err=%v\n%s", pkg, code, err, acsassert.OutputTail(errOut, 20))
		}
		return out
	}
	const internalPrefix = "github.com/mickeyyaya/evolve-loop/go/internal/"
	for _, imp := range strings.Fields(list("./internal/redact")) {
		if strings.HasPrefix(imp, internalPrefix) {
			t.Errorf("internal/redact must be a leaf, it imports %s", imp)
		}
	}
	if !strings.Contains(list("./internal/panetrust"), internalPrefix+"redact") {
		t.Errorf("internal/panetrust must call internal/redact")
	}
}

// acs-predicate: config-check
func TestC2496_009_RedactIsEnrolledInTheRepoWideApicoverGateWithItsNamedTest(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforce := filepath.Join(root, "go", ".apicover-enforce")
	if !acsassert.LineContainsAll(enforce, "./internal/redact") {
		t.Errorf("go/.apicover-enforce must list ./internal/redact")
	}
	named := filepath.Join("go", "internal", "redact", "apicover_named_test.go")
	if !acsassert.FileExists(t, filepath.Join(root, named)) {
		return
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", named); code != 0 {
		t.Errorf("%s is untracked and would be dropped at ship", named)
	}
}
