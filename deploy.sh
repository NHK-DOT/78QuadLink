#!/usr/bin/env bash
set -eo pipefail
quad_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
quad_tools_only=false
quad_jobs=2
quad_bin_dir="${HOME}/.local/bin"
while [[ $# -gt 0 ]]; do
 case "$1" in
  --tools-only) quad_tools_only=true; shift ;;
  --jobs) quad_jobs="${2:?missing jobs}"; shift 2 ;;
  --bin-dir) quad_bin_dir="${2:?missing bin directory}"; shift 2 ;;
  -h|--help) echo './deploy.sh [--tools-only] [--jobs 2] [--bin-dir ~/.local/bin]'; exit 0 ;;
  *) echo "Unknown argument: $1" >&2; exit 2 ;;
 esac
done
[[ "$quad_jobs" =~ ^[1-9][0-9]*$ ]] || { echo 'jobs must be positive' >&2; exit 2; }
quad_link="$quad_bin_dir/quadlink78"
if [[ -e "$quad_link" || -L "$quad_link" ]]; then
 [[ -L "$quad_link" ]] || { echo "Unrelated command exists: $quad_link" >&2; exit 1; }
 case "$(readlink "$quad_link")" in
  */scripts/quadlink78|*/tools/quadlink78/quadlink78.sh) ;;
  *) echo "Unrelated command exists: $quad_link" >&2; exit 1 ;;
 esac
fi
if [[ "$quad_tools_only" != true ]]; then
 python3 "$quad_root/scripts/fetch_integration.py"
 "$quad_root/external/go1sim/deploy.sh" --jobs "$quad_jobs" --bin-dir "$quad_root/artifacts/integration-bin"
fi
if ! command -v go >/dev/null; then
 quad_go_root="${GO1SIM_GO_ROOT:-${HOME}/.local/opt/go-1.18/usr/lib/go-1.18}"
 export PATH="$quad_go_root/bin:$PATH"
fi
command -v go >/dev/null || { echo 'Go >= 1.18 required (or run full deploy).' >&2; exit 1; }
mkdir -p "$quad_root/artifacts/bin" "$quad_bin_dir"
(cd "$quad_root/tools/simkit" && CGO_ENABLED=0 go build -trimpath -o "$quad_root/artifacts/bin/simkit" .)
(cd "$quad_root/tools/go_relay" && CGO_ENABLED=0 go build -trimpath -o "$quad_root/artifacts/bin/go1relay" .)
cmake -S "$quad_root" -B "$quad_root/build" -DBUILD_TESTING=ON -DCMAKE_INSTALL_PREFIX="$quad_root/artifacts/sdk"
cmake --build "$quad_root/build" -j "$quad_jobs"
ctest --test-dir "$quad_root/build" --output-on-failure
cmake --install "$quad_root/build"
ln -sfn "$quad_root/scripts/quadlink78" "$quad_link"
"$quad_root/artifacts/bin/simkit" doctor -adapter "$quad_root/tools/simkit/adapters/a1-gazebo-classic.json" -backend "$quad_root/artifacts/bin/go1relay" > "$quad_root/artifacts/deployment-doctor.json"
echo "78QuadLink v0.5 ready: $quad_link"
echo 'Run quadlink78 a1 --gui or quadlink78 go1 --gui after full deployment.'
