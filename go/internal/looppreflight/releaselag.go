package looppreflight

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	releaseLagCheckName = "claude-release-lag"
	releaseLagBin       = "claude"
	releaseLagCacheFile = "release-lag-cache.json"
	releaseLagUpdateHow = "verify then update by hand: run `claude update`, then `evolve doctor live claude-tmux`; the loop never updates the CLI itself"
)

type ReleaseLagConfig struct {
	Threshold int
	CacheTTL  time.Duration
}

func defaultReleaseLagConfig(c ReleaseLagConfig) ReleaseLagConfig {
	def := policy.Policy{}.PreflightConfig()
	if c.Threshold <= 0 {
		c.Threshold = def.ReleaseLagThreshold
	}
	if c.CacheTTL <= 0 {
		c.CacheTTL = def.ReleaseCheckTTL
	}
	return c
}

type releaseLagCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func checkReleaseLag(o resolved) CheckResult {
	pass := func(msg string) CheckResult {
		return CheckResult{Name: releaseLagCheckName, Level: LevelPass, Message: msg}
	}
	if o.latestRelease == nil {
		return pass("no release source wired; claude release lag not checked")
	}
	installed := o.versionInventory()[releaseLagBin]
	if installed == "" {
		return pass("claude release lag not checked: installed version unknown")
	}
	cachePath := filepath.Join(o.evolveDir, releaseLagCacheFile)
	now := o.now()
	if cached, ok := readReleaseLagCache(cachePath); ok && now.Sub(cached.CheckedAt) < o.releaseLag.CacheTTL && !cached.CheckedAt.After(now) {
		return pass(fmt.Sprintf("claude release lag checked at %s; next lookup after the cache window", cached.CheckedAt.UTC().Format(time.RFC3339)))
	}
	latest, err := o.latestRelease(releaseLagBin)
	entry := releaseLagCache{CheckedAt: now, Latest: latest}
	if err != nil {
		entry.Error = err.Error()
	}
	saveNote := ""
	if werr := atomicwrite.JSON(cachePath, entry); werr != nil {
		saveNote = fmt.Sprintf("; release cache not saved: %v", werr)
	}
	if err != nil {
		return pass(fmt.Sprintf("claude release lag not checked: release lookup failed (%v)%s", err, saveNote))
	}
	if !IsLaggingBy(installed, latest, o.releaseLag.Threshold) {
		return pass(fmt.Sprintf("claude %s is within %d patch(es) of the latest release %s%s", installed, o.releaseLag.Threshold, latest, saveNote))
	}
	return CheckResult{Name: releaseLagCheckName, Level: LevelWarn,
		Message: fmt.Sprintf("claude %s lags the latest release %s by more than %d patch(es); the tier aliases resolve inside the installed build, so a pinned build can run an older model", installed, latest, o.releaseLag.Threshold),
		Detail:  releaseLagUpdateHow + saveNote,
	}
}

func readReleaseLagCache(path string) (releaseLagCache, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return releaseLagCache{}, false
	}
	var c releaseLagCache
	if json.Unmarshal(data, &c) != nil || c.CheckedAt.IsZero() {
		return releaseLagCache{}, false
	}
	return c, true
}
