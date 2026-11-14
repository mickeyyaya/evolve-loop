package gc

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func ListOpenPaths(ctx context.Context, run sysexec.RunFunc) ([]string, error) {
	var out strings.Builder
	code, err := run(ctx, "lsof", "", []string{"-n", "-P", "-u", strconv.Itoa(os.Getuid()), "-F", "n"}, nil, nil, &out, nil)
	if err != nil {
		return nil, fmt.Errorf("list open paths: %w", err)
	}
	if code != 0 && out.Len() == 0 {
		return nil, fmt.Errorf("list open paths: lsof exit %d with no output", code)
	}
	return parseOpenPaths(out.String()), nil
}

func parseOpenPaths(listing string) []string {
	var paths []string
	sc := bufio.NewScanner(strings.NewReader(listing))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "n/") {
			paths = append(paths, line[1:])
		}
	}
	return paths
}

func LiveHolderProbe(ctx context.Context, run sysexec.RunFunc) func(path string) (bool, error) {
	var once sync.Once
	var open []string
	var listErr error
	return func(path string) (bool, error) {
		once.Do(func() { open, listErr = ListOpenPaths(ctx, run) })
		if listErr != nil {
			return false, listErr
		}
		return holdsAny(open, path), nil
	}
}

func holdsAny(open []string, dir string) bool {
	forms := []string{filepath.Clean(dir)}
	if real, err := filepath.EvalSymlinks(dir); err == nil && real != forms[0] {
		forms = append(forms, real)
	}
	for _, o := range open {
		for _, f := range forms {
			if o == f || strings.HasPrefix(o, f+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}
