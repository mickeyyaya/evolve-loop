//go:build acs

package cycle1888

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const brokerPkg = "./internal/agentmsg/broker"

func goTest(t *testing.T, pkg string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"test", "-count=1", "-v"}, append(args, pkg)...)...)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go test %s %v failed: %v\n%s", pkg, args, err, out.String())
	}
	return out.String()
}

func requirePassed(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass", name)
		}
	}
}

func specSection(t *testing.T, heading, next string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "agent-messaging.md"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	doc := string(raw)
	begin := strings.Index(doc, heading)
	if begin < 0 {
		t.Fatalf("spec has no heading %q", heading)
	}
	end := strings.Index(doc[begin:], next)
	if end < 0 {
		t.Fatalf("spec has no heading %q after %q", next, heading)
	}
	return doc[begin : begin+end]
}

func linesMatchingAll(section string, patterns ...string) []string {
	var hits []string
	for _, line := range strings.Split(section, "\n") {
		all := true
		for _, p := range patterns {
			if !regexp.MustCompile("(?i)" + p).MatchString(line) {
				all = false
				break
			}
		}
		if all {
			hits = append(hits, line)
		}
	}
	return hits
}

func TestC1888_001_AnAckOrReadByOneDispatchLeavesAnotherQueueUnchanged(t *testing.T) {
	out := goTest(t, brokerPkg, "-run", "^TestAck_")
	requirePassed(t, out,
		"TestAck_CrossQueueIndependence",
		"TestAck_CrossQueueIndependence/an_ack_to_the_head",
		"TestAck_CrossQueueIndependence/a_read_from_the_start",
		"TestAck_CrossQueueIndependence/a_read_from_the_head",
		"TestAck_ClearsTheQueueWithoutARead",
	)
}

func TestC1888_002_AGapEmitsOneDeadEventAndStaysInEveryMailbox(t *testing.T) {
	out := goTest(t, brokerPkg, "-run", "^TestRead_(GapEventsAreDeduplicatedOnReRead|AGapFromTheStoreIsReportedAndNeverSilent)$")
	requirePassed(t, out,
		"TestRead_GapEventsAreDeduplicatedOnReRead",
		"TestRead_AGapFromTheStoreIsReportedAndNeverSilent",
	)
}

// acs-predicate: config-check — the criterion IS the documentation contract: §4 must name the planned entries that §10 already dead-letters at the seal.
func TestC1888_003_TheSealRuleOfSection4NamesPlannedEntriesLikeSection10(t *testing.T) {
	identity := specSection(t, "### 4. Identity", "### 5.")
	if hits := linesMatchingAll(identity, `\bseal\b`, `\bplanned\b`, `dead letters`); len(hits) == 0 {
		t.Errorf("§4 has no seal line that moves the queues of planned entries to the dead letters")
	}
	if hits := linesMatchingAll(identity, `\bseal\b`, `\bpast\b`, `dead letters`); len(hits) == 0 {
		t.Errorf("§4 lost the seal rule for past entries")
	}
	semantics := specSection(t, "### 10. Delivery semantics", "### 11.")
	if hits := linesMatchingAll(semantics, `\bseal\b`, `did not run|planned`); len(hits) == 0 {
		t.Errorf("§10 lost the seal rule for a phase that did not run")
	}
}

// acs-predicate: config-check — the criterion IS the documentation contract; the behaviour it documents is pinned by the named broker test.
func TestC1888_004_APlannedPhaseMessageIsDocumentedAtLeastOnceForARetry(t *testing.T) {
	out := goTest(t, brokerPkg, "-run", "^TestRead_AMessageToAPlannedPhaseReachesARetryAgain$")
	requirePassed(t, out, "TestRead_AMessageToAPlannedPhaseReachesARetryAgain")
	grammar := specSection(t, "### 3. The address grammar", "### 4.")
	if hits := linesMatchingAll(grammar, `\bplanned\b`, `\bretr(y|ies)\b`, `at[- ]least[- ]once`); len(hits) == 0 {
		t.Errorf("§3 does not say that a message to a planned phase reaches its retry again (at least once)")
	}
	if hits := linesMatchingAll(grammar, `retried phase gets one copy`); len(hits) != 0 {
		t.Errorf("§3 still claims a retried phase gets one copy: %q", hits)
	}
	semantics := specSection(t, "### 10. Delivery semantics", "### 11.")
	if hits := linesMatchingAll(semantics, `\bplanned\b`, `\bretr(y|ies)\b`); len(hits) == 0 {
		t.Errorf("§10 does not name the planned-phase retry redelivery")
	}
}

// acs-predicate: config-check — the criterion IS the documentation contract; the behaviour it documents is pinned by the named broker test.
func TestC1888_005_AReadFromTheHeadIsDocumentedToClearTheQueue(t *testing.T) {
	out := goTest(t, brokerPkg, "-run", "^TestRead_FromTheHeadClearsTheQueueWithoutMessages$")
	requirePassed(t, out, "TestRead_FromTheHeadClearsTheQueueWithoutMessages")
	flow := specSection(t, "### 6. The message flow", "### 7.")
	if hits := linesMatchingAll(flow, `never empties the queue`); len(hits) != 0 {
		t.Errorf("§6 step 7 still claims a caller cursor never empties the queue: %q", hits)
	}
	if hits := linesMatchingAll(flow, `^7\. `, `\bhead\b`); len(hits) == 0 {
		t.Errorf("§6 step 7 does not say that a read from the head clears the queue")
	}
	threats := specSection(t, "### 15. The threat model", "### 16.")
	t12 := linesMatchingAll(threats, `^\| T12 \|`)
	if len(t12) != 1 {
		t.Fatalf("§15 has %d T12 rows, want 1", len(t12))
	}
	if strings.Contains(t12[0], "moves only to the cursor that a read returns") {
		t.Errorf("T12 still claims the queue moves only to the returned cursor: %q", t12[0])
	}
	if !regexp.MustCompile(`(?i)\bhead\b.*\b(clear|empt)`).MatchString(t12[0]) {
		t.Errorf("T12 does not say that a read or ack at the head clears the queue: %q", t12[0])
	}
}

func TestC1888_006_TheChangedBrokerFilesCarryNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{"service.go", "read_test.go"} {
		path := filepath.Join(root, "go", "internal", "agentmsg", "broker", rel)
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		if len(f.Comments) != 0 {
			t.Errorf("%s carries %d comment groups, want 0", rel, len(f.Comments))
		}
	}
}

func TestC1888_007_TheWholeBrokerPackageStaysGreen(t *testing.T) {
	out := goTest(t, brokerPkg)
	if strings.Contains(out, "--- FAIL") || !strings.Contains(out, "--- PASS: TestRead_OnlyARecipientSeesAMessage ") {
		t.Errorf("broker package did not run green:\n%s", out)
	}
}
