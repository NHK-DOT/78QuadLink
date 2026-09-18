#!/usr/bin/env python3
"""Fetch a pinned integration; never reset an existing checkout or trust a tag alone."""
import json
import hashlib
from pathlib import Path
import subprocess
root = Path(__file__).resolve().parents[1]
lock = json.loads((root / 'integration.lock.json').read_text())
target = root / 'external/go1sim'
def git(*args):
    return subprocess.check_output(['git', '-C', str(target), *args], text=True).strip()
if not target.exists():
    target.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run(['git', 'clone', '--filter=blob:none', '--no-checkout', lock['repository'], str(target)], check=True)
    subprocess.run(['git', '-C', str(target), 'checkout', '--detach', lock['commit']], check=True)
if git('rev-parse', 'HEAD') != lock['commit']:
    raise SystemExit('Integration commit differs from integration.lock.json; preserve it and use a fresh directory.')
if git('status', '--porcelain', '--untracked-files=normal'):
    raise SystemExit('Integration has modified tracked files; refusing to deploy an unverified checkout.')
print('Verified integration commit ' + lock['commit'])

# Ensure this SDK and the pinned simulator compile the same ABI implementation.
for header in sorted((root / 'include/quadlink78').glob('*.hpp')):
    peer = target / 'gazebo_ros2/ros2_unitree_legged_msgs/include/ros2_unitree_legged_msgs' / header.name
    if hashlib.sha256(header.read_bytes()).digest() != hashlib.sha256(peer.read_bytes()).digest():
        raise SystemExit('SDK/integration header hash mismatch: ' + header.name)
print('Verified SHA-256 equality of all four SDK/integration headers')
