#!/bin/bash

# Simple script to create a new release tag for the monitorly-probe project

# Ensure script is run from the project root
cd "$(dirname "$0")/.." || exit 1

if [[ -z "$1" ]]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 1.0.0"
  exit 1
fi

VERSION="$1"

# Verify version format (should be like 1.0.0)
if ! [[ $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: Version must be in format X.Y.Z (e.g., 1.0.0)"
  exit 1
fi

# Check if there are uncommitted changes
if [[ -n "$(git status --porcelain)" ]]; then
  echo "Error: There are uncommitted changes in the repository."
  echo "Please commit or stash them before creating a release."
  exit 1
fi

# Check if tag already exists
if git rev-parse "v$VERSION" >/dev/null 2>&1; then
  echo "Error: Tag v$VERSION already exists."
  exit 1
fi

# Update version in README if it exists
if [[ -f "README.md" ]]; then
  echo "Updating version in README.md..."
  sed -i.bak "s/badge\/version-v[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*-blue/badge\/version-v$VERSION-blue/g" README.md
  rm README.md.bak
  git add README.md
  git commit -m "Bump version to $VERSION in README"
fi

# Create tag and push
echo "Creating tag v$VERSION..."
git tag -a "v$VERSION" -m "Release v$VERSION"

# Build the application with the version information
echo "Building application with version v$VERSION..."
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT=$(git rev-parse HEAD)
go build -v -ldflags="-X 'github.com/monitorly-app/probe/internal/version.Version=$VERSION' -X 'github.com/monitorly-app/probe/internal/version.BuildDate=$BUILD_DATE' -X 'github.com/monitorly-app/probe/internal/version.Commit=$COMMIT'" -o bin/monitorly-probe ./cmd/probe

echo
echo "Tag v$VERSION created locally and binary built."
echo "To push the tag and trigger the GitHub Actions build, run:"
echo "git push origin v$VERSION"