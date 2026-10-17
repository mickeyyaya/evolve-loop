package fleet

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"sort"
	"strings"
)

func ChainAwareBenches(benched map[string]string, chains [][]string, info io.Writer) map[string]string {
	if len(benched) == 0 {
		return nil
	}
	if len(chains) == 0 {
		return maps.Clone(benched)
	}
	keys := make([]string, 0, len(benched))
	for key := range benched {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	down := map[string]string{}
	for _, key := range keys {
		fam := benchFamily(key)
		if _, seen := down[fam]; !seen {
			down[fam] = key
		}
	}
	costed := map[string]string{}
	for _, chain := range chains {
		if len(chain) > 0 && healthyIn(chain, down) == "" {
			key := down[chain[len(chain)-1]]
			costed[key] = benched[key]
		}
	}
	for _, key := range keys {
		if _, costs := costed[key]; !costs {
			fmt.Fprintf(info, "[loop] INFO: fleet: quota bench on CLI family %q (%s) costs no lane: %s\n",
				key, benched[key], fallbackOf(benchFamily(key), chains, down))
		}
	}
	return costed
}

func benchFamily(key string) string {
	fam, _, _ := strings.Cut(key, "@")
	return fam
}

func healthyIn(chain []string, down map[string]string) string {
	for _, fam := range chain {
		if _, benched := down[fam]; !benched {
			return fam
		}
	}
	return ""
}

func fallbackOf(fam string, chains [][]string, down map[string]string) string {
	counted := ""
	for _, chain := range chains {
		if !slices.Contains(chain, fam) {
			continue
		}
		if healthy := healthyIn(chain, down); healthy != "" {
			return fmt.Sprintf("a lane walks to %q", healthy)
		}
		counted = chain[len(chain)-1]
	}
	if counted == "" {
		return "no lane chain uses it"
	}
	return fmt.Sprintf("its chain is counted under %q", counted)
}
