package reachabilityprobe

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type tddHandoff struct {
	TestFiles        []string `json:"testFiles"`
	DoNotModifyTests bool     `json:"doNotModifyTests"`
}

func HandoffTestFiles(reportPath string) ([]string, error) {
	files, _, err := handoffOf(reportPath)
	return files, err
}

func FrozenTestFiles(reportPath string) ([]string, error) {
	files, frozen, err := handoffOf(reportPath)
	if err != nil || !frozen {
		return nil, err
	}
	return files, nil
}

func handoffOf(reportPath string) (files []string, frozen bool, err error) {
	body, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, false, fmt.Errorf("reachabilityprobe: reading tdd deliverable %s: %w", reportPath, err)
	}
	for _, block := range jsonBlocks(string(body)) {
		var handoff tddHandoff
		if err := json.Unmarshal([]byte(block), &handoff); err != nil {
			continue
		}
		if len(handoff.TestFiles) == 0 {
			continue
		}
		return handoff.TestFiles, handoff.DoNotModifyTests, nil
	}
	return nil, false, nil
}

func jsonBlocks(body string) []string {
	var out []string
	rest := body
	for {
		open := strings.Index(rest, "```json")
		if open < 0 {
			return out
		}
		rest = rest[open+len("```json"):]
		closing := strings.Index(rest, "```")
		if closing < 0 {
			return out
		}
		out = append(out, rest[:closing])
		rest = rest[closing+len("```"):]
	}
}

var (
	quotedLiteral   = regexp.MustCompile(`"([^"\\]*)"`)
	goSourceLiteral = regexp.MustCompile(`^[\w./-]+\.go$`)
	pinnedCallSite  = regexp.MustCompile(`^([A-Za-z_]\w*)\.([A-Za-z_]\w*)\(`)
)

type pinnedRef struct {
	site       CallSite
	moduleRoot string
	aliases    map[string]string
}

func ExtractFrozenPins(worktreeRoot string, frozenTestFiles []string) ([]CallSite, error) {
	refs := extractPins(worktreeRoot, frozenTestFiles)
	sites := make([]CallSite, 0, len(refs))
	for _, ref := range refs {
		sites = append(sites, ref.site)
	}
	return sites, nil
}

func extractPins(worktreeRoot string, frozenTestFiles []string) []pinnedRef {
	var refs []pinnedRef
	for _, rel := range frozenTestFiles {
		body, err := os.ReadFile(filepath.Join(worktreeRoot, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		aliases := importAliases(body)
		for _, line := range strings.Split(string(body), "\n") {
			source, referenced, symbol, ok := pinOnLine(line)
			if !ok {
				continue
			}
			moduleRoot, pinning, ok := packageOfFile(worktreeRoot, source)
			if !ok {
				continue
			}
			refs = append(refs, pinnedRef{
				site: CallSite{
					PinningPackage:    pinning,
					ReferencedPackage: referenced,
					Symbol:            symbol,
				},
				moduleRoot: moduleRoot,
				aliases:    aliases,
			})
		}
	}
	return refs
}

func importAliases(src []byte) map[string]string {
	file, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	var aliases map[string]string
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name == "_" || spec.Name.Name == "." {
			continue
		}
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if aliases == nil {
			aliases = map[string]string{}
		}
		aliases[spec.Name.Name] = imported
	}
	return aliases
}

func pinOnLine(line string) (source, referenced, symbol string, ok bool) {
	for _, m := range quotedLiteral.FindAllStringSubmatch(line, -1) {
		literal := m[1]
		switch {
		case source == "" && goSourceLiteral.MatchString(literal):
			source = literal
		case referenced == "":
			if call := pinnedCallSite.FindStringSubmatch(literal); call != nil {
				referenced, symbol = call[1], call[2]
			}
		}
	}
	return source, referenced, symbol, source != "" && referenced != ""
}

func packageOfFile(worktreeRoot, rel string) (moduleRoot, importPath string, ok bool) {
	root := filepath.Clean(worktreeRoot)
	dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(rel)))
	for cur := dir; ; {
		if data, err := os.ReadFile(filepath.Join(cur, "go.mod")); err == nil {
			module := modulePath(string(data))
			if module == "" {
				return "", "", false
			}
			within, err := filepath.Rel(cur, dir)
			if err != nil {
				return "", "", false
			}
			if within = filepath.ToSlash(within); within != "." {
				module += "/" + within
			}
			return cur, module, true
		}
		parent := filepath.Dir(cur)
		if parent == cur || len(cur) <= len(root) {
			return "", "", false
		}
		cur = parent
	}
}

func modulePath(gomod string) string {
	for _, line := range strings.Split(gomod, "\n") {
		if rest, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func CheckFrozenPins(worktreeRoot string, frozenTestFiles []string) ([]Violation, error) {
	refs := extractPins(worktreeRoot, frozenTestFiles)

	var out []Violation
	graphs := map[string]ImportGraph{}
	for _, ref := range refs {
		graph, seen := graphs[ref.moduleRoot]
		if !seen {
			var err error
			graph, err = BuildImportGraph(ref.moduleRoot, "./...")
			if err != nil {
				graph = nil
			}
			graphs[ref.moduleRoot] = graph
		}
		if graph == nil {
			continue
		}
		referenced, ok := resolvePackage(graph, ref.site.ReferencedPackage, ref.site.PinningPackage, ref.aliases)
		if !ok {
			continue
		}
		site := ref.site
		site.ReferencedPackage = referenced
		if violation := CheckCallSite(graph, site); violation != nil {
			out = append(out, *violation)
		}
	}
	return out, nil
}

func resolvePackage(graph ImportGraph, ident, pinning string, aliases map[string]string) (string, bool) {
	if _, exact := graph[ident]; exact {
		return ident, true
	}
	best, bestScore := "", -1
	for pkg := range graph {
		if path.Base(pkg) != ident {
			continue
		}
		score := commonPrefixLen(pkg, pinning)
		if score > bestScore || (score == bestScore && pkg < best) {
			best, bestScore = pkg, score
		}
	}
	if best != "" {
		return best, true
	}
	if aliased, bound := aliases[ident]; bound {
		if _, known := graph[aliased]; known {
			return aliased, true
		}
	}
	return "", false
}

func commonPrefixLen(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}
