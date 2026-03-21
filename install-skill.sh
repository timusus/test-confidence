#!/bin/bash
# Install the confidence Claude skill globally
# This makes the skill available in all projects, not just this repo.

SKILL_DIR="$HOME/.claude/skills/confidence"
mkdir -p "$SKILL_DIR"
cp "$(dirname "$0")/.claude/skills/confidence/SKILL.md" "$SKILL_DIR/"
echo "Installed confidence skill to $SKILL_DIR"
