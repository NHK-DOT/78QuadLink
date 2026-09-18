# Third-party provenance

`testdata/a1.urdf` is the unmodified Unitree A1 description from
https://github.com/unitreerobotics/unitree_ros at commit
`ccfc6fd8430a17ba3dacef9a1e2faf64ff3b0aee`, path
`robots/a1_description/urdf/a1.urdf`. Its BSD-3-Clause license is
retained as `testdata/UNITREE_LICENSE`.

The external Go1/A1/Go2 simulation is a separate pinned dependency, not
relicensed by this repository. See `integration.lock.json` and that
repository's `THIRD_PARTY.md` and package licenses. Unitree names and
trademarks remain with their owners.

Shared-board/frame headers and Go tools originated as additions in
NHK-DOT/go1sim. They preserve their ABI names for source compatibility.
No Unitree motion-controller sources or mesh assets are copied into this core.

`testdata/go2.urdf` is from the same commit, original path `robots/go2_description/urdf/go2_description.urdf`, with the same Unitree license.

## v0.5.1 icon

The pixel U silhouette is adapted from https://www.unitree.com/unitree-favicon.svg
(retrieved 2026-09-19), with a lightning addition. Unitree trademarks remain
with their owner; the project code license does not grant rights to those marks.
The current assets directory contains a transparent PNG wordmark with original
pixel lettering and an ICO symbol.
