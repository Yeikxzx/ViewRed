package discovery

import (
	"os"
	"testing"
)

func TestParseOUILine(t *testing.T) {
	// Reset state
	ouiMu.Lock()
	ouiMap = make(map[string]string)
	ouiMu.Unlock()

	parseOUILine("  00-00-00   (hex)\t\tXEROX CORPORATION")
	parseOUILine("11-22-33   (hex)\t\tSOME VENDOR")

	v := GetVendor("00:00:00:11:22:33")
	if v != "XEROX CORPORATION" {
		t.Fatalf("expected XEROX CORPORATION, got %s", v)
	}

	v2 := GetVendor("11:22:33:AA:BB:CC")
	if v2 != "SOME VENDOR" {
		t.Fatalf("expected SOME VENDOR, got %s", v2)
	}

	v3 := GetVendor("FF:FF:FF:00:00:00")
	if v3 != "Unknown" {
		t.Fatalf("expected Unknown, got %s", v3)
	}
}

func TestLoadOUI(t *testing.T) {
	// Create dummy oui.txt
	tmp := "test_oui.txt"
	defer os.Remove(tmp)

	content := "  AA-BB-CC   (hex)\t\tTEST VENDOR\n"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := LoadOUI(tmp); err != nil {
		t.Fatal(err)
	}

	if v := GetVendor("AA:BB:CC:11:22:33"); v != "TEST VENDOR" {
		t.Fatalf("expected TEST VENDOR, got %s", v)
	}
}
