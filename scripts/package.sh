#!/usr/bin/env bash
set -euo pipefail
quad_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
quad_out="${1:-$quad_root/artifacts/release-v0.5.1}"
[[ ! -e "$quad_out" ]] || { echo "Output exists: $quad_out" >&2; exit 1; }
mkdir -p "$quad_out"
quad_out="$(cd "$quad_out" && pwd)"
cd "$quad_root"
git archive --format=tar.gz --prefix=78QuadLink-v0.5.1/ -o "$quad_out/78QuadLink-v0.5.1-source.tar.gz" HEAD
./tools/simkit/package.sh "$quad_out/78QuadLink-v0.5.1-linux-amd64"
cp LICENSE THIRD_PARTY.md "$quad_out/78QuadLink-v0.5.1-linux-amd64/"
cp -r include "$quad_out/78QuadLink-v0.5.1-linux-amd64/"
(cd "$quad_out/78QuadLink-v0.5.1-linux-amd64" && sha256sum bin/* adapters/* README.md LICENSE THIRD_PARTY.md include/quadlink78/* > SHA256SUMS)
tar -C "$quad_out" -czf "$quad_out/78QuadLink-v0.5.1-linux-amd64.tar.gz" 78QuadLink-v0.5.1-linux-amd64
cp assets/quadlink78.ico "$quad_out/quadlink78.ico"
(cd "$quad_out" && sha256sum *.tar.gz *.ico > SHA256SUMS)
echo "$quad_out"
