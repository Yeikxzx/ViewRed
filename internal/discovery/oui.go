package discovery

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

var (
	ouiMap = make(map[string]string)
	ouiMu  sync.RWMutex
)

// LoadOUI loads the OUI database from a local file, downloading it if necessary.
func LoadOUI(filePath string) error {
	ouiMu.Lock()
	defer ouiMu.Unlock()

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Download it
		resp, err := http.Get("https://standards-oui.ieee.org/oui/oui.txt")
		if err != nil {
			return fmt.Errorf("failed to download oui.txt: %w", err)
		}
		defer resp.Body.Close()

		out, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create oui.txt: %w", err)
		}
		defer out.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			out.WriteString(line + "\n")
			parseOUILine(line)
		}
		return nil
	}

	// Read existing
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parseOUILine(scanner.Text())
	}
	return nil
}

func parseOUILine(line string) {
	// Format: "  00-00-00   (hex)\t\tXEROX CORPORATION"
	// Or:     "000000     (base 16)\t\tXEROX CORPORATION"
	if strings.Contains(line, "(hex)") {
		parts := strings.SplitN(line, "(hex)", 2)
		if len(parts) == 2 {
			macPrefix := strings.TrimSpace(parts[0])
			macPrefix = strings.ReplaceAll(macPrefix, "-", ":")
			macPrefix = strings.ToLower(macPrefix)
			vendor := strings.TrimSpace(parts[1])
			ouiMap[macPrefix] = vendor
		}
	}
}

// GetVendor returns the vendor string for a given MAC address (e.g. "aa:bb:cc:dd:ee:ff")
func GetVendor(mac string) string {
	ouiMu.RLock()
	defer ouiMu.RUnlock()

	mac = strings.ToLower(mac)
	if len(mac) >= 8 {
		// e.g. "aa:bb:cc"
		prefix := mac[:8]
		if vendor, ok := ouiMap[prefix]; ok {
			return vendor
		}
	}
	return "Unknown"
}
