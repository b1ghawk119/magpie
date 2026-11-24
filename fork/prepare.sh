#!/usr/bin/env bash
set -euo pipefail
[[ $REF =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]
[[ ${VERSION#"$REF+hawk."} =~ ^[1-9][0-9]*$ ]]
# Hold the downstream patch against a fixed upstream baseline. Never resolve a
# conflict by taking upstream or skipping the patch. main owns the automation.
base=fc4406fbb924537643e45e61ab6200c5ae5f2520
git fetch --no-tags --depth=1 origin "$base"
git diff --binary "$base" HEAD -- . ':!.github/workflows' > "$RUNNER_TEMP/fork.patch"
git fetch --no-tags --depth=1 https://github.com/yetone/magpie.git "refs/tags/$REF"
upstream=$(git rev-parse 'FETCH_HEAD^{commit}')
git switch --detach "$upstream"
git apply --index --3way "$RUNNER_TEMP/fork.patch"
git restore --staged --worktree --source="$GITHUB_SHA" -- .github/workflows
git config user.name 'Magpie fork release'; git config user.email '41898282+github-actions[bot]@users.noreply.github.com'
git commit --allow-empty -m "Build $REF with downstream changes from $GITHUB_SHA"
echo "source=$(git rev-parse HEAD)" >> "$GITHUB_OUTPUT"
git push origin "HEAD:refs/heads/build/$REF/$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT"
echo "version=${VERSION#v}" >> "$GITHUB_OUTPUT"
