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
	"os/signal"
	"syscall"
	"time"
)

type recordedSnapshot struct {
	slot     uint32
	revision uint64
	packet   []byte
}
type recordManifest struct {
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

func writeRecordedSnapshot(w io.Writer, item recordedSnapshot) (uint64, error) {
	header := make([]byte, 16)
	binary.LittleEndian.PutUint32(header, item.slot)
	binary.LittleEndian.PutUint32(header[4:], uint32(len(item.packet)))
	binary.LittleEndian.PutUint64(header[8:], item.revision)
	digest := sha256.New()
	digest.Write(header)
	digest.Write(item.packet)
	for _, data := range [][]byte{header, item.packet, digest.Sum(nil)} {
		if _, err := w.Write(data); err != nil {
			return 0, err
		}
	}
	return uint64(48 + len(item.packet)), nil
}

// Optional side consumer: drops its own samples on backpressure, never blocks a controller.
func recordBoard(path, output string, duration, period time.Duration) error {
	if output == "" || duration <= 0 || period <= 0 {
		return fmt.Errorf("output, positive duration and period required")
	}
	data, err := openReadBoard(path)
	if err != nil {
		return err
	}
	defer syscall.Munmap(data)
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	compressedHash := sha256.New()
	zipped, err := gzip.NewWriterLevel(io.MultiWriter(file, compressedHash), gzip.BestSpeed)
	if err != nil {
		return err
	}
	buffered := bufio.NewWriterSize(zipped, 64*1024)
	if _, err = buffered.WriteString("G1REC1\x00\x00"); err != nil {
		return err
	}
	queue := make(chan recordedSnapshot, 128)
	done := make(chan error, 1)
	manifest := recordManifest{Format: "G1REC1", Period: period.String(), RawBytes: 8}
	go func() {
		for item := range queue {
			n, e := writeRecordedSnapshot(buffered, item)
			if e != nil {
				done <- e
				return
			}
			manifest.Records++
			manifest.RawBytes += n
		}
		if e := buffered.Flush(); e != nil {
			done <- e
			return
		}
		done <- zipped.Close()
	}()
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	var revisions [8]uint64
running:
	for {
		select {
		case e := <-done:
			return fmt.Errorf("record writer stopped: %v", e)
		case <-signals:
			break running
		case <-timer.C:
			break running
		case <-ticker.C:
			for slot := 0; slot < 8; slot++ {
				packet, revision, ok := readBoardSlot(data[64+slot*1088:])
				if !ok || revision == revisions[slot] {
					continue
				}
				if revisions[slot] != 0 && revision > revisions[slot]+2 {
					manifest.Overwritten += (revision-revisions[slot])/2 - 1
				}
				revisions[slot] = revision
				valid := validObservationPacket(packet)
				if slot < 2 {
					_, v := parsePacket(packet)
					valid = v == validationOK
				}
				if !valid {
					manifest.Invalid++
					continue
				}
				select {
				case queue <- recordedSnapshot{uint32(slot), revision, packet}:
				default:
					manifest.QueueDrops++
				}
			}
		}
	}
	close(queue)
	if err = <-done; err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	manifest.CompressedBytes = stat.Size()
	manifest.SHA256 = hex.EncodeToString(compressedHash.Sum(nil))
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	sidecar, err := os.OpenFile(output+".json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = sidecar.Write(encoded)
	closeErr := sidecar.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Println(string(encoded))
	return nil
}

func verifyRecord(path string) error {
	encoded, err := os.ReadFile(path + ".json")
	if err != nil {
		return err
	}
	var manifest recordManifest
	if err = json.Unmarshal(encoded, &manifest); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err = io.Copy(digest, file); err != nil {
		return err
	}
	if hex.EncodeToString(digest.Sum(nil)) != manifest.SHA256 {
		return fmt.Errorf("compressed SHA-256 mismatch")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	zipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer zipped.Close()
	reader := bufio.NewReader(zipped)
	prefix := make([]byte, 8)
	if _, err = io.ReadFull(reader, prefix); err != nil {
		return err
	}
	if string(prefix) != "G1REC1\x00\x00" {
		return fmt.Errorf("unsupported archive format")
	}
	var count uint64
	for {
		header := make([]byte, 16)
		_, err = io.ReadFull(reader, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		size := binary.LittleEndian.Uint32(header[4:])
		slot := binary.LittleEndian.Uint32(header)
		if size == 0 || size > 1024 || slot >= 8 {
			return fmt.Errorf("invalid archive record header")
		}
		payload := make([]byte, int(size)+32)
		if _, err = io.ReadFull(reader, payload); err != nil {
			return err
		}
		hash := sha256.New()
		hash.Write(header)
		hash.Write(payload[:size])
		if !bytes.Equal(hash.Sum(nil), payload[size:]) {
			return fmt.Errorf("record SHA-256 mismatch at %d", count)
		}
		count++
	}
	if count != manifest.Records {
		return fmt.Errorf("record count mismatch")
	}
	fmt.Printf("verified %d sampled snapshots: per-record and archive SHA-256 OK\n", count)
	return nil
}
