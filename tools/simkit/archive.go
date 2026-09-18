package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

type archiveManifest struct {
	Format          string `json:"format"`
	SHA256          string `json:"compressed_sha256"`
	Records         uint64 `json:"records"`
	RawBytes        uint64 `json:"uncompressed_bytes"`
	CompressedBytes int64  `json:"compressed_bytes"`
	QueueDrops      uint64 `json:"queue_drops"`
	Overwritten     uint64 `json:"overwritten_before_sampling"`
	Invalid         uint64 `json:"invalid_frames"`
	Period          string `json:"sample_period"`
}
type slotStats struct {
	Slot             uint32 `json:"slot"`
	Records          uint64 `json:"records"`
	PayloadBytes     uint64 `json:"payload_bytes"`
	FirstRevision    uint64 `json:"first_revision"`
	LastRevision     uint64 `json:"last_revision"`
	SkippedRevisions uint64 `json:"unobserved_commits_between_records"`
	NonIncreasing    uint64 `json:"non_increasing_revisions"`
	MinPayload       uint32 `json:"min_payload_bytes"`
	MaxPayload       uint32 `json:"max_payload_bytes"`
}
type archiveReport struct {
	Format          string          `json:"format"`
	Integrity       string          `json:"integrity"`
	Records         uint64          `json:"records"`
	RawBytes        uint64          `json:"uncompressed_bytes"`
	CompressedBytes int64           `json:"compressed_bytes"`
	Reduction       float64         `json:"size_reduction_fraction"`
	Slots           []*slotStats    `json:"slots"`
	Recorder        archiveManifest `json:"recorder_manifest"`
	Semantics       string          `json:"semantics"`
}

// Streaming, O(slot count + max packet size) memory; no payload/robot model decoder.
// Manifest counters are reported metadata, not independently reconstructed facts.
func analyzeArchive(path string) (archiveReport, error) {
	var result archiveReport
	f, err := os.Open(path + ".json")
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(io.LimitReader(f, 1<<20)).Decode(&result.Recorder)
	f.Close()
	if err != nil {
		return result, err
	}
	if result.Recorder.Format != "G1REC1" {
		return result, fmt.Errorf("unsupported manifest format")
	}
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return result, err
	}
	compressedHash := sha256.New()
	zipped, err := gzip.NewReader(io.TeeReader(file, compressedHash))
	if err != nil {
		return result, err
	}
	defer zipped.Close()
	reader := bufio.NewReader(zipped)
	var prefix [8]byte
	if _, err = io.ReadFull(reader, prefix[:]); err != nil {
		return result, err
	}
	if string(prefix[:]) != "G1REC1\x00\x00" {
		return result, fmt.Errorf("unsupported archive format")
	}
	result.RawBytes = 8
	slots := make(map[uint32]*slotStats)
	for {
		var header [16]byte
		if _, err = io.ReadFull(reader, header[:]); err == io.EOF {
			break
		} else if err != nil {
			return result, err
		}
		slot := binary.LittleEndian.Uint32(header[:])
		size := binary.LittleEndian.Uint32(header[4:])
		revision := binary.LittleEndian.Uint64(header[8:])
		if slot >= 8 || size == 0 || size > 1024 || revision == 0 || revision%2 != 0 {
			return result, fmt.Errorf("invalid committed record header at record %d", result.Records)
		}
		var packet [1024 + sha256.Size]byte
		if _, err = io.ReadFull(reader, packet[:int(size)+sha256.Size]); err != nil {
			return result, err
		}
		digest := sha256.New()
		digest.Write(header[:])
		digest.Write(packet[:size])
		if !bytes.Equal(digest.Sum(nil), packet[size:int(size)+sha256.Size]) {
			return result, fmt.Errorf("record SHA-256 mismatch at record %d", result.Records)
		}
		s, ok := slots[slot]
		if !ok {
			s = &slotStats{Slot: slot, FirstRevision: revision, MinPayload: size}
			slots[slot] = s
		} else {
			if revision <= s.LastRevision {
				s.NonIncreasing++
			} else {
				s.SkippedRevisions += (revision-s.LastRevision)/2 - 1
			}
		}
		s.Records++
		s.PayloadBytes += uint64(size)
		s.LastRevision = revision
		if size < s.MinPayload {
			s.MinPayload = size
		}
		if size > s.MaxPayload {
			s.MaxPayload = size
		}
		result.Records++
		result.RawBytes += uint64(48 + size)
	}
	if hex.EncodeToString(compressedHash.Sum(nil)) != result.Recorder.SHA256 {
		return result, fmt.Errorf("compressed SHA-256 mismatch")
	}
	if result.Records != result.Recorder.Records || result.RawBytes != result.Recorder.RawBytes || stat.Size() != result.Recorder.CompressedBytes {
		return result, fmt.Errorf("manifest record/byte count mismatch")
	}
	for _, s := range slots {
		result.Slots = append(result.Slots, s)
	}
	sort.Slice(result.Slots, func(i, j int) bool { return result.Slots[i].Slot < result.Slots[j].Slot })
	result.Format = "G1REC1"
	result.Integrity = "gzip CRC, per-record SHA-256 and compressed archive SHA-256 verified; not writer authentication"
	result.CompressedBytes = stat.Size()
	result.Reduction = 1 - float64(stat.Size())/float64(result.RawBytes)
	result.Semantics = "Sampled latest-state records, not lossless history. Revision gaps are unobserved commits, not network loss. Payload semantics and cross-slot time alignment are not validated. Manifest counters are writer-reported."
	return result, nil
}
