package capture

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/gopacket"
)

func TestPcapWriterCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	w, err := NewPcapWriter(dir, 1024*1024, 1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// Write a fake packet (just some bytes; pcapgo accepts any).
	data := []byte{0xde, 0xad, 0xbe, 0xef}
	ci := gopacket.CaptureInfo{Timestamp: time.Now(), CaptureLength: len(data), Length: len(data)}
	if err := w.WritePacket(data, ci); err != nil {
		t.Fatal(err)
	}

	// Verify a file was created in dir.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	pcapCount := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".pcap" {
			pcapCount++
		}
	}
	if pcapCount < 1 {
		t.Fatalf("expected at least 1 .pcap file, got %d", pcapCount)
	}
}

func TestPcapWriterRotatesOnSize(t *testing.T) {
	dir := t.TempDir()
	// 10-byte max → small enough that 5 writes of 4 bytes each force rotation
	// (the threshold check is `bytes + nextBytes > maxBytes`, so packets of 4
	// bytes against a 10-byte cap rotate after the second or third write).
	w, err := NewPcapWriter(dir, 10, 1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	data := []byte{0xde, 0xad, 0xbe, 0xef}
	ci := gopacket.CaptureInfo{Timestamp: time.Now(), CaptureLength: len(data), Length: len(data)}
	for i := 0; i < 5; i++ {
		if err := w.WritePacket(data, ci); err != nil {
			t.Fatal(err)
		}
	}

	// Should have rotated: multiple .pcap files.
	entries, _ := os.ReadDir(dir)
	pcapCount := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".pcap" {
			pcapCount++
		}
	}
	if pcapCount < 2 {
		t.Fatalf("expected at least 2 .pcap files after rotation, got %d", pcapCount)
	}
}

func TestPcapWriterRequiresValidDir(t *testing.T) {
	// On Windows we can't easily create a path that is guaranteed-uncreatable
	// as a child of a path that does not exist for the *current* user. Use a
	// directory path that contains a NUL byte — that fails on every OS
	// (including Windows) without depending on filesystem permissions.
	_, err := NewPcapWriter("invalid\x00dir", 1024, time.Hour)
	if err == nil {
		t.Fatal("expected error for non-creatable dir, got nil")
	}
}
