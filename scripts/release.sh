#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' 1>&2
Usage:
  make release <version>

Examples:
  make release 1.2.3
  make release v1.2.3

Notes:
  - <version> must be X.Y.Z or vX.Y.Z (SemVer).
  - This command must be run on the main branch.
EOF
}

if [[ $# -ne 1 ]]; then
  usage
  exit 2
fi

raw_version="$1"
if [[ "${raw_version}" == v* ]]; then
  raw_version="${raw_version#v}"
fi

if [[ ! "${raw_version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: bad version format: '${1}' (expected X.Y.Z or vX.Y.Z, e.g. 1.2.3)" 1>&2
  exit 2
fi

version="${raw_version}"
tag="v${version}"

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "${branch}" != "main" ]]; then
  echo "Error: releases must be made from the main branch (current: ${branch})" 1>&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Error: working tree must be clean before releasing" 1>&2
  git status --porcelain 1>&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
  echo "Error: tag already exists: ${tag}" 1>&2
  exit 1
fi

if ! git remote get-url origin >/dev/null 2>&1; then
  echo "Error: git remote 'origin' is not configured" 1>&2
  exit 1
fi

index_file="web/templates/index.html"
openapi_file="web/static/openapi.yaml"

if [[ ! -f "${index_file}" ]]; then
  echo "Error: missing file: ${index_file}" 1>&2
  exit 1
fi
if [[ ! -f "${openapi_file}" ]]; then
  echo "Error: missing file: ${openapi_file}" 1>&2
  exit 1
fi

VERSION="${version}" perl -0777 -i -pe 's/(<a[^>]*href="https:\/\/github\.com\/HammerMeetNail\/yearofbingo\/releases"[^>]*>)(v\d+\.\d+\.\d+)(<\/a>)/$1."v".$ENV{VERSION}.$3/ge' "${index_file}"
VERSION="${version}" perl -i -pe 's/^  version: \d+\.\d+\.\d+$/  version: $ENV{VERSION}/' "${openapi_file}"

version_regex="${version//./\\.}"
tag_regex="v${version_regex}"

if ! grep -Eq "href=\"https://github\\.com/HammerMeetNail/yearofbingo/releases\"[^>]*>${tag_regex}</a>" "${index_file}"; then
  echo "Error: failed to update ${index_file} (expected footer link to show ${tag})" 1>&2
  exit 1
fi
if ! grep -Eq "^  version: ${version_regex}$" "${openapi_file}"; then
  echo "Error: failed to update ${openapi_file} (expected info.version to be ${version})" 1>&2
  exit 1
fi

if git diff --quiet -- "${index_file}" "${openapi_file}"; then
  echo "Error: no changes detected after update (already at ${tag}?)" 1>&2
  exit 1
fi

git add "${index_file}" "${openapi_file}"
git commit -m "Release ${tag}"

git tag "${tag}"
git push origin main
git push origin "${tag}"
