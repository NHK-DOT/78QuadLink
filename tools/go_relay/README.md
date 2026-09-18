# Go fixed-frame relay

`go1relay` is the experimental v0.3 data plane. It forwards fixed-size Unix
datagrams between `junior_ctrl` and `UnitreeGroupController` without ROS/DDS in
the motor command/state path. Go owns endpoint registration, refresh, explicit
unregistration, lease expiration, forwarding counters, protocol validation, and
shutdown cleanup.

Build and run:

```bash
cd tools/go_relay
go build -o /tmp/go1relay .
/tmp/go1relay -socket /tmp/go1sim-relay.sock -lease 3s -socket-mode 0600
```

The relay is one event loop, so `-procs` defaults to `1`. Maintenance wakes at
100 ms rather than changing the socket deadline for every packet. The endpoint
table is a fixed three-slot array because only the Guide and Controller roles
are valid.

For the optional shared-state experiment, `go1relay -init-board /dev/shm/PATH`
creates a new board and exits. It refuses to overwrite existing files. C++
endpoints use `GO1SIM_SHARED_STATE=1` and the same `GO1SIM_STATE_BOARD` path;
no relay daemon is needed in that mode. Prefer the scoped wrapper
`tools/shared_state/run.sh` from the repository root. See
[`shared_state_experiment.md`](../../docs/shared_state_experiment.md) for its
different ownership model and current limitations.

`go1relay -inspect-board /dev/shm/PATH` maps an existing board read-only, decodes
motor and optional observation partitions, prints a JSON snapshot, and exits.
It uses atomic version checks compatible with the C++ board on Linux amd64/arm64.
It neither registers as a writer nor forwards packets. See the
[shared-state FAQ](../../docs/shared_state_faq_and_observations.md) for the layout
and access-control limits.

Optional sampled recording (current local experiment after v0.4.1):

```bash
go1relay -record-board /dev/shm/PATH -record-output /tmp/new-record.gz \
  -record-duration 30s -record-period 2ms
go1relay -verify-record /tmp/new-record.gz
```

The recorder is read-only and uses a bounded 128-record queue, buffered gzip
compression, per-record SHA-256 and a whole-file SHA-256 manifest. A full queue
drops recorder samples rather than blocking producers. Latest-value overwrites
before sampling are counted separately. This is not lossless rosbag recording;
SHA-256 is integrity verification, not writer authentication. Existing output
files are never overwritten. `-procs` defaults to one for this optional service.

Set `GO1SIM_RELAY_MODE=1` for both the simulation and `junior_ctrl`.

The v2 wire format is explicit little-endian and contains a session id, a
per-data-stream sequence number, payload length, and CRC32C. Command packets are
328 bytes (36-byte header, twelve 24-byte motor commands, four-byte CRC); state
packets are 280 bytes. The relay rejects bad CRC, wrong role/type, malformed
lengths, unregistered addresses, duplicate/old frames, and sequence gaps are
reported. A new endpoint session is allowed only through a fresh Register frame.
The C++ clients also reject datagrams whose source path is not the configured
relay and create their local sockets with mode 0600.

This path has endpoint admission and source validation, but it is still not
cryptographically authenticated or encrypted; processes under the same UID are
one trust domain. The default server socket mode is 0600. The C++ controller has
a separate 50 ms stale-command watchdog and applies brake effort after timeout;
the relay lease is endpoint cleanup, not the motor safety action.
