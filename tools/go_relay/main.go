package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

const (
	magic              = uint32(0x4731534d)
	version            = uint8(2)
	wireHeaderSize     = 36
	wireCRCSize        = 4
	controlSize        = wireHeaderSize + wireCRCSize
	commandPayloadSize = 12 * 24
	statePayloadSize   = 12 * 20
	commandSize        = wireHeaderSize + commandPayloadSize + wireCRCSize
	stateSize          = wireHeaderSize + statePayloadSize + wireCRCSize
	maxPacketSize      = commandSize

	typeRegister   = uint8(1)
	typeUnregister = uint8(2)
	typeCommand    = uint8(3)
	typeState      = uint8(4)

	roleGuide      = uint8(1)
	roleController = uint8(2)

	validationOK validationError = iota
	validationSize
	validationMagic
	validationVersion
	validationFlags
	validationType
	validationRole
	validationPayload
	validationCRC
)

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

type validationError uint8

type header struct {
	messageType uint8
	role        uint8
	payloadSize uint32
	sequence    uint64
	timestampNS uint64
	sessionID   uint64
}

type endpoint struct {
	address             *net.UnixAddr
	lastSeen            time.Time
	sessionID           uint64
	lastDataSequence    uint64
	lastControlSequence uint64
	active              bool
	dataSequenceSet     bool
}

type counters struct {
	commands      uint64
	states        uint64
	dropped       uint64
	invalid       uint64
	crcErrors     uint64
	unauthorized  uint64
	sequenceGaps  uint64
	oldSequence   uint64
	registrations uint64
	replacements  uint64
	unregisters   uint64
	expirations   uint64
}

type relay struct {
	connection *net.UnixConn
	lease      time.Duration
	metrics    time.Duration
	peers      [3]endpoint
	counters   counters
	lastReport time.Time
}

func main() {
	var recordPath, recordOutput, verifyPath string
	var recordDuration, recordPeriod time.Duration
	flag.StringVar(&recordPath, "record-board", "", "sample a shared board read-only into a compressed archive")
	flag.StringVar(&recordOutput, "record-output", "", "new .gz archive path (never overwrite)")
	flag.StringVar(&verifyPath, "verify-record", "", "verify an archive and its SHA-256 manifest")
	flag.DurationVar(&recordDuration, "record-duration", 30*time.Second, "maximum recording duration")
	flag.DurationVar(&recordPeriod, "record-period", 2*time.Millisecond, "board sampling interval, not lossless recording")
	var inspectPath string
	flag.StringVar(&inspectPath, "inspect-board", "", "read-only JSON snapshot of a shared board and exit")
	var boardPath string
	flag.StringVar(&boardPath, "init-board", "", "create shared state board exclusively and exit")
	var socketPath string
	var leaseDuration time.Duration
	var metricsPeriod time.Duration
	var maxProcs int
	var socketMode uint
	flag.StringVar(&socketPath, "socket", "/tmp/go1sim-relay.sock", "Unix datagram socket")
	flag.DurationVar(&leaseDuration, "lease", 3*time.Second, "endpoint lease duration")
	flag.DurationVar(&metricsPeriod, "metrics", 2*time.Second, "metrics print period")
	flag.IntVar(&maxProcs, "procs", 1, "Go scheduler threads for the single-loop relay")
	flag.UintVar(&socketMode, "socket-mode", 0o600, "Unix socket permission bits")
	flag.Parse()
	if recordPath != "" || verifyPath != "" {
		if maxProcs <= 0 {
			fmt.Fprintln(os.Stderr, "go1relay: procs must be positive")
			os.Exit(2)
		}
		runtime.GOMAXPROCS(maxProcs)
		var err error
		if verifyPath != "" {
			err = verifyRecord(verifyPath)
		} else {
			err = recordBoard(recordPath, recordOutput, recordDuration, recordPeriod)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "go1relay record:", err)
			os.Exit(1)
		}
		return
	}
	if inspectPath != "" {
		if err := inspectBoard(inspectPath); err != nil {
			fmt.Fprintln(os.Stderr, "go1relay inspect-board:", err)
			os.Exit(1)
		}
		return
	}
	if boardPath != "" {
		if err := initializeBoard(boardPath); err != nil {
			fmt.Fprintln(os.Stderr, "go1relay init-board:", err)
			os.Exit(1)
		}
		fmt.Println("shared state board ready:", boardPath)
		return
	}

	if err := validateSocketPath(socketPath); err != nil {
		fmt.Fprintln(os.Stderr, "go1relay:", err)
		os.Exit(2)
	}
	if leaseDuration <= time.Second || metricsPeriod <= 0 {
		fmt.Fprintln(os.Stderr, "go1relay: lease must exceed the 1s heartbeat and metrics must be positive")
		os.Exit(2)
	}
	if maxProcs <= 0 {
		fmt.Fprintln(os.Stderr, "go1relay: procs must be positive")
		os.Exit(2)
	}
	if socketMode > 0o660 || socketMode&0o007 != 0 {
		fmt.Fprintln(os.Stderr, "go1relay: socket mode may grant access only to owner/group")
		os.Exit(2)
	}
	runtime.GOMAXPROCS(maxProcs)
	if err := serve(socketPath, leaseDuration, metricsPeriod, os.FileMode(socketMode)); err != nil {
		fmt.Fprintln(os.Stderr, "go1relay:", err)
		os.Exit(1)
	}
}

func serve(socketPath string, leaseDuration, metricsPeriod time.Duration, socketMode os.FileMode) error {
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	address := &net.UnixAddr{Name: socketPath, Net: "unixgram"}
	connection, err := net.ListenUnixgram("unixgram", address)
	if err != nil {
		return err
	}
	defer connection.Close()
	defer os.Remove(socketPath)
	if err := os.Chmod(socketPath, socketMode); err != nil {
		return err
	}

	r := relay{
		connection: connection, lease: leaseDuration, metrics: metricsPeriod,
		lastReport: time.Now(),
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	fmt.Printf("go1relay ready protocol=v%d socket=%s mode=%#o lease=%s gomaxprocs=%d\n",
		version, socketPath, socketMode, leaseDuration, runtime.GOMAXPROCS(0))

	buffer := make([]byte, maxPacketSize)
	nextMaintenance := time.Now().Add(100 * time.Millisecond)
	if err := connection.SetReadDeadline(nextMaintenance); err != nil {
		return err
	}
	for {
		select {
		case sig := <-signals:
			r.report(time.Now(), true)
			fmt.Printf("go1relay stopping signal=%s\n", sig)
			return nil
		default:
		}

		count, source, readErr := connection.ReadFromUnix(buffer)
		now := time.Now()
		if readErr != nil {
			if timeout, ok := readErr.(net.Error); !ok || !timeout.Timeout() {
				return readErr
			}
		} else {
			r.handle(buffer[:count], source, now)
		}
		if !now.Before(nextMaintenance) {
			r.expire(now)
			r.report(now, false)
			nextMaintenance = now.Add(100 * time.Millisecond)
			if err := connection.SetReadDeadline(nextMaintenance); err != nil {
				return err
			}
		}
	}
}

func (r *relay) handle(packet []byte, source *net.UnixAddr, now time.Time) {
	h, validation := parsePacket(packet)
	if validation != validationOK {
		r.counters.invalid++
		if validation == validationCRC {
			r.counters.crcErrors++
		}
		return
	}
	switch h.messageType {
	case typeRegister:
		r.register(h, source, now)
	case typeUnregister:
		r.unregister(h, source)
	case typeCommand:
		if !r.acceptData(h, source, now) {
			return
		}
		r.route(packet, roleController)
		r.counters.commands++
	case typeState:
		if !r.acceptData(h, source, now) {
			return
		}
		r.route(packet, roleGuide)
		r.counters.states++
	}
}

func (r *relay) register(h header, source *net.UnixAddr, now time.Time) {
	peer := &r.peers[h.role]
	if peer.active && sameAddress(peer.address, source) && peer.sessionID == h.sessionID {
		if h.sequence <= peer.lastControlSequence {
			r.counters.oldSequence++
			return
		}
		peer.lastControlSequence = h.sequence
		peer.lastSeen = now
		return
	}
	if peer.active {
		r.counters.replacements++
	}
	addressCopy := *source
	*peer = endpoint{
		address: &addressCopy, lastSeen: now, sessionID: h.sessionID,
		lastControlSequence: h.sequence, active: true,
	}
	r.counters.registrations++
}

func (r *relay) unregister(h header, source *net.UnixAddr) {
	peer := &r.peers[h.role]
	if peer.active && sameAddress(peer.address, source) && peer.sessionID == h.sessionID &&
		h.sequence > peer.lastControlSequence {
		*peer = endpoint{}
		r.counters.unregisters++
		return
	}
	r.counters.unauthorized++
}

func (r *relay) acceptData(h header, source *net.UnixAddr, now time.Time) bool {
	peer := &r.peers[h.role]
	if !peer.active || !sameAddress(peer.address, source) || peer.sessionID != h.sessionID {
		r.counters.unauthorized++
		return false
	}
	if h.sequence == 0 || (peer.dataSequenceSet && h.sequence <= peer.lastDataSequence) {
		r.counters.oldSequence++
		return false
	}
	if peer.dataSequenceSet && h.sequence > peer.lastDataSequence+1 {
		r.counters.sequenceGaps += h.sequence - peer.lastDataSequence - 1
	}
	peer.lastDataSequence = h.sequence
	peer.dataSequenceSet = true
	peer.lastSeen = now
	return true
}

func (r *relay) route(packet []byte, destination uint8) {
	peer := &r.peers[destination]
	if !peer.active {
		r.counters.dropped++
		return
	}
	if _, err := r.connection.WriteToUnix(packet, peer.address); err != nil {
		r.counters.dropped++
	}
}

func (r *relay) expire(now time.Time) {
	for role := roleGuide; role <= roleController; role++ {
		peer := &r.peers[role]
		if peer.active && now.Sub(peer.lastSeen) > r.lease {
			*peer = endpoint{}
			r.counters.expirations++
		}
	}
}

func (r *relay) report(now time.Time, force bool) {
	if !force && now.Sub(r.lastReport) < r.metrics {
		return
	}
	fmt.Printf("relay metrics commands=%d states=%d dropped=%d invalid=%d crc_errors=%d unauthorized=%d sequence_gaps=%d old_sequence=%d registrations=%d replacements=%d unregisters=%d expirations=%d peers=%d\n",
		r.counters.commands, r.counters.states, r.counters.dropped, r.counters.invalid,
		r.counters.crcErrors, r.counters.unauthorized, r.counters.sequenceGaps,
		r.counters.oldSequence, r.counters.registrations, r.counters.replacements,
		r.counters.unregisters, r.counters.expirations, r.activePeers())
	r.lastReport = now
}

func (r *relay) activePeers() int {
	count := 0
	for role := roleGuide; role <= roleController; role++ {
		if r.peers[role].active {
			count++
		}
	}
	return count
}

func parsePacket(packet []byte) (header, validationError) {
	if len(packet) < controlSize {
		return header{}, validationSize
	}
	if binary.LittleEndian.Uint32(packet[0:4]) != magic {
		return header{}, validationMagic
	}
	if packet[4] != version {
		return header{}, validationVersion
	}
	if packet[7] != 0 {
		return header{}, validationFlags
	}
	h := header{
		messageType: packet[5], role: packet[6],
		payloadSize: binary.LittleEndian.Uint32(packet[8:12]),
		sequence:    binary.LittleEndian.Uint64(packet[12:20]),
		timestampNS: binary.LittleEndian.Uint64(packet[20:28]),
		sessionID:   binary.LittleEndian.Uint64(packet[28:36]),
	}
	if !validRole(h.role) {
		return header{}, validationRole
	}
	expectedPayload, expectedRole, ok := messageContract(h.messageType)
	if !ok {
		return header{}, validationType
	}
	if (expectedRole != 0 && h.role != expectedRole) || h.payloadSize != expectedPayload ||
		len(packet) != wireHeaderSize+int(expectedPayload)+wireCRCSize {
		return header{}, validationPayload
	}
	expectedCRC := binary.LittleEndian.Uint32(packet[len(packet)-wireCRCSize:])
	if crc32.Checksum(packet[:len(packet)-wireCRCSize], crc32cTable) != expectedCRC {
		return header{}, validationCRC
	}
	if h.sequence == 0 || h.sessionID == 0 {
		return header{}, validationPayload
	}
	return h, validationOK
}

func messageContract(messageType uint8) (uint32, uint8, bool) {
	switch messageType {
	case typeRegister, typeUnregister:
		return 0, 0, true
	case typeCommand:
		return commandPayloadSize, roleGuide, true
	case typeState:
		return statePayloadSize, roleController, true
	default:
		return 0, 0, false
	}
}

func validRole(role uint8) bool {
	return role == roleGuide || role == roleController
}

func sameAddress(left, right *net.UnixAddr) bool {
	return left != nil && right != nil && left.Net == right.Net && left.Name == right.Name
}

func validateSocketPath(path string) error {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) || filepath.Dir(clean) != "/tmp" || filepath.Ext(clean) != ".sock" {
		return fmt.Errorf("socket must be an absolute /tmp/*.sock path, got %q", path)
	}
	return nil
}
