package bridge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var grantReportLine = regexp.MustCompile(`^Sandbox: [^\n]*?\(\d+\) allow \S+ ([^\n]+)\n` + regexp.QuoteMeta(sandbox.CellMessagePrefix) + `\S+ rule=` + regexp.QuoteMeta(sandbox.RuleGrantIndex) + `$`)

func parseGrantReports(raw []byte) []string {
	var out []string
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var event struct {
			EventMessage string `json:"eventMessage"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if m := grantReportLine.FindStringSubmatch(event.EventMessage); m != nil && !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

func (deps Deps) recordContextAccess(cfg *Config, targets []string, source string) {
	idx, err := inputindex.Read(cfg.Workspace)
	if err != nil && !errors.Is(err, fs.ErrNotExist) && deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN context access log: %v (no access record)\n", err)
	}
	if err != nil || idx.Phase != cfg.Agent {
		return
	}
	deps.emitBudgetOverflow(cfg, idx)
	var opened []string
	for _, e := range idx.Entries {
		if slices.Contains(targets, canonicalOrSelf(filepath.Join(cfg.ProjectRoot, filepath.FromSlash(e.Path)))) {
			opened = append(opened, e.ID)
		}
	}
	unopened, err := inputindex.RecordAccess(cfg.ProjectRoot, idx, cfg.DispatchID, source, opened)
	if err != nil {
		if deps.Stderr != nil {
			fmt.Fprintf(deps.Stderr, "[bridge] WARN context access log: %v\n", err)
		}
		return
	}
	if source == inputindex.AccessSourceSandbox && len(unopened) > 0 {
		deps.emitContext(cfg, signalcenter.SeverityInfo, isolation.CodeContextRequiredUnopened,
			"the cell "+cfg.Agent+" closed without a read of the required entries "+strings.Join(unopened, ", "),
			map[string]string{"unopened": strings.Join(unopened, ","), "source": source})
	}
}

func (deps Deps) emitBudgetOverflow(cfg *Config, idx inputindex.Index) {
	layers := make([]string, 0, len(idx.Budgets))
	for layer := range idx.Budgets {
		layers = append(layers, layer)
	}
	sort.Strings(layers)
	for _, layer := range layers {
		b := idx.Budgets[layer]
		if b.Over == 0 {
			continue
		}
		deps.emitContext(cfg, signalcenter.SeverityWarn, isolation.CodeContextBudgetOver,
			fmt.Sprintf("%d required entries of %s passed the budget of %d tokens and lost the flag", b.Over, layer, b.LimitTokens),
			map[string]string{"layer": layer, "limit_tokens": strconv.Itoa(b.LimitTokens), "over": strconv.Itoa(b.Over)})
	}
}

func (deps Deps) emitContext(cfg *Config, severity signalcenter.Severity, code signalcenter.Code, reason string, fields map[string]string) {
	id := configIdentity(cfg)
	fields["cell"], fields["family"] = cfg.Agent, cfg.CLI
	deps.Signals.Emit(signalcenter.Event{
		Cycle: id.cycle, RunID: id.runID, Phase: id.phase, Module: signalcenter.ModuleIsolation, Origin: "bridge.recordContextAccess",
		Kind: signalcenter.KindIsolationContext, Severity: severity, Code: code, Reason: reason, Fields: fields,
	})
}
