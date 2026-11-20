package stageevidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const closureListTimeout = 60 * time.Second

type listedPkg struct {
	dir     string
	imports []string
	files   []string
}

func DecisionVersion(moduleDir string, closure []string, policy []byte, subtree string) (string, error) {
	if strings.TrimSpace(moduleDir) == "" {
		return "", errors.New("stageevidence: empty module dir")
	}
	modPath, modLines, err := readGoModDirectives(moduleDir)
	if err != nil {
		return "", err
	}
	subtreeBytes, present, err := canonicalPolicySubtree(policy, subtree)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	writeChunk(h, "gomod", []byte(strings.Join(modLines, "\n")))
	if err := hashClosure(h, moduleDir, modPath, closure); err != nil {
		return "", err
	}
	flag := byte(0)
	if present {
		flag = 1
	}
	writeChunk(h, "policy", append([]byte{flag}, subtreeBytes...))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeChunk(h hash.Hash, tag string, body []byte) {
	fmt.Fprintf(h, "%s:%d:", tag, len(body))
	_, _ = h.Write(body)
}

func readGoModDirectives(moduleDir string) (string, []string, error) {
	raw, err := os.ReadFile(filepath.Join(moduleDir, "go.mod"))
	if err != nil {
		return "", nil, fmt.Errorf("stageevidence: read go.mod: %w", err)
	}
	var modPath string
	var directives []string
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "module":
			if len(fields) > 1 {
				modPath = fields[1]
			}
			directives = append(directives, strings.Join(fields, " "))
		case "go", "toolchain":
			directives = append(directives, strings.Join(fields, " "))
		}
	}
	if modPath == "" {
		return "", nil, errors.New("stageevidence: go.mod has no module directive")
	}
	return modPath, directives, nil
}

func canonicalPolicySubtree(policy []byte, subtree string) ([]byte, bool, error) {
	dec := json.NewDecoder(bytes.NewReader(policy))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return nil, false, fmt.Errorf("stageevidence: parse policy: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, false, fmt.Errorf("stageevidence: %w", ErrTrailingData)
	}
	if subtree == "" {
		return nil, false, nil
	}
	cur := root
	for _, seg := range strings.Split(subtree, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		val, exists := obj[seg]
		if !exists {
			return nil, false, nil
		}
		cur = val
	}
	encoded, _ := json.Marshal(cur)
	return encoded, true, nil
}

func resolveClosureStarts(modPath string, closure []string) []string {
	var starts []string
	for _, c := range closure {
		c = strings.TrimPrefix(strings.TrimSpace(c), "./")
		if c == "" {
			continue
		}
		if c != modPath && !strings.HasPrefix(c, modPath+"/") {
			c = modPath + "/" + c
		}
		starts = append(starts, c)
	}
	return starts
}

func listClosurePackages(moduleDir string, starts []string) (map[string]listedPkg, map[string][]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), closureListTimeout)
	defer cancel()
	args := append([]string{"list", "-deps", "-f", `{{.ImportPath}}|{{.Dir}}|{{join .Imports " "}}|{{join .GoFiles " "}}|{{join .CgoFiles " "}}`}, starts...)
	stdout, err := sysexec.Output(ctx, sysexec.DefaultRunner, moduleDir, "go", args...)
	if err != nil {
		return nil, nil, fmt.Errorf("stageevidence: go list closure: %w", err)
	}
	pkgs, graph := parseClosureList(stdout)
	return pkgs, graph, nil
}

func parseClosureList(stdout string) (map[string]listedPkg, map[string][]string) {
	pkgs := map[string]listedPkg{}
	graph := map[string][]string{}
	for _, line := range strings.Split(stdout, "\n") {
		parts := strings.Split(line, "|")
		if len(parts) != 5 || parts[0] == "" {
			continue
		}
		imports := strings.Fields(parts[2])
		files := append(strings.Fields(parts[3]), strings.Fields(parts[4])...)
		sort.Strings(files)
		pkgs[parts[0]] = listedPkg{dir: parts[1], imports: imports, files: files}
		graph[parts[0]] = imports
	}
	return pkgs, graph
}

func hashClosure(h hash.Hash, moduleDir, modPath string, closure []string) error {
	starts := resolveClosureStarts(modPath, closure)
	if len(starts) == 0 {
		return nil
	}
	pkgs, graph, err := listClosurePackages(moduleDir, starts)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var local []string
	add := func(p string) {
		if !seen[p] && (p == modPath || strings.HasPrefix(p, modPath+"/")) {
			seen[p] = true
			local = append(local, p)
		}
	}
	for _, s := range starts {
		add(s)
	}
	for _, r := range regressiontia.ModuleLocalReach(graph, modPath, starts) {
		add(r)
	}
	sort.Strings(local)
	return hashListedPackages(h, local, pkgs)
}

func hashListedPackages(h hash.Hash, local []string, pkgs map[string]listedPkg) error {
	for _, pkg := range local {
		info, ok := pkgs[pkg]
		if !ok {
			return fmt.Errorf("stageevidence: %w: %s", ErrMissingPackage, pkg)
		}
		writeChunk(h, "pkg", []byte(pkg))
		for _, f := range info.files {
			body, err := os.ReadFile(filepath.Join(info.dir, f))
			if err != nil {
				return fmt.Errorf("stageevidence: read %s/%s: %w", pkg, f, err)
			}
			writeChunk(h, "file:"+f, body)
		}
	}
	return nil
}
