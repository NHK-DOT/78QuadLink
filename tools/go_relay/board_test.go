package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestBoardCreationDoesNotResetLiveBoard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board")
	if err := initializeBoard(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 64+8*1088 || binary.LittleEndian.Uint64(data) != 0x31564452414f4253 {
		t.Fatal("incorrect board ABI")
	}
	if err := initializeBoard(path); err == nil {
		t.Fatal("reset existing board")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("removed existing board")
	}
}
