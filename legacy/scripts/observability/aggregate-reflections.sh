#!/usr/bin/env bash
#
# aggregate-reflections.sh — Cross-cycle aggregator for the Reflection Journal.
#
# Reads <phase>-reflection.yaml sidecars across the last N cycles and produces
# a rollup grouped by slowdown category, upstream friction source, and
# recurring improvement suggestion.
#
# Schema reference: agents/agent-templates.md → Reflection Journal Schema
# Design doc:       docs/architecture/reflection-journal.md
#
# Usage:
#   bash legacy/scripts/observability/aggregate-reflections.sh                    # default --window 5, human format
#   bash legacy/scripts/observability/aggregate-reflections.sh --window 10
#   bash legacy/scripts/observability/aggregate-reflections.sh --phase scout      # filter by phase
#   bash legacy/scripts/observability/aggregate-reflections.sh --format=json      # emit JSON for dashboard.sh
#   bash legacy/scripts/observability/aggregate-reflections.sh --runs-dir <path>  # override .evolve/runs/ (testing)
#
# Exit codes:
#   0 — rollup produced
#   1 — no reflection YAMLs found in window
#  10 — bad arguments
#
# bash 3.2 compatible — no declare -A, no mapfile, no GNU-only flags.
# YAML parsing is grep-based against the constrained schema (see schema ref above).

set -uo pipefail

WINDOW=5
PHASE_FILTER=""
FORMAT="human"
RUNS_DIR=""
MIN_CONFIDENCE="0.5"

while [ $# -gt 0 ]; do
    case "$1" in
        --window=*) WINDOW="${1#*=}" ;;
        --window) shift; WINDOW="${1:-}" ;;
        --phase=*) PHASE_FILTER="${1#*=}" ;;
        --phase) shift; PHASE_FILTER="${1:-}" ;;
        --format=*) FORMAT="${1#*=}" ;;
        --format) shift; FORMAT="${1:-}" ;;
        --runs-dir=*) RUNS_DIR="${1#*=}" ;;
        --runs-dir) shift; RUNS_DIR="${1:-}" ;;
        --min-confidence=*) MIN_CONFIDENCE="${1#*=}" ;;
        --help|-h) sed -n '2,28p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[aggregate-reflections] unknown flag: $1" >&2; exit 10 ;;
        *) echo "[aggregate-reflections] unexpected arg: $1" >&2; exit 10 ;;
    esac
    shift
done

case "$FORMAT" in
    human|json) ;;
    *) echo "[aggregate-reflections] --format must be human or json (got: $FORMAT)" >&2; exit 10 ;;
esac

# Resolve runs dir
if [ -z "$RUNS_DIR" ]; then
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
    RUNS_DIR="$REPO_ROOT/.evolve/runs"
fi

if [ ! -d "$RUNS_DIR" ]; then
    echo "[aggregate-reflections] runs dir not found: $RUNS_DIR" >&2
    exit 1
fi

# Collect cycle dirs (cycle-N), sorted numerically descending, take WINDOW most recent
TMP_BASE="${TMPDIR:-/tmp}/evolve-aggregate-reflections.$$"
mkdir -p "$TMP_BASE"
trap 'rm -rf "$TMP_BASE"' EXIT

CYCLE_LIST="$TMP_BASE/cycle-list"
: > "$CYCLE_LIST"

# bash 3.2: no mapfile. Use find + while-read.
find "$RUNS_DIR" -maxdepth 1 -type d -name 'cycle-*' 2>/dev/null \
    | while IFS= read -r dir; do
        bn="${dir##*/}"
        n="${bn#cycle-}"
        # Accept only purely-numeric suffixes (skip archive/cycle-XX-foo dirs)
        case "$n" in
            *[!0-9]*) continue ;;
            "") continue ;;
        esac
        printf '%s\t%s\n' "$n" "$dir"
    done | sort -t '	' -k1,1 -rn | head -n "$WINDOW" > "$CYCLE_LIST"

CYCLE_COUNT=$(wc -l < "$CYCLE_LIST" | tr -d ' ')
if [ "$CYCLE_COUNT" = "0" ]; then
    if [ "$FORMAT" = "json" ]; then
        printf '{"window":%s,"cycles_scanned":0,"reflections_found":0,"slowdown_categories":[],"friction_sources":[],"top_suggestions":[]}\n' "$WINDOW"
    else
        echo "[aggregate-reflections] no cycle dirs under $RUNS_DIR"
    fi
    exit 1
fi

# For each cycle dir, collect reflection YAMLs (optionally filtered by phase)
YAML_LIST="$TMP_BASE/yaml-list"
: > "$YAML_LIST"

while IFS=$'\t' read -r cyc dir; do
    if [ -n "$PHASE_FILTER" ]; then
        pattern="${PHASE_FILTER}-reflection.yaml"
    else
        pattern="*-reflection.yaml"
    fi
    find "$dir" -maxdepth 2 -type f -name "$pattern" 2>/dev/null \
        | while IFS= read -r yml; do
            printf '%s\t%s\n' "$cyc" "$yml" >> "$YAML_LIST"
        done
done < "$CYCLE_LIST"

YAML_COUNT=$(wc -l < "$YAML_LIST" | tr -d ' ')
if [ "$YAML_COUNT" = "0" ]; then
    if [ "$FORMAT" = "json" ]; then
        printf '{"window":%s,"cycles_scanned":%s,"reflections_found":0,"slowdown_categories":[],"friction_sources":[],"top_suggestions":[]}\n' "$WINDOW" "$CYCLE_COUNT"
    else
        echo "[aggregate-reflections] $CYCLE_COUNT cycles scanned, no reflection YAMLs found"
    fi
    exit 1
fi

# Counters as plain-text files (bash 3.2 — no associative arrays)
CAT_FILE="$TMP_BASE/categories"
FRICTION_FILE="$TMP_BASE/friction"
SUGGEST_FILE="$TMP_BASE/suggestions"
CYCLES_PER_CAT_FILE="$TMP_BASE/cycles-per-category"
: > "$CAT_FILE"
: > "$FRICTION_FILE"
: > "$SUGGEST_FILE"
: > "$CYCLES_PER_CAT_FILE"

# Parse each YAML
while IFS=$'\t' read -r cyc yml; do
    # Skip low-confidence reflections (signal noise)
    conf=$(grep -E '^reflection_confidence:[[:space:]]' "$yml" 2>/dev/null \
        | head -n 1 \
        | awk -F: '{gsub(/[[:space:]]/,"",$2); print $2}')
    if [ -n "$conf" ]; then
        # awk compares as numbers; bash 3.2 has no float comparison
        keep=$(awk -v c="$conf" -v m="$MIN_CONFIDENCE" 'BEGIN { print (c+0 >= m+0) ? "1" : "0" }')
        if [ "$keep" = "0" ]; then
            continue
        fi
    fi

    # Extract slowdown categories — lines like "  - category: research-quota"
    grep -E '^[[:space:]]*-[[:space:]]+category:[[:space:]]' "$yml" 2>/dev/null \
        | awk -F'category:' '{gsub(/^[[:space:]]+|[[:space:]]+$|#.*$/,"",$2); print $2}' \
        | while IFS= read -r cat; do
            [ -z "$cat" ] && continue
            printf '%s\n' "$cat" >> "$CAT_FILE"
            printf '%s\t%s\n' "$cat" "$cyc" >> "$CYCLES_PER_CAT_FILE"
        done

    # Extract friction upstream phases — lines like "  - upstream_phase: scout"
    # (upstream_phase is the first key in each friction_received_from array
    #  element, so it sits on the same line as the YAML dash; allow optional `- `
    #  between leading whitespace and the field name.)
    grep -E '^[[:space:]]*-?[[:space:]]*upstream_phase:[[:space:]]' "$yml" 2>/dev/null \
        | awk -F'upstream_phase:' '{gsub(/^[[:space:]]+|[[:space:]]+$|#.*$/,"",$2); print $2}' \
        | while IFS= read -r up; do
            [ -z "$up" ] && continue
            # Pair "upstream → this_phase" where this_phase derived from filename
            ph=$(basename "$yml" | sed -E 's/-reflection\.yaml$//')
            printf '%s\t%s\n' "$up" "$ph" >> "$FRICTION_FILE"
        done

    # Extract suggested-improvement actions — lines like '  - action: "Bump kb-search quota to 30"'
    # (action is the first key in each suggested_improvements array element.)
    grep -E '^[[:space:]]*-?[[:space:]]*action:[[:space:]]' "$yml" 2>/dev/null \
        | awk -F'action:' '{
            v = $2
            sub(/^[[:space:]]+/, "", v)
            sub(/^"/, "", v); sub(/"$/, "", v)
            gsub(/[[:space:]]+#.*$/, "", v)
            print v
          }' \
        | while IFS= read -r act; do
            [ -z "$act" ] && continue
            ph=$(basename "$yml" | sed -E 's/-reflection\.yaml$//')
            printf '%s\t%s\t%s\n' "$act" "$ph" "$cyc" >> "$SUGGEST_FILE"
        done
done < "$YAML_LIST"

# Compute rollups
CAT_ROLLUP="$TMP_BASE/cat-rollup"
FRICTION_ROLLUP="$TMP_BASE/friction-rollup"
SUGGEST_ROLLUP="$TMP_BASE/suggest-rollup"

# Slowdown categories: count distinct cycles per category (so "research-quota 4/5"
# means 4-of-5 cycles experienced it, not 4 total events).
sort -u "$CYCLES_PER_CAT_FILE" 2>/dev/null \
    | awk -F'\t' '{count[$1]++} END {for (c in count) print count[c]"\t"c}' \
    | sort -rn -k1,1 > "$CAT_ROLLUP"

# Friction sources: count upstream→downstream pairs
sort "$FRICTION_FILE" 2>/dev/null \
    | uniq -c \
    | awk '{n=$1; sub(/^[[:space:]]*[0-9]+[[:space:]]+/, ""); print n"\t"$0}' \
    | sort -rn -k1,1 > "$FRICTION_ROLLUP"

# Top suggestions: count distinct cycles per (action, phase)
sort -u "$SUGGEST_FILE" 2>/dev/null \
    | awk -F'\t' '{key=$2"\t"$1; cycles[key]++; cyclist[key]=cyclist[key]","$3}
                   END {for (k in cycles) {gsub(/^,/, "", cyclist[k]); print cycles[k]"\t"k"\t"cyclist[k]}}' \
    | sort -rn -k1,1 > "$SUGGEST_ROLLUP"

# ---- Emit ----
if [ "$FORMAT" = "json" ]; then
    printf '{"window":%s,"cycles_scanned":%s,"reflections_found":%s,"slowdown_categories":[' \
        "$WINDOW" "$CYCLE_COUNT" "$YAML_COUNT"
    first=1
    while IFS=$'\t' read -r n cat; do
        [ -z "$n" ] && continue
        if [ "$first" = "1" ]; then first=0; else printf ','; fi
        printf '{"category":"%s","cycles":%s}' "$cat" "$n"
    done < "$CAT_ROLLUP"
    printf '],"friction_sources":['
    first=1
    while IFS=$'\t' read -r n pair; do
        [ -z "$n" ] && continue
        up=$(printf '%s' "$pair" | awk '{print $1}')
        down=$(printf '%s' "$pair" | awk '{print $2}')
        if [ "$first" = "1" ]; then first=0; else printf ','; fi
        printf '{"upstream":"%s","downstream":"%s","occurrences":%s}' "$up" "$down" "$n"
    done < "$FRICTION_ROLLUP"
    printf '],"top_suggestions":['
    first=1
    while IFS=$'\t' read -r n ph act cyc; do
        [ -z "$n" ] && continue
        # Escape double-quotes in action text for JSON
        act_esc=$(printf '%s' "$act" | sed 's/"/\\"/g')
        if [ "$first" = "1" ]; then first=0; else printf ','; fi
        printf '{"phase":"%s","action":"%s","cycle_count":%s,"cycles":"%s"}' "$ph" "$act_esc" "$n" "$cyc"
    done < "$SUGGEST_ROLLUP"
    printf ']}\n'
    exit 0
fi

# Human-readable rollup
echo "Reflection rollup — last $CYCLE_COUNT cycle(s) (window=$WINDOW, min_confidence=$MIN_CONFIDENCE)"
echo "Reflections scanned: $YAML_COUNT"
echo ""

echo "By slowdown category (cycles affected / total scanned):"
if [ -s "$CAT_ROLLUP" ]; then
    while IFS=$'\t' read -r n cat; do
        [ -z "$n" ] && continue
        bar=""
        i=0
        while [ "$i" -lt "$n" ]; do bar="${bar}█"; i=$((i+1)); done
        printf '  %-24s %s/%s  %s\n' "$cat" "$n" "$CYCLE_COUNT" "$bar"
    done < "$CAT_ROLLUP"
else
    echo "  (none — phase_smooth=true across the window)"
fi
echo ""

echo "Upstream friction sources (upstream → downstream, occurrences):"
if [ -s "$FRICTION_ROLLUP" ]; then
    while IFS=$'\t' read -r n pair; do
        [ -z "$n" ] && continue
        up=$(printf '%s' "$pair" | awk '{print $1}')
        down=$(printf '%s' "$pair" | awk '{print $2}')
        printf '  %-10s → %-10s %s occurrence(s)\n' "$up" "$down" "$n"
    done < "$FRICTION_ROLLUP"
else
    echo "  (none reported)"
fi
echo ""

echo "Top recurring suggestions (cycle-count, phase, action):"
if [ -s "$SUGGEST_ROLLUP" ]; then
    head -n 10 "$SUGGEST_ROLLUP" | while IFS=$'\t' read -r n ph act cyc; do
        [ -z "$n" ] && continue
        printf '  [%s cycles] %s: %s (cycles=%s)\n' "$n" "$ph" "$act" "$cyc"
    done
else
    echo "  (no recurring suggestions in window)"
fi
