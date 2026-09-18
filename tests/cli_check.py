#!/usr/bin/env python3
"""Exercise the installed one-command A1 launcher via PTY and graceful shutdown."""
import os
from pathlib import Path
import pty
import signal
import subprocess
import sys
import time

root = Path(__file__).resolve().parents[1]
log = root / 'artifacts/quadlink78-cli-check.log'
master, slave = pty.openpty()
env = dict(os.environ, ROS_DOMAIN_ID='94', GAZEBO_MASTER_URI='http://127.0.0.1:11494')
with log.open('w') as output:
    p = subprocess.Popen([str(Path.home()/'.local/bin/quadlink78'), os.environ.get('SIM78_TEST_ROBOT', 'a1')], stdin=slave,
                         stdout=output, stderr=subprocess.STDOUT, env=env, start_new_session=True)
    os.close(slave)
    try:
        deadline=time.monotonic()+90
        while time.monotonic()<deadline:
            if p.poll() is not None:raise RuntimeError('launcher exited early')
            text=log.read_text(errors='replace')
            if 'initialized ctrl frame' in text:break
            time.sleep(.2)
        else:raise RuntimeError('controller not ready')
        time.sleep(1)
        os.write(master,b'2')
        time.sleep(7)
        assert 'Switched from passive to fixed stand' in log.read_text(errors='replace')
        print('PASS: installed launcher prepared robot, reached readiness and fixed stand')
    finally:
        os.killpg(p.pid,signal.SIGINT)
        try:p.wait(timeout=15)
        except subprocess.TimeoutExpired:
            os.killpg(p.pid,signal.SIGTERM)
            p.wait(timeout=5)
        os.close(master)
print(log)
