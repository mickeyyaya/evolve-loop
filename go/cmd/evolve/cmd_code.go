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
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/trail"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	codeUsage = "usage: evolve code callers <symbol> [--direction in|out] | api <file> | find <query> | map [dir]  [--project-root P]"

	codeExitNotFound    = 1
	codeExitAbsent      = 2
	codeExitUnsupported = 3
	codeExitFailed      = 4
	codeExitUsage       = 10

	codeQueryTimeout = 30 * time.Second
)

type codeBackendsBuilder func(projectRoot string) (codeindex.Backends, error)

type codeInvocation struct {
	request     codeindex.Request
	projectRoot string
}

func runCodeCommand(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return codeVerb(ctx, args, stdout, stderr, func(root string) (codeindex.Backends, error) {
		return buildCodeBackends(root, sysexec.DefaultRunner, os.Environ())
	})
}

func codeVerb(ctx context.Context, args []string, stdout, stderr io.Writer, build codeBackendsBuilder) int {
	inv, err := parseCodeArgs(args, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve code: %v\n%s\n", err, codeUsage)
		return codeExitUsage
	}
	backends, err := build(inv.projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "evolve code: %v\n", err)
		return codeExitFailed
	}
	ctx, cancel := context.WithTimeout(ctx, codeQueryTimeout)
	defer cancel()
	hits, err := backends.Search(ctx, inv.request)
	if err != nil {
		return reportCodeError(inv.request, err, stderr)
	}
	printCodeHits(hits, stdout)
	return 0
}

func reportCodeError(req codeindex.Request, err error, stderr io.Writer) int {
	switch {
	case errors.Is(err, codeindex.ErrNotFound):
		fmt.Fprintf(stderr, "evolve code %s: no result for %s\n", req.Query, codeSubject(req.Arg))
		return codeExitNotFound
	case errors.Is(err, codeindex.ErrAbsent):
		fmt.Fprintf(stderr, "evolve code %s: %v (see docs/architecture/code-index-cli.md, section 4)\n", req.Query, err)
		return codeExitAbsent
	case errors.Is(err, codeindex.ErrUnsupported):
		fmt.Fprintf(stderr, "evolve code %s: %v\n", req.Query, err)
		return codeExitUnsupported
	}
	fmt.Fprintf(stderr, "evolve code %s: %v\n", req.Query, err)
	return codeExitFailed
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
	return codeInvocation{request: req, projectRoot: codeProjectRoot(*root)}, nil
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

func buildCodeBackends(projectRoot string, run sysexec.RunFunc, env []string) (codeindex.Backends, error) {
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return codeindex.Backends{}, err
	}
	cfg, err := pol.CodeIndexConfig()
	if err != nil {
		return codeindex.Backends{}, err
	}
	indexDir, trailBin, err := resolveCodeIndexPaths(projectRoot, cfg)
	if err != nil {
		return codeindex.Backends{}, err
	}
	factories := map[string]codeindex.Factory{
		"trail": func() (codeindex.Backend, error) {
			return trail.New(trail.Config{Bin: trailBin, IndexDir: indexDir, Env: env, Run: run}), nil
		},
	}
	return codeindex.New(cfg.Backends, factories)
}

func resolveCodeIndexPaths(projectRoot string, cfg policy.CodeIndexSettings) (indexDir, trailBin string, err error) {
	if filepath.IsAbs(cfg.IndexDir) && filepath.IsAbs(cfg.TrailBin) {
		return cfg.IndexDir, cfg.TrailBin, nil
	}
	hub, err := plane.ResolveHub(projectRoot)
	if err != nil {
		return "", "", fmt.Errorf("code_index.index_dir %q and code_index.trail_bin %q are relative to the hub root, but %s is not a hub worktree: %w",
			cfg.IndexDir, cfg.TrailBin, projectRoot, err)
	}
	return underHub(hub.Root, cfg.IndexDir), underHub(hub.Root, cfg.TrailBin), nil
}

func underHub(hubRoot, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(hubRoot, p)
}
