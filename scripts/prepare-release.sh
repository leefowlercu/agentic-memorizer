#!/bin/bash
set -e

# Cleanup temporary files on exit
trap 'rm -f .next-version' EXIT

VERSION=$1

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Merge GoReleaser changelog into Keep a Changelog format
# Arguments:
#   $1 - Version (e.g., v0.11.0)
# Returns:
#   0 on success, 1 on failure
merge_changelog() {
    local version="$1"
    local version_number="${version#v}"
    local dist_changelog="dist/CHANGELOG.md"
    local root_changelog="CHANGELOG.md"
    local repo_url="https://github.com/leefowlercu/agentic-memorizer"

    # Edge case 1: dist/CHANGELOG.md doesn't exist
    if [ ! -f "$dist_changelog" ]; then
        echo -e "${YELLOW}Warning: $dist_changelog not found${NC}"
        echo -e "${YELLOW}Skipping changelog merge${NC}"
        return 0
    fi

    # Edge case 2: dist/CHANGELOG.md is empty
    if [ ! -s "$dist_changelog" ]; then
        echo -e "${YELLOW}Warning: $dist_changelog is empty (no commits since last release)${NC}"
        echo -e "${YELLOW}Skipping changelog merge${NC}"
        return 0
    fi

    # Edge case 3: Version already exists
    if grep -q "^## \[${version_number}\]" "$root_changelog"; then
        echo -e "${RED}Error: Version ${version_number} already exists in $root_changelog${NC}"
        return 1
    fi

    # Get date from tag
    local tag_date
    tag_date=$(git log -1 --format=%cd --date=short "$version")

    # Build version header
    local new_section="## [${version_number}] - ${tag_date}"

    # Parse dist/CHANGELOG.md and transform to Keep a Changelog format
    local current_section=""
    local section_content=""
    local all_content=""

    while IFS= read -r line; do
        # Skip "## Changelog" header
        if [[ "$line" == "## Changelog" ]]; then
            continue
        fi

        # Detect section headers
        if [[ "$line" =~ ^###[[:space:]](.+)$ ]]; then
            # Save previous section if it has content
            if [ -n "$current_section" ] && [ -n "$section_content" ]; then
                all_content+="### ${current_section}"$'\n'
                all_content+="${section_content}"$'\n'
            fi

            # Start new section
            current_section="${BASH_REMATCH[1]}"
            section_content=""

        # Transform entry lines
        elif [[ "$line" =~ ^\*[[:space:]][a-f0-9]{7}[[:space:]](.+)$ ]]; then
            # Extract message and capitalize first letter
            local message="${BASH_REMATCH[1]}"
            local first_char="${message:0:1}"
            local rest="${message:1}"
            message="$(echo "$first_char" | tr '[:lower:]' '[:upper:]')${rest}"

            section_content+="- ${message}"$'\n'
        fi
    done < "$dist_changelog"

    # Add final section
    if [ -n "$current_section" ] && [ -n "$section_content" ]; then
        all_content+="### ${current_section}"$'\n'
        all_content+="${section_content}"
    fi

    # Edge case 4: No content extracted
    if [ -z "$all_content" ]; then
        echo -e "${YELLOW}Warning: No changelog entries found in $dist_changelog${NC}"
        echo -e "${YELLOW}Skipping changelog merge${NC}"
        return 0
    fi

    # Build complete new version section
    local new_version_block="${new_section}"$'\n'$'\n'
    new_version_block+="${all_content}"

    # Find insertion point (first version section)
    local first_version_line
    first_version_line=$(grep -n '^## \[' "$root_changelog" | head -1 | cut -d: -f1)

    if [ -z "$first_version_line" ]; then
        echo -e "${RED}Error: Could not find version sections in $root_changelog${NC}"
        return 1
    fi

    # Insert new version section
    {
        head -n $((first_version_line - 1)) "$root_changelog"
        echo "$new_version_block"
        tail -n "+${first_version_line}" "$root_changelog"
    } > "${root_changelog}.new"

    mv "${root_changelog}.new" "$root_changelog"

    # Update footer links
    local links_start
    links_start=$(grep -n '^\[[0-9]' "$root_changelog" | head -1 | cut -d: -f1)

    if [ -z "$links_start" ]; then
        # Edge case 5: No links section - append at end
        echo "" >> "$root_changelog"

        # Edge case 6: Check if this is first release
        local prev_tag
        prev_tag=$(git describe --tags --abbrev=0 "${version}^" 2>/dev/null)

        if [ -z "$prev_tag" ]; then
            echo "[${version_number}]: ${repo_url}/releases/tag/${version}" >> "$root_changelog"
        else
            echo "[${version_number}]: ${repo_url}/compare/${prev_tag}...${version}" >> "$root_changelog"
        fi
    else
        # Insert new link at top of links section
        local prev_tag
        prev_tag=$(git describe --tags --abbrev=0 "${version}^" 2>/dev/null)

        local new_link
        if [ -z "$prev_tag" ]; then
            new_link="[${version_number}]: ${repo_url}/releases/tag/${version}"
        else
            new_link="[${version_number}]: ${repo_url}/compare/${prev_tag}...${version}"
        fi

        # Use sed to insert before links_start
        sed -i.bak "${links_start}i\\
${new_link}
" "$root_changelog"
        rm -f "${root_changelog}.bak"
    fi

    return 0
}

# Validate version argument
if [ -z "$VERSION" ]; then
    echo -e "${RED}Error: VERSION argument is required${NC}"
    echo "Usage: $0 v0.11.0"
    exit 1
fi

# Validate version format (vX.Y.Z)
if ! [[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo -e "${RED}Error: VERSION must be in format vX.Y.Z (e.g., v0.11.0)${NC}"
    exit 1
fi

# Extract version without 'v' prefix
VERSION_NUMBER=${VERSION#v}

echo -e "${GREEN}Preparing release ${VERSION}...${NC}"

# Check if tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo -e "${RED}Error: Tag $VERSION already exists${NC}"
    exit 1
fi

# Update version in internal/version/VERSION
echo -e "${YELLOW}Updating VERSION file...${NC}"
VERSION_FILE="internal/version/VERSION"
if [ ! -f "$VERSION_FILE" ]; then
    echo -e "${RED}Error: $VERSION_FILE not found${NC}"
    exit 1
fi

# Write new version to VERSION file (without 'v' prefix)
echo "$VERSION_NUMBER" > "$VERSION_FILE"

echo -e "${GREEN}Updated $VERSION_FILE to $VERSION_NUMBER${NC}"

# Stage version file
git add "$VERSION_FILE"

# Commit version change
echo -e "${YELLOW}Creating version bump commit...${NC}"
COMMIT_MESSAGE="release: prepare ${VERSION}"
git commit -m "$COMMIT_MESSAGE"
echo -e "${GREEN}Created commit: $COMMIT_MESSAGE${NC}"

# Create temporary tag for GoReleaser validation (will be recreated after amending)
echo -e "${YELLOW}Creating temporary git tag ${VERSION} for GoReleaser...${NC}"
git tag -a "$VERSION" -m "Agentic Memorizer $VERSION"
echo -e "${GREEN}Created temporary tag: $VERSION${NC}"

# Check for GoReleaser and GITHUB_TOKEN
if ! command -v goreleaser &> /dev/null; then
    echo -e "${RED}Error: goreleaser is not installed${NC}"
    echo -e "${YELLOW}Install with: go install github.com/goreleaser/goreleaser/v2@latest${NC}"
    git tag -d "$VERSION"
    git reset --hard HEAD~1
    exit 1
fi

if [ -z "$GITHUB_TOKEN" ]; then
    echo -e "${RED}Warning: GITHUB_TOKEN environment variable not set${NC}"
    echo -e "${YELLOW}GoReleaser needs this to create GitHub releases${NC}"
    echo -e "${YELLOW}See README.md for setup instructions${NC}"
    echo ""
    echo -e "${YELLOW}Continue anyway? Release will fail but you can retry later. (y/N)${NC}"
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Undoing tag and commit...${NC}"
        git tag -d "$VERSION"
        git reset --hard HEAD~1
        exit 1
    fi
fi

# Run GoReleaser to create draft release and generate changelog
echo -e "${YELLOW}Running GoReleaser...${NC}"
if ! goreleaser release --clean; then
    echo -e "${RED}Error: GoReleaser failed${NC}"
    echo -e "${YELLOW}Undoing tag and commit...${NC}"
    git tag -d "$VERSION"
    git reset --hard HEAD~1
    exit 1
fi

echo -e "${GREEN}GoReleaser completed successfully!${NC}"

# Merge dist/CHANGELOG.md into root CHANGELOG.md
echo -e "${YELLOW}Merging changelog...${NC}"
if ! merge_changelog "$VERSION"; then
    echo -e "${RED}Error: Changelog merge failed${NC}"
    echo -e "${YELLOW}Undoing tag and commit...${NC}"
    git tag -d "$VERSION"
    git reset --hard HEAD~1
    exit 1
fi

echo -e "${GREEN}Changelog merged successfully!${NC}"

# Amend commit to include CHANGELOG.md changes
git add CHANGELOG.md
git commit --amend --no-edit
echo -e "${GREEN}Updated release commit with changelog${NC}"

# Delete temporary tag and recreate on amended commit (so tag points to final commit hash)
echo -e "${YELLOW}Recreating git tag ${VERSION} on final commit...${NC}"
git tag -d "$VERSION"
git tag -a "$VERSION" -m "Agentic Memorizer $VERSION"
echo -e "${GREEN}Created final tag: $VERSION${NC}"

# Display next steps
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Release preparation complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}Completed steps:${NC}"
echo "  - Updated version in internal/version/VERSION"
echo "  - Merged GoReleaser changelog into CHANGELOG.md"
echo "  - Created release commit and tag $VERSION"
echo "  - Created draft GitHub release with binaries"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Review changelog changes:"
echo "   git show HEAD:CHANGELOG.md"
echo "   git diff HEAD~1 CHANGELOG.md"
echo ""
echo "2. Review draft release on GitHub:"
echo "   https://github.com/leefowlercu/agentic-memorizer/releases"
echo ""
echo "3. If changes needed:"
echo "   - Edit CHANGELOG.md locally and/or release notes on GitHub"
echo "   - Amend commit and update tag:"
echo "     git add CHANGELOG.md && git commit --amend --no-edit"
echo "     git tag -fa $VERSION -m \"Agentic Memorizer $VERSION\""
echo "   - OR undo completely: git tag -d $VERSION && git reset --hard HEAD~1"
echo ""
echo "4. When ready to publish:"
echo "   - Push commit and tag: git push && git push --tags"
echo "   - Publish draft release on GitHub"
echo ""
echo -e "${YELLOW}Note: Review merged changelog format matches Keep a Changelog standards.${NC}"
echo ""
