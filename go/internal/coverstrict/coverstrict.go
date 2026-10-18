package coverstrict

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
)

type Floor struct {
	Pkg string  `json:"pkg"`
	Min float64 `json:"min"`
}

type Result struct {
	Pkg        string  `json:"pkg"`
	Min        float64 `json:"min"`
	Percent    float64 `json:"percent"`
	Statements int     `json:"statements"`
	Covered    int     `json:"covered"`
	Problem    string  `json:"problem,omitempty"`
}

func (r Result) OK() bool { return r.Problem == "" }

func (r Result) Line() string {
	if r.Problem != "" {
		return fmt.Sprintf("cover strict: %s: %s FAIL", r.Pkg, r.Problem)
	}
	return fmt.Sprintf("cover strict: %s: %.1f%% >= %.1f%% ok", r.Pkg, r.Percent, r.Min)
}

var floorNumber = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

func ParseFloors(data []byte) ([]Floor, error) {
	var floors []Floor
	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		floor, err := parseFloor(line)
		if err != nil {
			return nil, fmt.Errorf("coverstrict: floors line %d: %w", n+1, err)
		}
		floors = append(floors, floor)
	}
	return floors, nil
}

func parseFloor(line string) (Floor, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return Floor{}, fmt.Errorf("%q is not `<package> <minimum percent>`", line)
	}
	pkg := path.Clean(fields[0])
	if !fs.ValidPath(pkg) || strings.Contains(pkg, "...") {
		return Floor{}, fmt.Errorf("%q is not a package directory under the module", fields[0])
	}
	if !floorNumber.MatchString(fields[1]) {
		return Floor{}, fmt.Errorf("the floor %q is not a number", fields[1])
	}
	floor, _ := strconv.ParseFloat(fields[1], 64)
	if floor > 100 {
		return Floor{}, fmt.Errorf("the floor %q is over 100", fields[1])
	}
	return Floor{Pkg: pkg, Min: floor}, nil
}

func Clean(results []Result) bool {
	for _, r := range results {
		if !r.OK() {
			return false
		}
	}
	return true
}

func Measure(floors []Floor, profile covchanged.Profile) []Result {
	results := make([]Result, 0, len(floors))
	for _, f := range floors {
		results = append(results, measureFloor(f, profile))
	}
	return results
}

func measureFloor(f Floor, profile covchanged.Profile) Result {
	r := Result{Pkg: f.Pkg, Min: f.Min}
	for file, blocks := range profile {
		if path.Dir(file) != f.Pkg {
			continue
		}
		for _, b := range blocks {
			r.Statements += b.Statements
			if b.Hit {
				r.Covered += b.Statements
			}
		}
	}
	if r.Statements == 0 {
		r.Problem = "no statements in the profile"
		return r
	}
	r.Percent = 100 * float64(r.Covered) / float64(r.Statements)
	if r.Percent < r.Min {
		r.Problem = belowFloor(r)
	}
	return r
}

func belowFloor(r Result) string {
	short := fmt.Sprintf("%.1f", r.Percent)
	if rounded, _ := strconv.ParseFloat(short, 64); rounded < r.Min {
		return fmt.Sprintf("%s%% < %.1f%%", short, r.Min)
	}
	truncated := math.Floor(r.Percent*100) / 100
	return fmt.Sprintf("%.2f%% (%d of %d statements) < %.1f%%", truncated, r.Covered, r.Statements, r.Min)
}

func Check(moduleDir, profilePath, floorsPath string) ([]Result, error) {
	floorData, err := os.ReadFile(floorsPath)
	if err != nil {
		return nil, fmt.Errorf("coverstrict: read the floors file: %w", err)
	}
	floors, err := ParseFloors(floorData)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(profilePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("coverstrict: no coverage profile at %s; run `make -C go test-integration` first, then run this check again", profilePath)
	}
	if err != nil {
		return nil, fmt.Errorf("coverstrict: read the profile: %w", err)
	}
	for _, f := range floors {
		if _, err := os.Stat(filepath.Join(moduleDir, filepath.FromSlash(f.Pkg))); err != nil {
			return nil, fmt.Errorf("coverstrict: list the sources of %s: %w", f.Pkg, err)
		}
	}
	if err := refuseStale(moduleDir, profilePath, info.ModTime().UnixNano()); err != nil {
		return nil, err
	}
	module, err := covchanged.ModulePath(moduleDir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("coverstrict: read the profile: %w", err)
	}
	profile, err := covchanged.ParseProfile(data, module)
	if err != nil {
		return nil, err
	}
	return Measure(floors, profile), nil
}

var entryInfo = func(d fs.DirEntry) (fs.FileInfo, error) { return d.Info() }

func refuseStale(moduleDir, profilePath string, measuredNano int64) error {
	return filepath.WalkDir(moduleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("coverstrict: walk %s: %w", path, err)
		}
		if d.IsDir() {
			if path != moduleDir && skippedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !watchedFile(d.Name()) {
			return nil
		}
		info, err := entryInfo(d)
		if err != nil {
			return fmt.Errorf("coverstrict: stat %s: %w", path, err)
		}
		if info.ModTime().UnixNano() > measuredNano {
			return fmt.Errorf("coverstrict: the profile %s is older than %s; run `make -C go test-integration` again, then run this check again", profilePath, path)
		}
		return nil
	})
}

func skippedDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".")
}

func watchedFile(name string) bool {
	return name == "go.mod" || name == "go.sum" || strings.HasSuffix(name, ".go")
}
