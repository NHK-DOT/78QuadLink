#!/usr/bin/env python3
"""Integration checks for an already packaged toolkit, without a ROS environment."""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time


def check(kit, archive=None):
    kit = kit.resolve()
    cli = str(kit / 'bin/simkit')
    common = ['-adapter', str(kit / 'adapters/go1-gazebo-classic.json'),
              '-backend', str(kit / 'bin/go1relay')]
    env = {'PATH': '/usr/bin:/bin'}
    subprocess.run(['sha256sum', '-c', 'SHA256SUMS'], cwd=kit, env=env, check=True)
    doctor = json.loads(subprocess.check_output([cli, 'doctor', *common], env=env))
    assert doctor['selected_profile'] == 'motor-imu'
    if archive:
        result = json.loads(subprocess.check_output(
            [cli, 'analyze', '-input', str(archive.resolve())], env=env))
        print('Verified real archive records:', result['records'])
    with tempfile.TemporaryDirectory(prefix='simkit-check-') as directory:
        root = Path(directory)
        # Exercise the public recorder/inspector commands on a valid empty board.
        code = '''import json, os, subprocess, sys
cli, adapter, backend, output = sys.argv[1:]
common = ['-adapter', adapter, '-backend', backend]
board = os.environ['GO1SIM_STATE_BOARD']
view = json.loads(subprocess.check_output([cli, 'inspect', *common, '-board', board]))
assert len(view['partitions']) == 8
subprocess.run([cli, 'record', *common, '-board', board, '-output', output, '-duration', '20ms'], check=True)
subprocess.run([cli, 'verify', *common, '-output', output], check=True)
report = json.loads(subprocess.check_output([cli, 'analyze', '-input', output]))
assert report['records'] == 0
'''
        subprocess.run([cli, 'run', *common, '-shm-root', directory, '--',
                        'python3', '-c', code, cli, common[1], common[3], str(root / 'empty.gz')],
                       env=env, check=True, stdout=subprocess.DEVNULL)
        assert not list(root.glob('simkit-*')), 'board leaked after normal exit'
        # SIGTERM must reach the owned process group before the board is removed.
        marker = root / 'marker'
        child = subprocess.Popen(
            [cli, 'run', *common, '-shm-root', directory, '--', 'sh', '-c',
             'printf "%s" "$GO1SIM_STATE_BOARD" > "$1"; sleep 60 & wait', 'sh', str(marker)],
            env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            deadline = time.monotonic() + 5
            while not marker.exists():
                if child.poll() is not None or time.monotonic() > deadline:
                    raise RuntimeError('signal test failed to start')
                time.sleep(0.02)
            child.send_signal(signal.SIGTERM)
            child.communicate(timeout=5)
            assert child.returncode != 0, 'termination unexpectedly reported success'
            assert not list(root.glob('simkit-*')), 'board leaked after SIGTERM'
        finally:
            if child.poll() is None:
                child.kill()
                child.communicate()
    print('PASS: relocatable bundle, inspect/record/verify/analyze, normal cleanup and SIGTERM cleanup')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('kit', type=Path)
    parser.add_argument('--archive', type=Path)
    args = parser.parse_args()
    check(args.kit, args.archive)
