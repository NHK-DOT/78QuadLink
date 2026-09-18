package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func archiveFixture(t *testing.T, corruptRecord bool) string {
	t.Helper()
	var raw bytes.Buffer
	raw.WriteString("G1REC1\x00\x00")
	for _, revision := range []uint64{2, 8} {
		var header [16]byte
		binary.LittleEndian.PutUint32(header[4:], 3)
		binary.LittleEndian.PutUint64(header[8:], revision)
		packet := append(header[:], 1, 2, 3)
		digest := sha256.Sum256(packet)
		if corruptRecord {
			digest[0] ^= 1
		}
		raw.Write(packet)
		raw.Write(digest[:])
	}
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	if _, err := z.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sample.gz")
	if err := os.WriteFile(path, compressed.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(compressed.Bytes())
	manifest := archiveManifest{Format: "G1REC1", SHA256: hex.EncodeToString(digest[:]), Records: 2, RawBytes: uint64(raw.Len()), CompressedBytes: int64(compressed.Len())}
	encoded, _ := json.Marshal(manifest)
	if err := os.WriteFile(path+".json", encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestAnalyzeVerifiedRevisionGaps(t *testing.T) {
	r, err := analyzeArchive(archiveFixture(t, false))
	if err != nil {
		t.Fatal(err)
	}
	if r.Records != 2 || len(r.Slots) != 1 || r.Slots[0].SkippedRevisions != 2 || r.Slots[0].PayloadBytes != 6 {
		t.Fatalf("bad report: %+v", r)
	}
}
func TestRejectRecordTamperEvenWithValidArchiveHash(t *testing.T) {
	if _, err := analyzeArchive(archiveFixture(t, true)); err == nil {
		t.Fatal("accepted bad record hash")
	}
}
func TestRejectTruncatedArchive(t *testing.T) {
	path := archiveFixture(t, false)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data[:len(data)-4], 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = analyzeArchive(path); err == nil {
		t.Fatal("accepted truncation")
	}
}
