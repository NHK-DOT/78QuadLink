# Validation

- `cmake -S . -B build && cmake --build build && ctest --test-dir build --output-on-failure`: real two-process snapshot coherence and competing writer rejection.
- `(cd tools/simkit && go test -race ./... && go vet ./...)` and the same for `tools/go_relay`: adapter validation, model preparation, archive integrity, backend tests.
- `python3 tools/simkit/check_bundle.py ARTIFACT_DIRECTORY`: packaged tools with a minimal environment and no ROS, record/inspect/analyze plus normal/signal cleanup.
- After full deployment, `python3 tests/cli_check.py`: installed A1 launcher through a PTY, readiness, keyboard stand, SIGINT. `SIM78_TEST_ROBOT=go1` selects Go1. The script needs an idle Gazebo host and uses ROS domain 94 / Gazebo port 11494.
- Functional motion, observation parity and performance scripts are in pinned `external/go1sim/tools/quadlink78` and `external/go1sim/tools/shared_state`.

The placeholder mesh bytes in the Go parser fixture test exercise file resolution only. Real robot physics/compatibility claims come from the actual official models in the integration, never from that fixture.
