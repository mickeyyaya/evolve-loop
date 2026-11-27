package skillcheck

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

type FileBudget struct {
	File  string `json:"file"`
	Bytes int    `json:"bytes"`
}

type HostProfile struct {
	Name             string       `json:"name"`
	SkillRoot        string       `json:"skill_root"`
	Frontmatter      []string     `json:"-"`
	DescriptionLimit int          `json:"description_limit"`
	Budgets          []FileBudget `json:"budgets"`
}

//go:embed templates/host_profiles.json
var hostProfilesJSON []byte

type hostProfileData struct {
	SpecFrontmatter []string `json:"spec_frontmatter"`
	Profiles        []struct {
		HostProfile
		HostFrontmatter []string `json:"host_frontmatter"`
	} `json:"profiles"`
}

func parseHostProfiles(raw []byte) ([]HostProfile, error) {
	var data hostProfileData
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("parse the host profiles: %w", err)
	}
	profiles := make([]HostProfile, 0, len(data.Profiles))
	for _, entry := range data.Profiles {
		p := entry.HostProfile
		p.Frontmatter = slices.Concat(data.SpecFrontmatter, entry.HostFrontmatter)
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func mustParseHostProfiles(raw []byte) []HostProfile {
	profiles, err := parseHostProfiles(raw)
	if err != nil {
		panic(err)
	}
	return profiles
}

var hostProfiles = mustParseHostProfiles(hostProfilesJSON)

func HostProfiles() []HostProfile {
	return slices.Clone(hostProfiles)
}

func hostFileBudget(file string) int {
	for _, p := range HostProfiles() {
		for _, b := range p.Budgets {
			if b.File == file {
				return b.Bytes
			}
		}
	}
	return 0
}

func hostSkillProblems(projectRoot string) ([]string, error) {
	var problems []string
	for _, p := range HostProfiles() {
		found, err := SkillProblems(projectRoot, p)
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
}

func SkillProblems(projectRoot string, p HostProfile) ([]string, error) {
	root := filepath.Join(projectRoot, filepath.FromSlash(p.SkillRoot))
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the %s skill root %s: %w", p.Name, p.SkillRoot, err)
	}
	var problems []string
	for _, e := range entries {
		raw, readErr := os.ReadFile(filepath.Join(root, e.Name(), "SKILL.md"))
		if readErr != nil {
			continue
		}
		fm, _, parseErr := prompts.ParseFrontmatter(string(raw))
		if parseErr != nil {
			continue
		}
		problems = append(problems, frontmatterProblems(p, p.SkillRoot+"/"+e.Name()+"/SKILL.md", fm)...)
	}
	return problems, nil
}

func frontmatterProblems(p HostProfile, rel string, fm map[string]any) []string {
	var problems []string
	if desc, _ := fm["description"].(string); utf8.RuneCountInString(desc) > p.DescriptionLimit {
		problems = append(problems, fmt.Sprintf("DESCRIPTION: %s description is %d characters; the %s limit is %d characters (shorten the description)", rel, utf8.RuneCountInString(desc), p.Name, p.DescriptionLimit))
	}
	keys := make([]string, 0, len(fm))
	for k := range fm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !slices.Contains(p.Frontmatter, k) {
			problems = append(problems, fmt.Sprintf("FRONTMATTER: %s has the key %q; the %s profile does not allow it", rel, k, p.Name))
		}
	}
	return problems
}
