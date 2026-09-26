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

// tddHandoff is the part of the tdd deliverable's handoff JSON that decides whether its pins are frozen.
type tddHandoff struct {
	TestFiles        []string `json:"testFiles"`
	DoNotModifyTests bool     `json:"doNotModifyTests"`
}

// FrozenTestFiles returns the worktree-relative test files a tdd deliverable froze, or nil when it froze none.
func FrozenTestFiles(reportPath string) ([]string, error) {
	body, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("reachabilityprobe: reading tdd deliverable %s: %w", reportPath, err)
	}
	for _, block := range jsonBlocks(string(body)) {
		var handoff tddHandoff
		if err := json.Unmarshal([]byte(block), &handoff); err != nil {
			continue
		}
		if len(handoff.TestFiles) == 0 {
			continue
		}
		if !handoff.DoNotModifyTests {
			return nil, nil
		}
		return handoff.TestFiles, nil
	}
	return nil, nil
}

// jsonBlocks returns the body of each ```json fenced block in body, in order.
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

// A pin is one line holding a .go path literal and an ident.Symbol( literal. Scanning
// string literals, not raw line text, keeps the assertion call itself from matching.
var (
	quotedLiteral  = regexp.MustCompile(`"([^"\\]*)"`)
	goSourceLiteal = regexp.MustCompile(`^[\w./-]+\.go$`)
	pinnedCallSite = regexp.MustCompile(`^([A-Za-z_]\w*)\.([A-Za-z_]\w*)\(`)
)

// pinnedRef is one extracted pin plus what resolving it needs: its module root and import aliases.
type pinnedRef struct {
	site       CallSite
	moduleRoot string
	// aliases come from the frozen test file: a pinned file importing the package
	// would already be cyclic, and `go list` would produce no graph.
	aliases map[string]string
}

// ExtractFrozenPins returns each pin with the pinned file's package and the referenced identifier as written.
func ExtractFrozenPins(worktreeRoot string, frozenTestFiles []string) ([]CallSite, error) {
	refs, err := extractPins(worktreeRoot, frozenTestFiles)
	if err != nil {
		return nil, err
	}
	sites := make([]CallSite, 0, len(refs))
	for _, ref := range refs {
		sites = append(sites, ref.site)
	}
	return sites, nil
}

func extractPins(worktreeRoot string, frozenTestFiles []string) ([]pinnedRef, error) {
	var refs []pinnedRef
	for _, rel := range frozenTestFiles {
		body, err := os.ReadFile(filepath.Join(worktreeRoot, filepath.FromSlash(rel)))
		if err != nil {
			continue // fail open: a frozen file we cannot read proves nothing
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
	return refs, nil
}

// importAliases maps each aliased import's name to its path; base-name matching covers unaliased imports.
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

// pinOnLine reports the first .go path literal and the first ident.Symbol( literal on line.
func pinOnLine(line string) (source, referenced, symbol string, ok bool) {
	for _, m := range quotedLiteral.FindAllStringSubmatch(line, -1) {
		literal := m[1]
		switch {
		case source == "" && goSourceLiteal.MatchString(literal):
			source = literal
		case referenced == "":
			if call := pinnedCallSite.FindStringSubmatch(literal); call != nil {
				referenced, symbol = call[1], call[2]
			}
		}
	}
	return source, referenced, symbol, source != "" && referenced != ""
}

// packageOfFile resolves rel's package from the nearest go.mod at or below worktreeRoot.
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

// CheckFrozenPins returns a Violation for each frozen pin its module's real import graph proves would close a cycle.
func CheckFrozenPins(worktreeRoot string, frozenTestFiles []string) ([]Violation, error) {
	refs, err := extractPins(worktreeRoot, frozenTestFiles)
	if err != nil {
		return nil, err
	}

	var out []Violation
	// A nil graph marks an underivable module, so it costs one `go list`, not one per pin.
	graphs := map[string]ImportGraph{}
	for _, ref := range refs {
		graph, seen := graphs[ref.moduleRoot]
		if !seen {
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

// resolvePackage maps ident to a graph package: exact path, then the base-name match
// nearest to pinning (ties broken lexically), then the frozen test file's alias.
//
// The alias goes last because ident compiles in the pinned production file, not the
// test file, so an alias may add reach but must never displace a real match.
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
