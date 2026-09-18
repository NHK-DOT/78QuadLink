package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSampleBoardAndVerify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board")
	if err := initializeBoard(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	packet := testPacket(typeCommand, roleGuide, 1, 7)
	binary.LittleEndian.PutUint64(data[64:], 2)
	binary.LittleEndian.PutUint64(data[72:], uint64(len(packet)))
	copy(data[80:], packet)
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "record.gz")
	if err = recordBoard(path, output, 20*time.Millisecond, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err = verifyRecord(output); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(output + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest recordManifest
	if err = json.Unmarshal(bytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Records != 1 || manifest.Invalid != 0 {
		t.Fatalf("bad manifest: %+v", manifest)
	}
	if err = recordBoard(path, output, time.Millisecond, time.Millisecond); err == nil {
		t.Fatal("overwrote archive")
	}
}

func TestArchiveRejectsModifiedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "samples.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zip := gzip.NewWriter(file)
	zip.Write([]byte("G1REC1\x00\x00"))
	_, err = writeRecordedSnapshot(zip, recordedSnapshot{0, 2, []byte{1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	if err = zip.Close(); err != nil {
		t.Fatal(err)
	}
	file.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	metadata, _ := json.Marshal(recordManifest{Records: 1, SHA256: hex.EncodeToString(hash[:])})
	if err = os.WriteFile(path+".json", metadata, 0600); err != nil {
		t.Fatal(err)
	}
	if err = verifyRecord(path); err != nil {
		t.Fatal(err)
	}
	data[len(data)/2] ^= 1
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = verifyRecord(path); err == nil {
		t.Fatal("accepted modified archive")
	}
}

var hashSink uint32
var shaSink [32]byte

func BenchmarkCRC32C328(b *testing.B) {
	data := make([]byte, 328)
	for i := range data {
		data[i] = byte(i)
	}
	b.SetBytes(328)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashSink = crc32.Checksum(data, crc32cTable)
	}
}
func BenchmarkSHA256328(b *testing.B) {
	data := make([]byte, 328)
	for i := range data {
		data[i] = byte(i)
	}
	b.SetBytes(328)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		shaSink = sha256.Sum256(data)
	}
}
