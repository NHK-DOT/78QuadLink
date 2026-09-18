package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// Read-only, same-host Linux ABI. Matches GCC lock-free 64-bit atomic board access.
// This is a diagnostic snapshot, not a multi-partition transaction or Go RT loop.
func openReadBoard(path string) ([]byte, error) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return nil, fmt.Errorf("inspector supports little-endian amd64/arm64 only")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)
	var stat syscall.Stat_t
	if err = syscall.Fstat(fd, &stat); err != nil {
		return nil, err
	}
	if stat.Size != 64+8*1088 || stat.Mode&syscall.S_IFMT != syscall.S_IFREG {
		return nil, fmt.Errorf("invalid board size/type")
	}
	data, err := syscall.Mmap(fd, 0, int(stat.Size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint64(data) != 0x31564452414f4253 {
		syscall.Munmap(data)
		return nil, fmt.Errorf("invalid board ABI")
	}
	return data, nil
}

func inspectBoard(path string) error {
	data, err := openReadBoard(path)
	if err != nil {
		return err
	}
	defer syscall.Munmap(data)
	rows := make([]map[string]interface{}, 0, 8)
	for slot := 0; slot < 8; slot++ {
		row := map[string]interface{}{"slot": slot, "offset": 64 + slot*1088}
		packet, revision, ok := readBoardSlot(data[64+slot*1088:])
		row["revision"] = revision
		if !ok {
			row["status"] = "empty_or_busy"
			rows = append(rows, row)
			continue
		}
		row["bytes"] = len(packet)
		if slot < 2 {
			h, valid := parsePacket(packet)
			if valid != validationOK {
				row["status"] = "invalid_motor_frame"
			} else {
				row["status"] = "ok"
				row["sequence"] = h.sequence
				row["session"] = h.sessionID
				row["steady_ns"] = h.timestampNS
				row["type"] = h.messageType
				fields := []string{"q", "dq", "tau", "kp", "kd"}
				stride := 24
				if h.messageType == typeState {
					fields = []string{"q", "dq", "ddq", "tau_est"}
					stride = 20
				}
				if h.messageType == typeCommand || h.messageType == typeState {
					motors := make([]map[string]interface{}, 0, 12)
					for i := 0; i < 12; i++ {
						start := 36 + i*stride
						m := map[string]interface{}{"index": i, "mode": binary.LittleEndian.Uint32(packet[start:])}
						for j, field := range fields {
							m[field] = math.Float32frombits(binary.LittleEndian.Uint32(packet[start+4+4*j:]))
						}
						motors = append(motors, m)
					}
					row["motors"] = motors
				}
			}
		} else if validObservationPacket(packet) {
			row["status"] = "ok"
			row["type"] = binary.LittleEndian.Uint32(packet[4:])
			row["sim_ns"] = binary.LittleEndian.Uint64(packet[8:])
			row["steady_ns"] = binary.LittleEndian.Uint64(packet[16:])
			row["sequence"] = binary.LittleEndian.Uint64(packet[24:])
			row["parent"] = boardString(packet[32:96])
			row["child"] = boardString(packet[96:160])
			values := make([]float64, (len(packet)-164)/8)
			for i := range values {
				values[i] = math.Float64frombits(binary.LittleEndian.Uint64(packet[160+8*i:]))
			}
			row["values"] = values
		} else {
			row["status"] = "unknown_or_invalid_schema"
		}
		rows = append(rows, row)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]interface{}{"path": path, "abi": 1, "bytes": len(data), "partitions": rows})
}

func boardString(data []byte) string {
	for i, b := range data {
		if b == 0 {
			return string(data[:i])
		}
	}
	return string(data)
}

func readBoardSlot(data []byte) ([]byte, uint64, bool) {
	load := func(offset int) uint64 { return atomic.LoadUint64((*uint64)(unsafe.Pointer(&data[offset]))) }
	var revision uint64
	for attempt := 0; attempt < 3; attempt++ {
		revision = load(0)
		if revision == 0 {
			return nil, 0, false
		}
		if revision&1 != 0 {
			continue
		}
		size := load(8)
		if size == 0 || size > 1024 {
			return nil, revision, false
		}
		packet := make([]byte, (int(size)+7)/8*8)
		for i := 0; i < int(size); i += 8 {
			binary.LittleEndian.PutUint64(packet[i:], load(16+i))
		}
		if load(0) == revision {
			return packet[:size], revision, true
		}
	}
	return nil, revision, false
}

func validObservationPacket(packet []byte) bool {
	if len(packet) != 244 && len(packet) != 844 {
		return false
	}
	magic := binary.LittleEndian.Uint32(packet)
	kind := binary.LittleEndian.Uint32(packet[4:])
	if !((len(packet) == 244 && magic == 0x3142534f && (kind == 1 || kind == 2)) ||
		(len(packet) == 844 && magic == 0x314f444f && kind == 3)) {
		return false
	}
	end := len(packet) - 4
	return binary.LittleEndian.Uint32(packet[end:]) == crc32.Checksum(packet[:end], crc32cTable)
}
