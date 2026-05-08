import pytest
import tempfile
import os
import json
from pathlib import Path
from setup_skill_inventory import (
    SkillOutput,
    CategoryIndex,
    InventorySchema,
    parse_frontmatter,
    categorize_skill,
    extract_lang_fw,
    process_skill_dir,
    write_inventory
)
from pydantic import ValidationError

def test_pydantic_schema_validation():
    # Valid input
    skill = SkillOutput(
        name="test-skill",
        description="A test skill",
        origin="project",
        path="/tmp/SKILL.md",
        referenceFiles=[],
        categories=["testing"]
    )
    assert skill.name == "test-skill"

    # Missing required field should throw ValidationError
    with pytest.raises(ValidationError):
        SkillOutput(
            description="A test skill",
            origin="project",
            path="/tmp/SKILL.md",
            referenceFiles=[],
            categories=["testing"]
        )

def test_parse_frontmatter():
    text = """---
name: my-skill
description: this is a description
---
# Content
"""
    fm = parse_frontmatter(text)
    assert fm["name"] == "my-skill"
    assert fm["description"] == "this is a description"

def test_process_skill_dir():
    with tempfile.TemporaryDirectory() as tmpdir:
        skill_dir = Path(tmpdir) / "my-skill"
        skill_dir.mkdir()
        skill_file = skill_dir / "SKILL.md"
        skill_file.write_text("""---
name: my-skill
description: code review tool
---
Content
""")
        ref_file = skill_dir / "reference.md"
        ref_file.write_text("Reference")
        
        result = process_skill_dir(skill_file, "project")
        assert result is not None
        assert result["name"] == "my-skill"
        assert result["origin"] == "project"
        assert "code-review" in result["categories"]
        assert "reference.md" in result["referenceFiles"]

def test_write_inventory():
    with tempfile.TemporaryDirectory() as tmpdir:
        out_file = Path(tmpdir) / "inventory.json"
        
        inventory_data = {
            "lastBuilt": "2026-04-20T10:00:00Z",
            "scopes": {"project": 1},
            "categoryIndex": {"testing": ["test-skill"]},
            "skills": {
                "test-skill": {
                    "name": "test-skill",
                    "description": "test",
                    "origin": "project",
                    "path": "/tmp/SKILL.md",
                    "referenceFiles": [],
                    "categories": ["testing"]
                }
            }
        }
        
        # This should succeed if the data matches the Pydantic schema
        write_inventory(inventory_data, out_file, quiet=True)
        
        assert out_file.exists()
        loaded = json.loads(out_file.read_text())
        assert loaded["skills"]["test-skill"]["name"] == "test-skill"
