#!/usr/bin/env bash

# Bump version script for agentic-memorizer
# Usage: ./scripts/bump-version.sh [major|minor|patch]

set -e

VERSION_FILE="internal/version/VERSION"

# Validate arguments
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 [major|minor|patch]" >&2
    exit 1
fi

BUMP_TYPE="$1"

if [[ ! "$BUMP_TYPE" =~ ^(major|minor|patch)$ ]]; then
    echo "Error: Bump type must be major, minor, or patch" >&2
    exit 1
fi

# Read current version
if [ ! -f "$VERSION_FILE" ]; then
    echo "Error: VERSION file not found at $VERSION_FILE" >&2
    exit 1
fi

CURRENT_VERSION=$(cat "$VERSION_FILE" | tr -d '[:space:]')

# Validate version format (X.Y.Z)
if ! [[ $CURRENT_VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Invalid version format in $VERSION_FILE: $CURRENT_VERSION" >&2
    echo "Expected format: X.Y.Z" >&2
    exit 1
fi

# Parse version components
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"

# Calculate new version
case "$BUMP_TYPE" in
    major)
        NEW_VERSION="$((MAJOR + 1)).0.0"
        ;;
    minor)
        NEW_VERSION="$MAJOR.$((MINOR + 1)).0"
        ;;
    patch)
        NEW_VERSION="$MAJOR.$MINOR.$((PATCH + 1))"
        ;;
esac

# Write new version to temp file (with 'v' prefix for git tag)
echo "v$NEW_VERSION" > .next-version

echo "Current version: $CURRENT_VERSION"
echo "Next version: v$NEW_VERSION"
