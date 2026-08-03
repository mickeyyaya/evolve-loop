// frozenpins.go closes the deterministic half of inbox item
// tdd-structural-test-reachability-probe (root cause cycle-644): turning the
// probe from a library an agent MAY call by hand into one the phase-gate
// pipeline calls automatically.
//
// CheckCallSite (reachabilityprobe.go) answers the question for ONE call site
// the caller already knows. The missing piece was deriving those call sites
// from the artefact the TDD phase actually produces: a test-report.md whose
// handoff JSON freezes a set of test files (`doNotModifyTests: true`), each
// pinning package-qualified call sites into named production files. This file
// walks that path end to end — deliverable -> frozen files -> pins -> import
// graph -> violations — so `evolve phase verify tdd` can refuse a permanently
// unsatisfiable acceptance criterion BEFORE the build phase burns on it.
//
// Fail-open discipline throughout: an unreadable file, an underivable module,
// a `go list` failure or an unresolvable package identifier yields NO
// violation. Only a compiler-provable cycle is a confirmed violation, because
// a false HALT here taxes every cycle.
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

// tddHandoff is the subset of the tdd deliverable's "Handoff to Builder" JSON
// that decides whether its pins are permanent commitments.
type tddHandoff struct {
	TestFiles        []string `json:"testFiles"`
	DoNotModifyTests bool     `json:"doNotModifyTests"`
}

// FrozenTestFiles returns the test files a tdd deliverable froze: the handoff
// JSON's testFiles when doNotModifyTests is true, and nil when it is false
// (unfrozen tests are not permanent commitments, so their pins are not the
// cycle-644 failure mode). Paths are returned exactly as written — worktree
// relative, slash separated. An unreadable deliverable is an error; a
// deliverable with no parseable handoff block yields nil, nil.
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

// jsonBlocks returns the bodies of every ```json fenced block in body, in
// document order.
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

// A structural pin is a single line naming BOTH the production file the
// requirement lands in and the package-qualified call site required inside it
// — the `acsassert.FileContains(t, "go/internal/core/state.go",
// "storage.UpdateStateMap(")` idiom, which is literally the cycle-644
// artefact. Both halves are Go string literals, so scanning literals (not raw
// line text) keeps the surrounding assertion call from matching as a pin.
var (
	quotedLiteral  = regexp.MustCompile(`"([^"\\]*)"`)
	goSourceLiteal = regexp.MustCompile(`^[\w./-]+\.go$`)
	pinnedCallSite = regexp.MustCompile(`^([A-Za-z_]\w*)\.([A-Za-z_]\w*)\(`)
)

// pinnedRef is one extracted pin plus the module root its packages resolve
// against and the import aliases in scope where the pin was written — both are
// needed to resolve the pin against a real import graph but are implementation
// details, so ExtractFrozenPins projects them away.
type pinnedRef struct {
	site       CallSite
	moduleRoot string
	// aliases maps an identifier introduced by an import alias in the frozen
	// test file carrying this pin to the full import path it binds. That file
	// is the only place the binding can live: were the PINNED PRODUCTION file
	// to import the referenced package already, the module would be cyclic
	// today and `go list` would produce no graph at all.
	aliases map[string]string
}

// ExtractFrozenPins returns one CallSite per package-qualified pin found in
// frozenTestFiles (worktree-relative paths, as the tdd handoff writes them).
// PinningPackage is the full import path of the package owning the PINNED
// SOURCE FILE — not the test's own package, since the requirement lands in the
// former. ReferencedPackage is the bare identifier exactly as written; it is
// resolved to a full import path inside CheckFrozenPins, where a real import
// graph is available. Files that cannot be read, and pins whose source file
// belongs to no module, are skipped rather than reported.
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

// importAliases returns the identifier -> full import path bindings introduced
// by ALIASED imports in the Go source src. Unaliased imports are omitted: the
// identifier they bind is already covered by resolvePackage's base-name
// matching. `_` and `.` bind no usable package identifier and are never
// resolved, so a pin spelled through one still fails open. A source that will
// not parse binds nothing (fail open) — it proves no cycle either way.
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

// pinOnLine returns the pinned source path, referenced package identifier and
// symbol when line carries both halves of a structural pin.
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

// packageOfFile resolves the module root and full import path of the package
// owning the worktree-relative Go source path rel, by walking up from its
// directory to the nearest go.mod at or below worktreeRoot.
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

// modulePath returns the module path declared by a go.mod body.
func modulePath(gomod string) string {
	for _, line := range strings.Split(gomod, "\n") {
		if rest, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// CheckFrozenPins reports every frozen pin in frozenTestFiles that would close
// an import cycle — the cycle-644 shape — by resolving each pin against the
// real import graph of the module owning its pinned source file. It returns no
// violation for pins it cannot prove: an unresolvable package identifier, a
// module whose graph `go list` will not produce, or a pinning package absent
// from that graph (CheckCallSite's own rule: absence of evidence is not
// evidence of a cycle).
func CheckFrozenPins(worktreeRoot string, frozenTestFiles []string) ([]Violation, error) {
	refs, err := extractPins(worktreeRoot, frozenTestFiles)
	if err != nil {
		return nil, err
	}

	var out []Violation
	// nil entry = this module's graph is underivable; remembered so a broken
	// module costs one `go list` invocation, not one per pin.
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

// resolvePackage maps a bare package identifier as written at a call site to a
// full import path present in graph, preferring the candidate sharing the
// longest prefix with the pinning package (the nearest neighbour in the same
// module) and breaking ties lexically so the verdict is deterministic.
//
// Precedence is exact path, then base name, then alias — and the ORDER is the
// load-bearing part. The identifier is compiled in the PINNED PRODUCTION file's
// scope, not the frozen test file's, so an alias declared in the test file is a
// hint about intent, never an authoritative binding. Consulting it first (as
// this resolver did until cycle-1248) makes the map able to SUPPRESS: one import
// line in the frozen test file rebinding `storage` to some benign package
// silently redirects a `storage.UpdateStateMap(` pin away from the real
// internal/storage that base-name matching would have found, and an alias
// binding anything outside the module graph killed resolution outright. Either
// turns the gate blind to the exact cycle-644 shape it exists to catch — and a
// frozen test file is agent-authored, so that suppression is one plausible-
// looking edit away.
//
// Consulted LAST, the alias can only ever ADD reach: it resolves identifiers
// (`st`, `lf`) that match no package's base name and would otherwise fail open,
// and it can no longer displace a real graph package that does match. An alias
// binding a package absent from graph resolves nothing, which is the same
// fail-open verdict as no alias at all.
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

// commonPrefixLen returns the length of the shared leading run of a and b.
func commonPrefixLen(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}
