#!/usr/bin/env bash
set -euo pipefail
cd dist
for arch in amd64 arm64; do
 for name in magpie-windows-$arch.exe magpie-darwin-$arch.zip magpie-darwin-$arch.dmg; do test -s "$name"; done
done
sha256sum magpie-* > SHA256SUMS
key="$RUNNER_TEMP/fork-release.pem"
umask 077
printf '%s\n' "$SIGNING_KEY" > "$key"
trap 'unlink "$key"' EXIT
pub=$(openssl pkey -in "$key" -pubout -outform DER | tail -c 32 | base64)
test "$pub" = "$(cat ../fork/update-public.key)"
{ printf 'magpie release %s\n' "${VERSION#v}"; cat SHA256SUMS; } > "$RUNNER_TEMP/signed-msg"
openssl pkeyutl -sign -rawin -inkey "$key" -in "$RUNNER_TEMP/signed-msg" | base64 -w0 > SHA256SUMS.sig
