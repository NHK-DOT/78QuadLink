package main

import (
	"encoding/binary"
	"sync/atomic"
	"testing"
	"unsafe"
)

func TestBoardReaderRejectsUncommittedAndOversize(t *testing.T) {
	words := make([]uint64, 136)
	data := unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), 1088)
	atomic.StoreUint64(&words[0], 1)
	if _, _, ok := readBoardSlot(data); ok {
		t.Fatal("accepted unfinished write")
	}
	atomic.StoreUint64(&words[1], 1025)
	atomic.StoreUint64(&words[0], 2)
	if _, _, ok := readBoardSlot(data); ok {
		t.Fatal("accepted oversize payload")
	}
	atomic.StoreUint64(&words[1], 8)
	atomic.StoreUint64(&words[2], 1234)
	packet, rev, ok := readBoardSlot(data)
	if !ok || rev != 2 || binary.LittleEndian.Uint64(packet) != 1234 {
		t.Fatal("bad committed snapshot")
	}
}
