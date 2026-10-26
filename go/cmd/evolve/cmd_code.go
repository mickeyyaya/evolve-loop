package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/gotypes"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/trail"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	codeUsage = "usage: evolve code callers <symbol> [--direction in|out] | api <file> | find <query> | map [dir]  [--project-root P] [--json] | refresh [--project-root P] [--json] | usage [--since D] [--json]"

	codeExitNotFound    = 1
	codeExitAbsent      = 2
	codeExitUnsupported = 3
	codeExitFailed      = 4
	codeExitUsage       = 10

	codeQueryTimeout = 30 * time.Second
)

type codeIndexHandle struct {
	backends    codeindex.Backends
	freshness   func(context.Context) (codeIndexFreshness, error)
	refuseStale bool
}

type codeBackendsBuilder func(projectRoot string) (codeIndexHandle, error)

type codeInvocation struct {
	request     codeindex.Request
	projectRoot string
	json        bool
}

type codeHitJSON struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Symbol string `json:"symbol"`
	Text   string `json:"text"`
}

type codeQueryData struct {
	Hits      []codeHitJSON `json:"hits"`
	Stale     *bool         `json:"stale,omitempty"`
	Behind    *int          `json:"behind,omitempty"`
	IndexHead string        `json:"index_head,omitempty"`
	MainHead  string        `json:"main_head,omitempty"`
}

var codeUsageCacheDir = os.UserCacheDir

func runCodeCommand(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == codeUsageVerbName {
		return codeUsageVerb(args[1:], stdout, stderr, codeUsageCacheDir, codeProjectRoot(""))
	}
	if len(args) > 0 && args[0] == codeRefreshVerbName {
		return codeRefreshVerb(args[1:], stdout, stderr, defaultCodeIndexHost(), os.Getenv, codeIndexRefreshTimeout)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	sink := codeUsageSink{env: os.Getenv, cacheDir: codeUsageCacheDir, rootOf: func() string { return codeProjectRoot("") }}
	return codeVerbLogged(ctx, args, stdout, stderr, func(root string) (codeIndexHandle, error) {
		return buildCodeIndex(root, defaultCodeIndexHost())
	}, sink)
}

func codeVerb(ctx context.Context, args []string, stdout, stderr io.Writer, build codeBackendsBuilder) (int, codeCall) {
	inv, err := parseCodeArgs(args, stderr)
	call := codeCall{verb: codeVerbName(args), root: inv.projectRoot}
	out := codeEnvelopeWriter{schema: "evolve.code." + cmp.Or(call.verb, "query") + "/1", json: slices.Contains(args, "--json"), stdout: stdout, stderr: stderr}
	if err != nil {
		return out.fail(codeExitUsage, "usage", fmt.Sprintf("evolve code: %v\n%s", err, codeUsage)), call
	}
	out.json = inv.json
	call.argLen = len(inv.request.Arg)
	idx, err := build(inv.projectRoot)
	if err != nil {
		return out.fail(codeExitFailed, "failed", fmt.Sprintf("evolve code: %v", err)), call
	}
	ctx, cancel := context.WithTimeout(ctx, codeQueryTimeout)
	defer cancel()
	check := checkCodeFreshness(ctx, idx, string(inv.request.Query))
	out.warnings = check.warnings
	for _, w := range check.warnings {
		fmt.Fprintln(stderr, w)
	}
	if check.refused != "" {
		return out.fail(codeExitStale, "stale", check.refused), call
	}
	hits, err := idx.backends.Search(ctx, inv.request)
	if err != nil {
		rc, kind, message := codeErrorOutcome(inv.request, err)
		return out.fail(rc, kind, message), call
	}
	call.hits = len(hits)
	if !out.json {
		printCodeHits(hits, stdout)
	}
	return out.ok(codeQueryResult(hits, check.fresh)), call
}

func codeQueryResult(hits []codeindex.Hit, fresh *codeIndexFreshness) codeQueryData {
	data := codeQueryData{Hits: make([]codeHitJSON, 0, len(hits))}
	for _, h := range hits {
		data.Hits = append(data.Hits, codeHitJSON{File: h.File, Line: h.Line, Symbol: h.Symbol, Text: h.Text})
	}
	if fresh != nil {
		stale, behind := fresh.stale(), fresh.Behind
		data.Stale, data.Behind, data.IndexHead, data.MainHead = &stale, &behind, fresh.Head, fresh.Main
	}
	return data
}

func codeVerbName(args []string) string {
	if len(args) > 0 && slices.Contains(codeindex.Queries(), codeindex.Query(args[0])) {
		return args[0]
	}
	return ""
}

func codeErrorOutcome(req codeindex.Request, err error) (int, string, string) {
	switch {
	case errors.Is(err, codeindex.ErrNotFound):
		return codeExitNotFound, "not_found", fmt.Sprintf("evolve code %s: no result for %s", req.Query, codeSubject(req.Arg))
	case errors.Is(err, codeindex.ErrAbsent):
		return codeExitAbsent, "absent", fmt.Sprintf("evolve code %s: %v (see docs/architecture/code-index-cli.md, section 4)", req.Query, err)
	case errors.Is(err, codeindex.ErrUnsupported):
		return codeExitUnsupported, "unsupported", fmt.Sprintf("evolve code %s: %v", req.Query, err)
	}
	return codeExitFailed, "failed", fmt.Sprintf("evolve code %s: %v", req.Query, err)
}

func codeSubject(arg string) string {
	if arg == "" {
		return "the whole project"
	}
	return fmt.Sprintf("%q", arg)
}

var codeFieldCleaner = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

func printCodeHits(hits []codeindex.Hit, stdout io.Writer) {
	for _, hit := range hits {
		place := hit.File
		if hit.Line > 0 {
			place = fmt.Sprintf("%s:%d", hit.File, hit.Line)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", codeFieldCleaner.Replace(place), codeFieldCleaner.Replace(hit.Symbol), codeFieldCleaner.Replace(hit.Text))
	}
}

func parseCodeArgs(args []string, stderr io.Writer) (codeInvocation, error) {
	if len(args) == 0 {
		return codeInvocation{}, errors.New("missing verb")
	}
	fs := flag.NewFlagSet("evolve code "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	direction := fs.String("direction", string(codeindex.DirectionIn), "callers: in (who calls it) or out (what it calls)")
	root := fs.String("project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	asJSON := fs.Bool("json", false, "print one evolve.code.<verb>/1 envelope on stdout with the hits and the index freshness")
	positional, err := parseInterleaved(fs, args[1:])
	if err != nil {
		return codeInvocation{}, err
	}
	directionSet := false
	fs.Visit(func(f *flag.Flag) { directionSet = directionSet || f.Name == "direction" })
	req, err := codeRequest(args[0], positional, *direction, directionSet)
	if err != nil {
		return codeInvocation{}, err
	}
	return codeInvocation{request: req, projectRoot: codeProjectRoot(*root), json: *asJSON}, nil
}

func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func codeRequest(verb string, positional []string, direction string, directionSet bool) (codeindex.Request, error) {
	if directionSet && verb != string(codeindex.QueryCallers) {
		return codeindex.Request{}, fmt.Errorf("--direction applies to callers only, not %s", verb)
	}
	switch codeindex.Query(verb) {
	case codeindex.QueryCallers:
		dir, err := codeindex.ParseDirection(direction)
		if err != nil {
			return codeindex.Request{}, err
		}
		arg, err := exactlyOne(verb, positional)
		return codeindex.Request{Query: codeindex.QueryCallers, Arg: arg, Direction: dir}, err
	case codeindex.QueryAPI:
		arg, err := exactlyOne(verb, positional)
		return codeindex.Request{Query: codeindex.QueryAPI, Arg: arg}, err
	case codeindex.QueryFind:
		query := strings.TrimSpace(strings.Join(positional, " "))
		if query == "" {
			return codeindex.Request{}, errors.New("find wants a query")
		}
		return codeindex.Request{Query: codeindex.QueryFind, Arg: query}, nil
	case codeindex.QueryMap:
		if len(positional) > 1 {
			return codeindex.Request{}, errors.New("map wants at most one directory")
		}
		return codeindex.Request{Query: codeindex.QueryMap, Arg: strings.Join(positional, "")}, nil
	}
	return codeindex.Request{}, fmt.Errorf("unknown verb %q", verb)
}

func exactlyOne(verb string, positional []string) (string, error) {
	if len(positional) != 1 || strings.TrimSpace(positional[0]) == "" {
		return "", fmt.Errorf("%s wants exactly one argument", verb)
	}
	return positional[0], nil
}

func codeProjectRoot(flagValue string) string {
	explicit := cmp.Or(flagValue, os.Getenv("EVOLVE_PROJECT_ROOT"))
	if explicit != "" {
		return paths.AbsoluteRoot("--project-root", explicit, nil)
	}
	return checkoutRoot(paths.AbsoluteRoot("--project-root", ".", nil))
}

func checkoutRoot(dir string) string {
	for current := dir; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		if filepath.Dir(current) == current {
			return dir
		}
	}
}

func buildCodeIndex(projectRoot string, host codeIndexHost) (codeIndexHandle, error) {
	loc, err := loadCodeIndexLocation(projectRoot)
	if err != nil {
		return codeIndexHandle{}, err
	}
	backends, err := newCodeBackends(loc, host.run, host.env)
	if err != nil {
		return codeIndexHandle{}, err
	}
	fresh := func(ctx context.Context) (codeIndexFreshness, error) { return host.freshness(ctx, loc.indexDir) }
	return codeIndexHandle{backends: backends, freshness: fresh, refuseStale: loc.stale == policy.CodeIndexStaleRefuse}, nil
}

type codeIndexLocation struct {
	backends           []string
	indexDir, trailBin string
	stale              string
}

func loadCodeIndexLocation(projectRoot string) (codeIndexLocation, error) {
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return codeIndexLocation{}, err
	}
	cfg, err := pol.CodeIndexConfig()
	if err != nil {
		return codeIndexLocation{}, err
	}
	indexDir, trailBin, err := resolveCodeIndexPaths(projectRoot, cfg)
	if err != nil {
		return codeIndexLocation{}, err
	}
	return codeIndexLocation{backends: cfg.Backends, indexDir: indexDir, trailBin: trailBin, stale: cfg.Stale}, nil
}

func newCodeBackends(loc codeIndexLocation, run sysexec.RunFunc, env []string) (codeindex.Backends, error) {
	factories := map[string]codeindex.Factory{
		"gotypes": func() (codeindex.Backend, error) {
			return gotypes.New(gotypes.IndexPath(loc.indexDir)), nil
		},
		"trail": func() (codeindex.Backend, error) {
			return trail.New(trail.Config{Bin: loc.trailBin, IndexDir: loc.indexDir, Env: env, Run: run}), nil
		},
	}
	return codeindex.New(loc.backends, factories)
}

func resolveCodeIndexPaths(projectRoot string, cfg policy.CodeIndexSettings) (indexDir, trailBin string, err error) {
	if filepath.IsAbs(cfg.IndexDir) && filepath.IsAbs(cfg.TrailBin) {
		return cfg.IndexDir, cfg.TrailBin, nil
	}
	hub, err := plane.ResolveHub(projectRoot)
	if err != nil {
		return "", "", fmt.Errorf("%w: code_index.index_dir %q and code_index.trail_bin %q are relative to the hub root, but %s is not a hub worktree: %w",
			errCodeIndexAbsent, cfg.IndexDir, cfg.TrailBin, projectRoot, err)
	}
	return underHub(hub.Root, cfg.IndexDir), underHub(hub.Root, cfg.TrailBin), nil
}

func underHub(hubRoot, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(hubRoot, p)
}
