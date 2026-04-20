#!/usr/bin/env python3
"""
setup_skill_inventory.py — Deterministic scan of installed 3pl skills and their
markdown companions. Writes a compact inventory JSON consumed by the evolve-loop
orchestrator at Phase 0 (CALIBRATE).

Replaces the LLM-side parsing of the system-reminder skill list described in
skills/evolve-loop/phases.md § "Skill Inventory". Deterministic filesystem scan
is cheaper (no tokens), more complete (every installed skill, not just what the
current session lists), and caches across sessions.

Usage:
    python3 scripts/setup_skill_inventory.py [--out .evolve/skill-inventory.json]
                                              [--force] [--quiet]

Scanned scopes (in precedence order):
    1. Project-local:   ./skills/**/SKILL.md
    2. User-global:     ~/.claude/skills/*/SKILL.md
    3. Plugin cache:    ~/.claude/plugins/cache/<marketplace>/<plugin>/<version>/skills/*/SKILL.md
                        (only the canonical "skills/" directory; IDE variants
                        like ".cursor/skills/" and ".kiro/skills/" are skipped)

For multiple versions of the same plugin, the newest version (lexicographic
max, which matches semver ordering for most cases) wins.

Output schema (aligned with state.json.skillInventory, see phases.md:110-125):

    {
      "lastBuilt": "<ISO-8601-UTC>",
      "scopes": {
        "project": <count>,
        "user": <count>,
        "plugin:<marketplace>:<plugin>": <count>
      },
      "categoryIndex": {
        "<category>": ["<skill-name>", ...],
        ...
      },
      "skills": {
        "<skill-name>": {
          "name": "<name>",
          "description": "<first 300 chars of description>",
          "origin": "<scope>",
          "path": "<absolute path to SKILL.md>",
          "referenceFiles": ["<sibling .md files>", ...],
          "categories": ["<category>", ...]
        }
      }
    }

Exit codes:
    0 — inventory written successfully
    1 — unrecoverable error (permission, missing HOME, etc.)
"""
from __future__ import annotations

import argparse
import datetime
import json
import os
import re
import sys
from pathlib import Path
from pydantic import BaseModel, Field
from typing import List, Dict

# --- Pydantic Schema Definitions (2026 Standards) ---
class SkillOutput(BaseModel):
    name: str
    description: str
    origin: str
    path: str
    referenceFiles: List[str]
    categories: List[str]

class InventorySchema(BaseModel):
    lastBuilt: str
    scopes: Dict[str, int]
    categoryIndex: Dict[str, List[str]]
    skills: Dict[str, SkillOutput]

# ─── Categorization rules ────────────────────────────────────────────────────
# Keyword → category. Matched against skill name + description (lowercased).
# Order matters: first match for a given category adds; a skill can match
# multiple categories. Keep in sync with phases.md § "Routing Categories".
CATEGORY_RULES: dict[str, list[str]] = {
    "code-review":  ["code review", "code-review", "pr review", "review patterns", "reviewer"],
    "testing":      ["tdd", "test generation", "testing", "test coverage", "unit test", "e2e"],
    "security":     ["security", "vulnerability", "owasp", "auth", "secret", "injection"],
    "e2e":          ["e2e", "end-to-end", "playwright", "browser test", "user flow"],
    "architecture": ["architecture", "architectural", "domain-driven", "ddd", "design pattern"],
    "debugging":    ["debug", "systematic debugging", "investigation", "root cause"],
    "performance":  ["performance", "profiling", "caching", "optimization", "bottleneck"],
    "frontend":     ["frontend", "ui ", "component", "react", "nextjs", "nuxt", "vue", "design system"],
    "database":     ["sql", "postgres", "orm", "migration", "schema", "jpa", "exposed orm"],
    "agent-design": ["agent pattern", "orchestration", "agent memory", "mcp server", "agentic"],
    "docs":         ["documentation", "api docs", "readme", "changelog", "technical writing"],
    "infra":        ["ci/cd", "cicd", "kubernetes", "docker", "container", "deployment"],
    "refactoring":  ["refactor", "code smell", "simplif", "dead code"],
}

# Skill-name patterns that map to language:<lang> / framework:<fw>
LANGUAGE_PATTERNS = {
    "python":     re.compile(r"(^|[-_])python([-_]|$)"),
    "typescript": re.compile(r"(^|[-_])(typescript|javascript|ts)([-_]|$)"),
    "go":         re.compile(r"(^|[-_])go(lang|[-_]|$)"),
    "rust":       re.compile(r"(^|[-_])rust([-_]|$)"),
    "java":       re.compile(r"(^|[-_])java([-_]|$)"),
    "kotlin":     re.compile(r"(^|[-_])kotlin([-_]|$)"),
    "swift":      re.compile(r"(^|[-_])swift([-_]|$)"),
    "cpp":        re.compile(r"(^|[-_])(cpp|c\+\+)([-_]|$)"),
    "csharp":     re.compile(r"(^|[-_])(csharp|c#|dotnet)([-_]|$)"),
    "dart":       re.compile(r"(^|[-_])(dart|flutter)([-_]|$)"),
    "perl":       re.compile(r"(^|[-_])perl([-_]|$)"),
    "php":        re.compile(r"(^|[-_])(php|laravel)([-_]|$)"),
}
FRAMEWORK_PATTERNS = {
    "django":     re.compile(r"django"),
    "laravel":    re.compile(r"laravel"),
    "springboot": re.compile(r"spring[-_]?boot"),
    "nestjs":     re.compile(r"nestjs"),
    "nextjs":     re.compile(r"next(js)?"),
    "nuxt":       re.compile(r"nuxt"),
    "flutter":    re.compile(r"flutter"),
}


# ─── Frontmatter parser ──────────────────────────────────────────────────────
FRONTMATTER_RE = re.compile(r"\A---\s*\n(.*?)\n---\s*\n", re.DOTALL)
KEY_VAL_RE = re.compile(r"^([a-zA-Z0-9_-]+)\s*:\s*(.*)$")


def parse_frontmatter(text: str) -> dict[str, str]:
    """Extract a flat YAML-ish frontmatter block. Handles simple `key: value`
    lines and continuation by leading whitespace. No nested structures — skill
    frontmatter is shallow in practice."""
    m = FRONTMATTER_RE.match(text)
    if not m:
        return {}
    block = m.group(1)
    out: dict[str, str] = {}
    current_key: str | None = None
    for raw in block.split("\n"):
        line = raw.rstrip()
        if not line:
            continue
        if line[0] in " \t" and current_key is not None:
            # continuation
            out[current_key] = (out[current_key] + " " + line.strip()).strip()
            continue
        kv = KEY_VAL_RE.match(line)
        if not kv:
            current_key = None
            continue
        key, value = kv.group(1).strip(), kv.group(2).strip()
        # strip quotes
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        out[key] = value
        current_key = key
    return out


# ─── Scope discovery ─────────────────────────────────────────────────────────
def project_skills_root(project_root: Path) -> Path:
    return project_root / "skills"


def user_skills_root() -> Path:
    return Path.home() / ".claude" / "skills"


def plugin_cache_root() -> Path:
    return Path.home() / ".claude" / "plugins" / "cache"


def gemini_user_skills_root() -> Path:
    return Path.home() / ".gemini" / "skills"


def gemini_extensions_root() -> Path:
    return Path.home() / ".gemini" / "extensions"


def newest_version_dir(plugin_dir: Path) -> Path | None:
    """For ~/.claude/plugins/cache/<marketplace>/<plugin>/, pick the newest
    version subdirectory (lexicographic max — matches semver for common cases).
    Returns None if no version dirs are present."""
    if not plugin_dir.is_dir():
        return None
    versions = [p for p in plugin_dir.iterdir() if p.is_dir()]
    if not versions:
        return None
    return max(versions, key=lambda p: p.name)


def find_skill_md_files(skills_dir: Path) -> list[Path]:
    """Return SKILL.md files under a canonical skills/ directory, exactly one
    level deep (<skills_dir>/<slug>/SKILL.md). Avoids recursive drift into
    unrelated nested directories."""
    if not skills_dir.is_dir():
        return []
    return sorted(
        (slug_dir / "SKILL.md")
        for slug_dir in skills_dir.iterdir()
        if slug_dir.is_dir() and (slug_dir / "SKILL.md").is_file()
    )


def sibling_reference_files(skill_md: Path) -> list[str]:
    """List sibling .md files next to SKILL.md (references, examples) and
    any files under a `references/` or `examples/` subdirectory. Returns
    paths relative to the skill directory."""
    root = skill_md.parent
    out: list[str] = []
    for child in sorted(root.iterdir()):
        if child.name == "SKILL.md":
            continue
        if child.is_file() and child.suffix == ".md":
            out.append(child.name)
        elif child.is_dir() and child.name in ("references", "examples"):
            for sub in sorted(child.rglob("*.md")):
                out.append(str(sub.relative_to(root)))
    return out


# ─── Categorization ──────────────────────────────────────────────────────────
def categorize(name: str, description: str) -> list[str]:
    haystack = f"{name} {description}".lower()
    cats: list[str] = []
    for category, keywords in CATEGORY_RULES.items():
        if any(kw in haystack for kw in keywords):
            cats.append(category)
    for lang, pat in LANGUAGE_PATTERNS.items():
        if pat.search(name.lower()):
            cats.append(f"language:{lang}")
            break  # one language per skill is enough
    for fw, pat in FRAMEWORK_PATTERNS.items():
        if pat.search(name.lower()):
            cats.append(f"framework:{fw}")
            break
    return cats


# ─── Scan orchestration ──────────────────────────────────────────────────────
def scan_project(project_root: Path) -> list[tuple[Path, str]]:
    out: list[tuple[Path, str]] = []
    for skill_md in find_skill_md_files(project_skills_root(project_root)):
        out.append((skill_md, "project"))
    return out


def scan_user_global() -> list[tuple[Path, str]]:
    claude_skills = [(p, "user") for p in find_skill_md_files(user_skills_root())]
    gemini_skills = [(p, "user") for p in find_skill_md_files(gemini_user_skills_root())]
    return claude_skills + gemini_skills


def scan_plugin_cache() -> list[tuple[Path, str]]:
    """Walk ~/.claude/plugins/cache/<marketplace>/<plugin>/<version>/skills/ and ~/.gemini/extensions/*/skills/."""
    out: list[tuple[Path, str]] = []
    
    # Claude plugins
    cache = plugin_cache_root()
    if cache.is_dir():
        for marketplace_dir in sorted(cache.iterdir()):
            if not marketplace_dir.is_dir():
                continue
            for plugin_dir in sorted(marketplace_dir.iterdir()):
                if not plugin_dir.is_dir():
                    continue
                version_dir = newest_version_dir(plugin_dir)
                if version_dir is None:
                    continue
                skills_dir = version_dir / "skills"
                origin = f"plugin:{marketplace_dir.name}:{plugin_dir.name}"
                for skill_md in find_skill_md_files(skills_dir):
                    out.append((skill_md, origin))
                    
    # Gemini extensions
    gemini_exts = gemini_extensions_root()
    if gemini_exts.is_dir():
        for ext_dir in sorted(gemini_exts.iterdir()):
            if not ext_dir.is_dir():
                continue
            skills_dir = ext_dir / "skills"
            origin = f"extension:{ext_dir.name}"
            for skill_md in find_skill_md_files(skills_dir):
                out.append((skill_md, origin))
                
    return out


def build_inventory(project_root: Path) -> dict:
    scopes = scan_project(project_root) + scan_user_global() + scan_plugin_cache()

    skills: dict[str, dict] = {}
    scope_counts: dict[str, int] = {}
    category_index: dict[str, list[str]] = {}
    skipped: list[str] = []

    for skill_md, origin in scopes:
        try:
            text = skill_md.read_text(encoding="utf-8", errors="replace")
        except OSError as e:
            skipped.append(f"{skill_md}: {e}")
            continue
        fm = parse_frontmatter(text)
        name = fm.get("name") or skill_md.parent.name
        description = fm.get("description", "").strip()
        # First-seen wins (project > user > plugin due to scan order).
        if name in skills:
            continue
        description_trim = description[:300]
        cats = categorize(name, description_trim)
        
        skill_data = {
            "name": name,
            "description": description_trim,
            "origin": origin,
            "path": str(skill_md),
            "referenceFiles": sibling_reference_files(skill_md),
            "categories": cats,
        }
        
        try:
            validated_skill = SkillOutput(**skill_data)
            skills[name] = validated_skill.model_dump()
            scope_counts[origin] = scope_counts.get(origin, 0) + 1
            for c in cats:
                category_index.setdefault(c, []).append(name)
        except Exception as e:
            print(f"[setup_skill_inventory] Warning: Skipping {skill_md} due to validation error: {e}", file=sys.stderr)
            skipped.append(str(skill_md))

    # Sort category index for stable output
    for c in category_index:
        category_index[c].sort()

    inventory_data = {
        "lastBuilt": datetime.datetime.now(datetime.UTC)
            .strftime("%Y-%m-%dT%H:%M:%SZ"),
        "scopes": dict(sorted(scope_counts.items())),
        "categoryIndex": dict(sorted(category_index.items())),
        "skills": dict(sorted(skills.items())),
    }
    
    try:
        validated_inventory = InventorySchema(**inventory_data)
        result = validated_inventory.model_dump()
        result["totalSkills"] = len(skills)
        result["skipped"] = skipped
        return result
    except Exception as e:
        print(f"[setup_skill_inventory] FATAL: Final inventory failed schema validation: {e}", file=sys.stderr)
        sys.exit(1)


# ─── Main ────────────────────────────────────────────────────────────────────
def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--out", default=".evolve/skill-inventory.json",
                        help="Output file (default: .evolve/skill-inventory.json)")
    parser.add_argument("--project-root", default=".",
                        help="Project root (default: .)")
    parser.add_argument("--force", action="store_true",
                        help="Ignore freshness cache; rebuild unconditionally")
    parser.add_argument("--quiet", action="store_true", help="Suppress summary output")
    args = parser.parse_args()

    project_root = Path(args.project_root).resolve()
    out_path = (project_root / args.out) if not Path(args.out).is_absolute() else Path(args.out)

    # Freshness check (1 hour, matching phases.md:84)
    if not args.force and out_path.is_file():
        try:
            existing = json.loads(out_path.read_text())
            built = datetime.datetime.strptime(existing["lastBuilt"], "%Y-%m-%dT%H:%M:%SZ")
            built = built.replace(tzinfo=datetime.UTC)
            age = (datetime.datetime.now(datetime.UTC) - built).total_seconds()
            if age < 3600:
                if not args.quiet:
                    print(f"[setup-skill-inventory] Cache hit ({age:.0f}s old, <3600s). "
                          f"Use --force to rebuild. {existing.get('totalSkills', 0)} skills.")
                return 0
        except (json.JSONDecodeError, KeyError, ValueError):
            pass  # stale or malformed — rebuild

    inventory = build_inventory(project_root)

    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(inventory, indent=2) + "\n")

    if not args.quiet:
        print(f"[setup-skill-inventory] Wrote {out_path}")
        print(f"[setup-skill-inventory] Indexed {inventory['totalSkills']} skills across {len(inventory['scopes'])} scopes")
        for scope, count in inventory["scopes"].items():
            print(f"  {scope}: {count}")
        print(f"[setup-skill-inventory] Categories: {len(inventory['categoryIndex'])}")
        top = sorted(inventory["categoryIndex"].items(), key=lambda kv: -len(kv[1]))[:5]
        for cat, names in top:
            print(f"  {cat}: {len(names)}")
        if inventory["skipped"]:
            print(f"[setup-skill-inventory] Skipped {len(inventory['skipped'])} unreadable files")
    return 0


if __name__ == "__main__":
    sys.exit(main())
