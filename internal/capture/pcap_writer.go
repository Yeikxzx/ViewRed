// Package capture's pcap file writer is OS-agnostic so it can be unit-tested
// on any host. The driver (npcap_windows.go / afpacket_linux.go) feeds raw
// bytes into this writer; the writer persists them as a pcap file with
// rotation by size and/or time.
//
// We intentionally do NOT add a build tag here — the writer is pure
// filesystem I/O and has no OS-specific dependencies. The build tags belong
// on the capture drivers, not on the file sink.
package capture

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcapgo"
)

// layersLinkTypeEthernet matches pcap's DLT_EN10MB / LINKTYPE_ETHERNET.
// See https://www.tcpdump.org/linktypes.html — value 1 means standard
// Ethernet. We keep the constant here (rather than importing
// github.com/google/gopacket/layers) so the writer has a single-purpose
// surface and a future move to a different link type is one line.
const layersLinkTypeEthernet = 1

// PcapWriter writes captured packets to a sequence of pcap files. It rotates
// the underlying file when either of two thresholds is exceeded:
//
//   - Size: the current file's tracked payload bytes + the next packet's
//     length exceed maxBytes.
//   - Age:  the current file has been open longer than maxAge.
//
// Rotation produces a new file named "capture-YYYYMMDD-HHMMSS.pcap" (UTC).
// When multiple rotations occur within the same wall-clock second, a
// sequence suffix ("-N") is appended to keep filenames unique.
//
// PcapWriter is safe for concurrent use; all mutations are guarded by mu.
type PcapWriter struct {
	dir      string
	maxBytes int64
	maxAge   time.Duration

	mu        sync.Mutex
	writer    *pcapgo.Writer
	file      *os.File
	path      string
	bytes     int64 // tracked payload bytes written to the current file
	openedAt  time.Time
	lastStamp string // last timestamp used in the filename (for sequence suffix)
	seq       int    // monotonic rotation counter, scoped to the writer
}

// NewPcapWriter creates a PcapWriter rooted at dir. The directory is created
// if it does not exist. maxBytes and maxAge are the rotation thresholds; a
// non-positive value disables that check.
//
// The first pcap file is opened immediately so CurrentPath() returns a
// valid path after this call returns.
func NewPcapWriter(dir string, maxBytes int64, maxAge time.Duration) (*PcapWriter, error) {
	if dir == "" {
		return nil, errors.New("pcap_writer: empty dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("pcap_writer: create dir: %w", err)
	}

	w := &PcapWriter{
		dir:      dir,
		maxBytes: maxBytes,
		maxAge:   maxAge,
	}
	if err := w.rotate(); err != nil {
		return nil, err
	}
	return w, nil
}

// rotate closes the current file (if any) and opens a fresh one. Callers
// must hold w.mu.
func (w *PcapWriter) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
		w.writer = nil
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	name := fmt.Sprintf("capture-%s.pcap", stamp)
	// If we already used this exact timestamp in this writer, append a
	// sequence suffix so rapid rotations within the same second do not
	// overwrite each other.
	if stamp == w.lastStamp {
		w.seq++
		name = fmt.Sprintf("capture-%s-%d.pcap", stamp, w.seq)
	} else {
		w.seq = 0
		w.lastStamp = stamp
	}
	path := filepath.Join(w.dir, name)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("pcap_writer: create file: %w", err)
	}
	pw := pcapgo.NewWriter(f)
	// 65535 snaplen matches what npcap/afpacket use. LINKTYPE_ETHERNET=1.
	if err := pw.WriteFileHeader(65535, layersLinkTypeEthernet); err != nil {
		_ = f.Close()
		return fmt.Errorf("pcap_writer: write header: %w", err)
	}
	w.file = f
	w.writer = pw
	w.path = path
	w.bytes = 0
	w.openedAt = time.Now()
	return nil
}

// shouldRotate reports whether a write of nextBytes should trigger rotation.
// Callers must hold w.mu.
func (w *PcapWriter) shouldRotate(nextBytes int) bool {
	if w.maxBytes > 0 && w.bytes+int64(nextBytes) > w.maxBytes {
		return true
	}
	if w.maxAge > 0 && time.Since(w.openedAt) > w.maxAge {
		return true
	}
	return false
}

// WritePacket appends a single packet to the current pcap file, rotating
// first if either threshold would be exceeded by this write.
func (w *PcapWriter) WritePacket(data []byte, ci gopacket.CaptureInfo) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.shouldRotate(len(data)) {
		if err := w.rotate(); err != nil {
			return err
		}
	}

	if w.writer == nil {
		return errors.New("pcap_writer: not initialized")
	}
	if err := w.writer.WritePacket(ci, data); err != nil {
		return fmt.Errorf("pcap_writer: write: %w", err)
	}
	w.bytes += int64(len(data))
	return nil
}

// Close releases the underlying file. After Close, no more writes are allowed.
func (w *PcapWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		w.writer = nil
		return err
	}
	return nil
}

// CurrentPath returns the absolute path of the current pcap file.
func (w *PcapWriter) CurrentPath() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.path
}
