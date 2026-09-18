package main

import (
	"encoding/binary"
	"hash/crc32"
	"net"
	"testing"
	"time"
)

func testPacket(messageType, role uint8, sequence, session uint64) []byte {
	payloadSize, _, ok := messageContract(messageType)
	if !ok {
		panic("invalid test message type")
	}
	packet := make([]byte, wireHeaderSize+int(payloadSize)+wireCRCSize)
	binary.LittleEndian.PutUint32(packet[0:4], magic)
	packet[4] = version
	packet[5] = messageType
	packet[6] = role
	binary.LittleEndian.PutUint32(packet[8:12], payloadSize)
	binary.LittleEndian.PutUint64(packet[12:20], sequence)
	binary.LittleEndian.PutUint64(packet[20:28], 99)
	binary.LittleEndian.PutUint64(packet[28:36], session)
	checksum := crc32.Checksum(packet[:len(packet)-wireCRCSize], crc32cTable)
	binary.LittleEndian.PutUint32(packet[len(packet)-wireCRCSize:], checksum)
	return packet
}

func TestParsePacket(t *testing.T) {
	packet := testPacket(typeCommand, roleGuide, 42, 7)
	h, validation := parsePacket(packet)
	if validation != validationOK || h.messageType != typeCommand || h.role != roleGuide ||
		h.sequence != 42 || h.timestampNS != 99 || h.sessionID != 7 {
		t.Fatalf("unexpected header: %+v validation=%v", h, validation)
	}
}

func TestParsePacketRejectsCorruption(t *testing.T) {
	packet := testPacket(typeCommand, roleGuide, 42, 7)
	packet[wireHeaderSize+3] ^= 0x80
	if _, validation := parsePacket(packet); validation != validationCRC {
		t.Fatalf("corrupted frame returned validation=%v", validation)
	}
}

func TestParsePacketRejectsWrongRoleAndLength(t *testing.T) {
	if _, validation := parsePacket(testPacket(typeCommand, roleController, 1, 1)); validation != validationPayload {
		t.Fatalf("wrong command role returned validation=%v", validation)
	}
	short := testPacket(typeState, roleController, 1, 1)
	if _, validation := parsePacket(short[:len(short)-1]); validation != validationPayload {
		t.Fatalf("short state returned validation=%v", validation)
	}
}

func TestRegisterAuthorizeSequenceAndReplace(t *testing.T) {
	now := time.Now()
	first := &net.UnixAddr{Name: "/tmp/first.sock", Net: "unixgram"}
	second := &net.UnixAddr{Name: "/tmp/second.sock", Net: "unixgram"}
	r := relay{}
	r.register(header{role: roleGuide, sequence: 1, sessionID: 11}, first, now)
	if !r.acceptData(header{role: roleGuide, sequence: 1, sessionID: 11}, first, now) {
		t.Fatal("registered peer data was rejected")
	}
	if r.acceptData(header{role: roleGuide, sequence: 1, sessionID: 11}, first, now) {
		t.Fatal("duplicate sequence was accepted")
	}
	if !r.acceptData(header{role: roleGuide, sequence: 4, sessionID: 11}, first, now) {
		t.Fatal("new sequence was rejected")
	}
	if r.counters.sequenceGaps != 2 || r.counters.oldSequence != 1 {
		t.Fatalf("unexpected sequence counters: %+v", r.counters)
	}
	if r.acceptData(header{role: roleGuide, sequence: 5, sessionID: 11}, second, now) {
		t.Fatal("unregistered address was accepted")
	}
	r.register(header{role: roleGuide, sequence: 1, sessionID: 12}, second, now)
	if r.counters.replacements != 1 || !r.acceptData(
		header{role: roleGuide, sequence: 1, sessionID: 12}, second, now) {
		t.Fatalf("new session did not replace old endpoint: %+v", r.counters)
	}
}

func TestExpireLease(t *testing.T) {
	now := time.Now()
	r := relay{lease: time.Second}
	r.peers[roleGuide] = endpoint{lastSeen: now.Add(-2 * time.Second), active: true}
	r.peers[roleController] = endpoint{lastSeen: now, active: true}
	r.expire(now)
	if r.activePeers() != 1 || r.counters.expirations != 1 {
		t.Fatalf("unexpected lease state: peers=%d expirations=%d", r.activePeers(), r.counters.expirations)
	}
}

func TestValidateSocketPath(t *testing.T) {
	if validateSocketPath("/tmp/go1sim-test.sock") != nil {
		t.Fatal("valid socket path was rejected")
	}
	if validateSocketPath("/home/yz/unsafe.sock") == nil || validateSocketPath("relative.sock") == nil {
		t.Fatal("unsafe socket path was accepted")
	}
}
