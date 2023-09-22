#!/usr/bin/env bash
set -e
set -o nounset

# check if got three parameter
if [[ $# -ne 1 ]]; then
    echo "script initialization error"
    echo "want one release_version parameters got: $#"
    exit 1
fi

release_version="$1"

# check release eligibility
commit_hash=$(git rev-parse --short HEAD)

# check commit is in master branch
if ! (git branch -r --contains "$commit_hash" | grep -E '(^|\s)origin/master$'); then
  echo "Releasing components from commit not belong to master is not allowed. Switch to master and retry"
  exit 1
fi

# check dirtyness
if output=$(git status --porcelain) && [ -n "$output" ]; then
  echo "Your git workspace is dirty. Stash or revert your changes and retry. 'git status -porcelain' output: $output"
  exit 1
fi

echo "Verified current git branch is eligible for release"

# update local tags
git fetch --all --tags

# check the release version in origin if already exists
tagExists=$(git tag --list "$release_version")
if [[ $tagExists ]]; then
  echo "$release_version tag is already exist in origin"
  exit 1
fi

echo "Pushing tag $release_version to origin"
git tag "$release_version"
git push origin "$release_version"