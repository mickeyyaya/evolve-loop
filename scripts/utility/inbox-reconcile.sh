#!/usr/bin/env bash
# inbox-reconcile.sh — Manual recovery escape hatches for .evolve/inbox/ (v9.6.1+, c38)
#
# Usage:
#   bash scripts/utility/inbox-reconcile.sh [--recover-all-orphans] \
#     [--force-skip-shipped <task_id>] [--help]
#
# Flags:
#   --recover-all-orphans            Recover all orphaned in-flight files back to inbox/
#   --force-skip-shipped <task_id>   Move inbox file for <task_id> to processed/cycle-0/
#   --help                           Print this usage
#
# Requires inbox-mover.sh in the same directory.
# Exits 0 always (best-effort recovery).

set -uo pipefail

__self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$__self_dir/../.." && pwd)"
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || echo "$REPO_ROOT")}"

MOVER="${__self_dir}/inbox-mover.sh"

usage() {
    awk '/^#!/{next} /^[^#]/{exit} /^#/{sub(/^# ?/,""); print}' "${BASH_SOURCE[0]}" >&2
}

if [ $# -eq 0 ]; then
    usage
    exit 0
fi

while [ $# -gt 0 ]; do
    case "$1" in
        --recover-all-orphans)
            echo "[inbox-reconcile] Running recover-orphans..." >&2
            EVOLVE_PROJECT_ROOT="$PROJECT_ROOT" bash "$MOVER" recover-orphans
            shift
            ;;
        --force-skip-shipped)
            TASK_ID="${2:-}"
            if [ -z "$TASK_ID" ]; then
                echo "ERROR: --force-skip-shipped requires <task_id>" >&2
                shift; continue
            fi
            echo "[inbox-reconcile] Force-skipping '$TASK_ID' → processed/cycle-0/" >&2
            EVOLVE_PROJECT_ROOT="$PROJECT_ROOT" bash "$MOVER" promote "$TASK_ID" processed 0
            shift 2
            ;;
        --help)
            usage
            exit 0
            ;;
        *)
            echo "ERROR: unknown argument: $1" >&2
            usage
            exit 0
            ;;
    esac
done

exit 0
