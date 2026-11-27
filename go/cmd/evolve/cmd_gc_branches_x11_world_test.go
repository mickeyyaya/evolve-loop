package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const fakeGhScriptX11 = `#!/bin/sh
d=@D@
printf '%s\n' "$*" >> "$d/calls.log"
case "$1" in
pr)
  head=; limit=1000; state=all
  while [ $# -gt 0 ]; do
    case "$1" in
      --head) head=$2; shift;;
      --limit) limit=$2; shift;;
      --state) state=$2; shift;;
    esac
    shift
  done
  if [ -n "$head" ]; then
    f="$d/head-$(printf %s "$head" | tr / _)-$state.json"
    if [ -f "$f" ]; then cat "$f"; else echo '[]'; fi
    exit 0
  fi
  if [ -f "$d/truncate" ]; then
    awk -v n="$limit" 'BEGIN{printf "["; for(i=1;i<=n;i++){if(i>1)printf ","; printf "{\"number\":%d,\"headRefName\":\"filler/%d\",\"headRefOid\":\"0000000000000000000000000000000000000000\",\"state\":\"MERGED\",\"closedAt\":null}",i,i} print "]"}'
    exit 0
  fi
  if [ -f "$d/$state.json" ]; then cat "$d/$state.json"; else echo '[]'; fi
  ;;
api)
  for a in "$@"; do last=$a; done
  case "$*" in
    *"-X DELETE"*)
      b=${last#*/git/refs/heads/}
      if grep -qxF "$b" "$d/fail-delete" 2>/dev/null; then echo "HTTP 422: Reference update failed" >&2; exit 1; fi
      git -C "$(cat "$d/origin")" update-ref -d "refs/heads/$b"
      ;;
    *) echo '[]';;
  esac
  ;;
*) echo '[]';;
esac
`

type prX11 struct {
	Number      int     `json:"number"`
	HeadRefName string  `json:"headRefName"`
	HeadRefOid  string  `json:"headRefOid"`
	State       string  `json:"state"`
	ClosedAt    *string `json:"closedAt"`
}

type worldX11 struct {
	t         *testing.T
	hub       *gittest.Repo
	origin    *gittest.Repo
	ghDir     string
	evolveDir string
	tips      map[string]string
	prs       []prX11
	now       time.Time
}

func newWorldX11(t *testing.T) *worldX11 {
	t.Helper()
	binDir := gcIsolateHost(t)
	w := &worldX11{t: t, hub: gittest.Fixture(t), origin: gittest.Bare(t), ghDir: t.TempDir(), tips: map[string]string{}, now: time.Now().UTC()}
	fakeclitest.Install(t, filepath.Join(binDir, "gh"), strings.ReplaceAll(fakeGhScriptX11, "@D@", w.ghDir))
	w.writeGh("origin", w.origin.Dir)
	w.hub.Write("seed.txt", "seed\n")
	w.hub.Git("add", "-A")
	w.hub.Git("commit", "-q", "-m", "seed")
	w.hub.Git("remote", "add", "origin", w.origin.Dir)
	w.hub.Git("push", "-q", "origin", "main")
	w.evolveDir = filepath.Join(w.hub.Dir, ".evolve")
	if err := os.MkdirAll(filepath.Join(w.evolveDir, "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.setPolicy(nil, false)
	return w
}

func (w *worldX11) writeGh(name, body string) {
	w.t.Helper()
	if err := os.WriteFile(filepath.Join(w.ghDir, name), []byte(body), 0o644); err != nil {
		w.t.Fatal(err)
	}
}

func (w *worldX11) setPolicy(gcExtra map[string]any, enforce bool) {
	w.t.Helper()
	block := map[string]any{}
	if enforce {
		block["remote_sweep"] = map[string]any{"stage": "enforce"}
	}
	maps.Copy(block, gcExtra)
	raw, err := json.Marshal(map[string]any{"gc": block})
	if err != nil {
		w.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.evolveDir, "policy.json"), raw, 0o644); err != nil {
		w.t.Fatal(err)
	}
}

func (w *worldX11) branch(name string, push bool) string {
	w.t.Helper()
	w.hub.Git("checkout", "-q", "-b", name, "main")
	w.hub.Write(strings.ReplaceAll(name, "/", "_")+".txt", name+"\n")
	w.hub.Git("add", "-A")
	w.hub.Git("commit", "-q", "-m", "work "+name)
	tip := w.hub.Git("rev-parse", "HEAD")
	if push {
		w.hub.Git("push", "-q", "origin", name)
	}
	w.hub.Git("checkout", "-q", "main")
	w.tips[name] = tip
	return tip
}

func (w *worldX11) advance(name string) string {
	w.t.Helper()
	w.hub.Git("checkout", "-q", name)
	w.hub.Write("more_"+strings.ReplaceAll(name, "/", "_")+".txt", "more\n")
	w.hub.Git("add", "-A")
	w.hub.Git("commit", "-q", "-m", "more "+name)
	tip := w.hub.Git("rev-parse", "HEAD")
	w.hub.Git("push", "-q", "origin", name)
	w.hub.Git("checkout", "-q", "main")
	return tip
}

func (w *worldX11) pr(branch, state string, closedAgo time.Duration) {
	w.t.Helper()
	row := prX11{Number: 11 + len(w.prs), HeadRefName: branch, HeadRefOid: w.tips[branch], State: state}
	if state != "OPEN" {
		stamp := w.now.Add(-closedAgo).Format(time.RFC3339)
		row.ClosedAt = &stamp
	}
	w.prs = append(w.prs, row)
}

func (w *worldX11) publish() {
	w.t.Helper()
	byState := map[string][]prX11{"all": w.prs, "open": nil, "closed": nil, "merged": nil}
	for _, p := range w.prs {
		byState[strings.ToLower(p.State)] = append(byState[strings.ToLower(p.State)], p)
	}
	for state, rows := range byState {
		w.writeRows(state+".json", rows)
	}
	for _, p := range w.prs {
		for state, rows := range byState {
			var own []prX11
			for _, r := range rows {
				if r.HeadRefName == p.HeadRefName {
					own = append(own, r)
				}
			}
			w.writeRows(fmt.Sprintf("head-%s-%s.json", strings.ReplaceAll(p.HeadRefName, "/", "_"), state), own)
		}
	}
}

func (w *worldX11) writeRows(name string, rows []prX11) {
	w.t.Helper()
	if rows == nil {
		rows = []prX11{}
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		w.t.Fatal(err)
	}
	w.writeGh(name, string(raw))
}

func (w *worldX11) truncateLookups() { w.writeGh("truncate", "") }

func (w *worldX11) failRemoteDeleteOf(branches ...string) {
	w.writeGh("fail-delete", strings.Join(branches, "\n")+"\n")
}

func (w *worldX11) pointOriginHeadAt(branch string) {
	w.t.Helper()
	w.hub.Git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/"+branch)
}

func (w *worldX11) standard(withCycle bool) {
	w.t.Helper()
	w.branch("console/merged", true)
	w.pr("console/merged", "MERGED", 24*time.Hour)
	w.branch("console/closed-old", true)
	w.pr("console/closed-old", "CLOSED", 10*24*time.Hour)
	w.branch("console/closed-young", true)
	w.pr("console/closed-young", "CLOSED", 36*time.Hour)
	w.branch("console/open", true)
	w.pr("console/open", "OPEN", 0)
	w.branch("stray/unowned", true)
	w.pr("stray/unowned", "MERGED", 24*time.Hour)
	w.branch("console/checked-out", false)
	w.pr("console/checked-out", "MERGED", 24*time.Hour)
	w.hub.Git("worktree", "add", "-q", filepath.Join(w.t.TempDir(), "held"), "console/checked-out")
	w.hub.Git("branch", "console/empty-new", "main")
	if withCycle {
		w.branch("cycle-x11-merged", true)
		w.pr("cycle-x11-merged", "MERGED", 24*time.Hour)
		w.branch("cycle-x11-closed", true)
		w.pr("cycle-x11-closed", "CLOSED", 10*24*time.Hour)
	}
	w.hub.Git("remote", "set-head", "origin", "main")
}

func (w *worldX11) run(args ...string) (rc int, stdout, stderr string) {
	w.t.Helper()
	var out, errOut bytes.Buffer
	rc = runGC(append(args, "--project-root", w.hub.Dir), nil, &out, &errOut)
	return rc, out.String(), errOut.String()
}

func (w *worldX11) plan(args ...string) gc.CheckReport {
	w.t.Helper()
	rc, out, errOut := w.run(append([]string{"check", "--only", "branches", "--json"}, args...)...)
	if rc != cliExitOK {
		w.t.Fatalf("evolve gc check --only branches rc=%d stderr=%s", rc, errOut)
	}
	var rep gc.CheckReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		w.t.Fatalf("decode check report: %v\n%s", err, out)
	}
	if len(rep.Checks) != 1 || rep.Checks[0].Name != "branches" {
		w.t.Fatalf("--only branches must run the branches check alone, got %+v", rep.Checks)
	}
	return rep
}

func (w *worldX11) checkNames(args ...string) []string {
	w.t.Helper()
	rc, out, errOut := w.run(args...)
	if rc != cliExitOK {
		w.t.Fatalf("evolve gc %v rc=%d stderr=%s", args, rc, errOut)
	}
	var rep gc.CheckReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		w.t.Fatalf("decode check report: %v\n%s", err, out)
	}
	var names []string
	for _, c := range rep.Checks {
		names = append(names, c.Name)
	}
	return names
}

func (w *worldX11) ghCalls() string {
	w.t.Helper()
	raw, err := os.ReadFile(filepath.Join(w.ghDir, "calls.log"))
	if err != nil && !os.IsNotExist(err) {
		w.t.Fatal(err)
	}
	return string(raw)
}

func (w *worldX11) localExists(branch string) bool {
	w.t.Helper()
	return w.hub.Git("branch", "--list", branch) != ""
}

func (w *worldX11) remoteTip(branch string) string {
	w.t.Helper()
	out := w.hub.Git("ls-remote", "--heads", "origin", "refs/heads/"+branch)
	tip, _, _ := strings.Cut(out, "\t")
	return tip
}

func itemNamesX11(it gc.CheckItem, branch string) bool {
	fields := strings.FieldsFunc(it.Subject, func(r rune) bool { return r == ' ' || r == ':' || r == '=' || r == '@' || r == '\t' })
	for _, f := range fields {
		if f == branch || strings.HasSuffix(f, "/"+branch) {
			return true
		}
	}
	return false
}

func itemDeletesX11(it gc.CheckItem) bool {
	return strings.Contains(strings.ToLower(it.Action), "delete")
}

func deletesOfX11(rep gc.CheckReport, branch string) []gc.CheckItem {
	var out []gc.CheckItem
	for _, it := range rep.Checks[0].Items {
		if itemNamesX11(it, branch) && itemDeletesX11(it) {
			out = append(out, it)
		}
	}
	return out
}

func requireDeletedByClassX11(t *testing.T, rep gc.CheckReport, branch, class string) {
	t.Helper()
	items := deletesOfX11(rep, branch)
	if len(items) == 0 {
		t.Errorf("%s is not planned for delete; items: %+v", branch, rep.Checks[0].Items)
		return
	}
	for _, it := range items {
		if !strings.Contains(strings.ToUpper(it.Rule+" "+it.Evidence), class) {
			t.Errorf("%s: delete item names no class %s: %+v", branch, class, it)
		}
	}
}

func requireKeptX11(t *testing.T, rep gc.CheckReport, branches ...string) {
	t.Helper()
	for _, b := range branches {
		if items := deletesOfX11(rep, b); len(items) > 0 {
			t.Errorf("%s must be kept but is planned for delete: %+v", b, items)
		}
	}
}
