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

# Run tests with coverage
echo "Running test suite with coverage..."
go test -v -coverprofile=coverage.out ./...
TEST_EXIT_CODE=$?

# Calculate total coverage
COVERAGE_PCT=$(go tool cover -func=coverage.out | grep total: | awk '{print $3}' | sed 's/%//')

# Update badges in README based on test results
if [[ -f "README.md" ]]; then
  echo "Updating badges in README.md..."

  # Update build badge based on test results
  if [ $TEST_EXIT_CODE -eq 0 ]; then
    BUILD_STATUS="passing-brightgreen"
    echo "Tests passed successfully!"
  else
    BUILD_STATUS="failed-brightred"
    echo "Tests failed!"
    exit 1
  fi

  # Update build badge
  sed -i.bak 's|badge/build-[^-]*-[^"]*|badge/build-'"$BUILD_STATUS"'|g' README.md

  # Update coverage badge (encode % as %25)
  sed -i.bak 's|badge/coverage-[^-]*-[^"]*|badge/coverage-'"${COVERAGE_PCT}"'%25-violet|g' README.md

  # Update version badge
  sed -i.bak 's|badge/version-v[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*-blue|badge/version-v'"$VERSION"'-blue|g' README.md

  # Clean up backup files
  rm README.md.bak

  # Stage the changes
  git add README.md
  git commit -m "Update badges and bump version to $VERSION"
fi

# Create tag and push
echo "Creating tag v$VERSION..."
git tag -a "v$VERSION" -m "Release v$VERSION"

# Build the application with the version information
echo "Building optimized application with version v$VERSION..."
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT=$(git rev-parse HEAD)

# Use optimized build flags for smaller binary size
export CGO_ENABLED=0
LDFLAGS="-s -w"  # Strip debug info and symbol table
LDFLAGS="$LDFLAGS -X 'github.com/monitorly-app/probe/internal/version.Version=$VERSION'"
LDFLAGS="$LDFLAGS -X 'github.com/monitorly-app/probe/internal/version.BuildDate=$BUILD_DATE'"
LDFLAGS="$LDFLAGS -X 'github.com/monitorly-app/probe/internal/version.Commit=$COMMIT'"

go build -v -a -installsuffix cgo -trimpath -ldflags="$LDFLAGS" -o bin/monitorly-probe ./cmd/probe

echo
echo "Tag v$VERSION created locally and binary built."
echo "To push the tag and trigger the GitHub Actions build, run:"
echo "git push origin v$VERSION"