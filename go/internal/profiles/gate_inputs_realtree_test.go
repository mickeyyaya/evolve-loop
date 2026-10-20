package profiles

import (
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const (
	worktreeGrant  = "{worktree_path}"
	workspaceGrant = ".evolve/runs/cycle-*"
)

var gateTrustClasses = map[string]bool{"authority": true, "verified": true, "subject": true, "advisory": true, "open-seam": true}

var unlaunchedProfiles = map[string]bool{"orchestrator": true}

var pinnedOpenSeams = []string{
	"{worktree_path}/go/acs/",
	".evolve/runs/cycle-*/failure-digest.json",
}

type gateInput struct {
	gate, path, trust string
}

var backtickedPath = regexp.MustCompile("^`([^`]+)`$")

func parseGateInventory(doc string) []gateInput {
	_, table, found := strings.Cut(doc, "\n## The inventory\n")
	if !found {
		return nil
	}
	table, _, _ = strings.Cut(table, "\n## ")
	var rows []gateInput
	for _, line := range strings.Split(table, "\n") {
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) != 5 || strings.HasPrefix(strings.TrimSpace(cells[0]), "---") || strings.TrimSpace(cells[0]) == "Gate" {
			continue
		}
		input := ""
		if m := backtickedPath.FindStringSubmatch(strings.TrimSpace(cells[1])); m != nil {
			input = m[1]
		}
		rows = append(rows, gateInput{gate: strings.TrimSpace(cells[0]), path: input, trust: strings.TrimSpace(cells[3])})
	}
	return rows
}

func agentWritable(sandboxes map[string]SandboxConfig, input string) []string {
	var writers []string
	for name, sb := range sandboxes {
		if sandboxWrites(sb, input) {
			writers = append(writers, name)
		}
	}
	slices.Sort(writers)
	return writers
}

func sandboxWrites(sb SandboxConfig, p string) bool {
	for _, grant := range sb.WriteSubpaths {
		if grantCovers(grant, p) && !deniedAtRoot(sb, p) {
			return true
		}
	}
	return false
}

func grantCovers(grant, p string) bool {
	grant, p = strings.TrimSuffix(grant, "/"), strings.TrimSuffix(p, "/")
	if p == grant || strings.HasPrefix(p, grant+"/") || strings.HasPrefix(grant, p+"/") {
		return true
	}
	gs, ps := strings.Split(grant, "/"), strings.Split(p, "/")
	if len(ps) < len(gs) {
		return false
	}
	for i, g := range gs {
		if ok, _ := path.Match(g, ps[i]); !ok {
			return false
		}
	}
	return true
}

func deniedAtRoot(sb SandboxConfig, p string) bool {
	if strings.HasPrefix(p, worktreeGrant) || strings.HasPrefix(p, workspaceGrant) {
		return false
	}
	return sb.Denies(p)
}

func realSandboxes(t *testing.T) map[string]SandboxConfig {
	t.Helper()
	loader, names := RealTreeProfiles(t)
	out := map[string]SandboxConfig{}
	for _, name := range names {
		if unlaunchedProfiles[name] {
			continue
		}
		p, err := loader.Get(name)
		if err != nil {
			t.Fatalf("load profile %s: %v", name, err)
		}
		if p.Sandbox != nil {
			out[name] = *p.Sandbox
		}
	}
	if len(out) == 0 {
		t.Fatal("no profile declares a sandbox: the write scopes did not load")
	}
	return out
}

func realGateInventory(t *testing.T) []gateInput {
	t.Helper()
	doc, err := os.ReadFile(filepath.Join(RealProfilesDir(t), "..", "..", "docs", "architecture", "gate-input-inventory.md"))
	if err != nil {
		t.Fatal(err)
	}
	rows := parseGateInventory(string(doc))
	if len(rows) < 20 {
		t.Fatalf("the inventory has %d rows: the table did not parse", len(rows))
	}
	for _, r := range rows {
		if r.path == "" || !gateTrustClasses[r.trust] {
			t.Fatalf("row %q: want one backticked input path and a trust class, got path %q trust %q", r.gate, r.path, r.trust)
		}
	}
	return rows
}

func TestGateInputInventory_NoAgentWritableInputIsAuthority(t *testing.T) {
	sandboxes := realSandboxes(t)
	for _, r := range realGateInventory(t) {
		if writers := agentWritable(sandboxes, r.path); r.trust == "authority" && len(writers) > 0 {
			t.Errorf("%s reads %s as authority, but profiles %v can write it: verify the input or deny the write", r.gate, r.path, writers)
		}
	}
}

func TestGateInputInventory_TheOpenSeamsArePinned(t *testing.T) {
	sandboxes := realSandboxes(t)
	var seams []string
	for _, r := range realGateInventory(t) {
		if r.trust != "open-seam" {
			continue
		}
		seams = append(seams, r.path)
		if len(agentWritable(sandboxes, r.path)) == 0 {
			t.Errorf("open seam %s is no longer agent-writable: reclassify the row and unpin it", r.path)
		}
	}
	if !reflect.DeepEqual(seams, pinnedOpenSeams) {
		t.Errorf("open seams = %v, want %v: a new seam needs a fix, and a closed one leaves the pinned list", seams, pinnedOpenSeams)
	}
}

func TestSandboxWrites_ReadsEachGrantKind(t *testing.T) {
	t.Parallel()
	builder := SandboxConfig{WriteSubpaths: []string{worktreeGrant, workspaceGrant}, DenySubpaths: []string{".evolve/ledger.jsonl"}}
	tdd := SandboxConfig{WriteSubpaths: []string{"{worktree_path}/go/acs/cycle*", workspaceGrant}}
	docs := SandboxConfig{WriteSubpaths: []string{workspaceGrant, "docs/"}, DenySubpaths: []string{"docs/private"}}
	for _, tc := range []struct {
		sb   SandboxConfig
		path string
		want bool
	}{
		{builder, "{worktree_path}/go/.cover-strict", true},
		{builder, ".evolve/runs/cycle-*/failure-digest.json", true},
		{builder, ".evolve/ledger.jsonl", false},
		{docs, "{worktree_path}/go/.cover-strict", false},
		{docs, "docs/architecture/x.md", true},
		{docs, "docs/private/x.md", false},
		{docs, "docsite/x.md", false},
		{docs, ".evolve/cycle-state.json", false},
		{tdd, "{worktree_path}/go/acs/", true},
		{tdd, "{worktree_path}/go/acs/cycle12/predicates_test.go", true},
		{tdd, "{worktree_path}/go/.cover-strict", false},
		{tdd, ".evolve/runs/cycle-*/acs-verdict.json", true},
	} {
		if got := sandboxWrites(tc.sb, tc.path); got != tc.want {
			t.Errorf("sandboxWrites(%v, %q) = %v, want %v", tc.sb.WriteSubpaths, tc.path, got, tc.want)
		}
	}
}

func TestParseGateInventory_ReadsOnlyTheInventoryTable(t *testing.T) {
	t.Parallel()
	doc := "# x\n\n| a | b | c | d | e |\n|---|---|---|---|---|\n| early | `x` | h | authority | c |\n\n## The inventory\n\n| Gate | Input | Writer | Trust | Check |\n|---|---|---|---|---|\n| G1 | `.evolve/ledger.jsonl` | harness | authority | denied |\n| G2 | two words | agent | verified | c |\n\n## Decisions\n\n| G3 | `y` | h | authority | c |\n"
	want := []gateInput{{"G1", ".evolve/ledger.jsonl", "authority"}, {"G2", "", "verified"}}
	if got := parseGateInventory(doc); !reflect.DeepEqual(got, want) {
		t.Errorf("rows = %+v, want %+v", got, want)
	}
	if got := parseGateInventory("# no inventory\n"); got != nil {
		t.Errorf("rows = %+v, want none without the section", got)
	}
}
