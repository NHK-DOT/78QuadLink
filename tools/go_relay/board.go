package main

import (
	"encoding/binary"
	"os"
)

// Board ABI v1: 64-byte header, eight 1088-byte partitions, 1024-byte payload each.
// Go only initializes an unused file; C++ owns live atomic access.
func initializeBoard(path string) (err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(path)
		}
	}()
	if err = file.Truncate(64 + 8*1088); err != nil {
		return err
	}
	var header [8]byte
	binary.LittleEndian.PutUint64(header[:], 0x31564452414f4253)
	_, err = file.WriteAt(header[:], 0)
	return err
}
