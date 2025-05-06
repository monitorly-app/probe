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
  sed -i.bak "s/Monitorly Probe v[0-9]\+\.[0-9]\+\.[0-9]\+/Monitorly Probe v$VERSION/g" README.md
  rm README.md.bak
  git add README.md
  git commit -m "Bump version to $VERSION in README"
fi

# Create tag and push
echo "Creating tag v$VERSION..."
git tag -a "v$VERSION" -m "Release v$VERSION"

echo
echo "Tag v$VERSION created locally."
echo "To push the tag and trigger the GitHub Actions build, run:"
echo "git push origin v$VERSION"