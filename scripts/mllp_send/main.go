// mllp_send sends HL7v2 files from a directory to a gateway via MLLP TCP.
//
// Usage:
//
//	go run ./scripts/mllp_send/ -dir .local/datasets/demo -addr 127.0.0.1:2575
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	vt = 0x0B
	fs = 0x1C
	cr = 0x0D
)

func main() {
	addr := flag.String("addr", "127.0.0.1:2575", "gateway MLLP address")
	dir := flag.String("dir", ".local/datasets/demo", "directory of .hl7 files")
	delay := flag.Duration("delay", 50*time.Millisecond, "delay between messages")
	flag.Parse()

	files, err := filepath.Glob(filepath.Join(*dir, "*.hl7"))
	if err != nil || len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no .hl7 files found in %s\n", *dir)
		os.Exit(1)
	}
	sort.Strings(files)

	fmt.Printf("Sending %d messages to %s\n\n", len(files), *addr)

	ok, nack, fail := 0, 0, 0
	for i, path := range files {
		name := filepath.Base(path)
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			fmt.Printf("[%02d] %-40s  SKIP (read error: %v)\n", i+1, name, readErr)
			fail++
			continue
		}

		// Normalise line endings to \r (HL7v2 segment separator).
		payload := strings.ReplaceAll(string(raw), "\r\n", "\r")
		payload = strings.ReplaceAll(payload, "\n", "\r")

		result, sendErr := send(*addr, payload)
		switch {
		case sendErr != nil:
			fmt.Printf("[%02d] %-40s  FAIL (%v)\n", i+1, name, sendErr)
			fail++
		case strings.Contains(result, "MSA|AA"):
			fmt.Printf("[%02d] %-40s  ACK AA\n", i+1, name)
			ok++
		case strings.Contains(result, "MSA|AE"):
			fmt.Printf("[%02d] %-40s  NACK AE\n", i+1, name)
			nack++
		default:
			fmt.Printf("[%02d] %-40s  UNKNOWN response\n", i+1, name)
			fail++
		}

		time.Sleep(*delay)
	}

	fmt.Printf("\nDone: %d ACK  %d NACK  %d FAIL  (total %d)\n", ok, nack, fail, len(files))
}

func send(addr, payload string) (string, error) {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	frame := []byte{vt}
	frame = append(frame, []byte(payload)...)
	frame = append(frame, fs, cr)

	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(frame); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	return string(buf[:n]), nil
}
