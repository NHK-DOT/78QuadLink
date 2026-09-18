#!/usr/bin/env bash
# Produce a relocatable Linux toolkit; does not build/source ROS or edit the host.
set -euo pipefail
kit_source="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
kit_output="${1:-${kit_source}/../../artifacts/78quadlink-tools-v0.5}"
kit_go="${GO_BIN:-go}"
if [[ -e "${kit_output}" ]]; then
  echo "Refusing to overwrite existing output: ${kit_output}" >&2
  exit 1
fi
mkdir -p -- "$(dirname -- "${kit_output}")"
mkdir -- "${kit_output}"
kit_output="$(cd -- "${kit_output}" && pwd)"
trap 'rm -rf -- "${kit_output}"' ERR
mkdir -- "${kit_output}/bin" "${kit_output}/adapters"
(cd "${kit_source}" && CGO_ENABLED=0 "${kit_go}" build -trimpath -o "${kit_output}/bin/simkit" .)
(cd "${kit_source}/../go_relay" && CGO_ENABLED=0 "${kit_go}" build -trimpath -o "${kit_output}/bin/go1relay" .)
cp -- "${kit_source}"/adapters/*.json "${kit_output}/adapters/"
cp -- "${kit_source}/README.md" "${kit_output}/README.md"
(cd "${kit_output}" && sha256sum bin/simkit bin/go1relay adapters/*.json README.md > SHA256SUMS)
echo "Toolkit ready: ${kit_output}"
