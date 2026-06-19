package storage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestDashboardSummary_JSONKeysAreSnakeCase pins the wire-format contract
// for DashboardSummary. The React dashboard (web/src/lib/view-state.ts)
// reads `metrics.packets_per_sec` etc.; if Marshal uses the default
// Go field names the dashboard renders empty values. This is the
// regression test for CRITICAL finding C1 in the Plan 3 review.
func TestDashboardSummary_JSONKeysAreSnakeCase(t *testing.T) {
	summary := DashboardSummary{
		PacketsPerSec:     1234.5,
		BytesPerSec:       4.2e6,
		ActiveConnections: 234,
		DeviceCount:       47,
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	// The four documented snake_case keys MUST be present; the
	// default Go (PascalCase) keys MUST NOT.
	wantKeys := []string{
		`"packets_per_sec":1234.5`,
		`"bytes_per_sec":4200000`,
		`"active_connections":234`,
		`"device_count":47`,
	}
	body := string(data)
	for _, k := range wantKeys {
		if !strings.Contains(body, k) {
			t.Errorf("expected key %q in JSON, got body=%s", k, body)
		}
	}

	for _, bad := range []string{
		`"PacketsPerSec"`,
		`"BytesPerSec"`,
		`"ActiveConnections"`,
		`"DeviceCount"`,
	} {
		if strings.Contains(body, bad) {
			t.Errorf("default Go key %q must not appear in JSON, got body=%s", bad, body)
		}
	}
}

// TestTopTalker_JSONKeysAreSnakeCase covers the same wire-format
// contract for TopTalker (used by /api/v1/dashboard/top-talkers
// and by the SSE feed consumers in the React UI).
func TestTopTalker_JSONKeysAreSnakeCase(t *testing.T) {
	talkers := []TopTalker{
		{SrcIP: "10.0.0.1", Bytes: 1024, Packets: 10},
		{SrcIP: "10.0.0.2", Bytes: 2048, Packets: 20},
	}

	data, err := json.Marshal(talkers)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	body := string(data)
	if !strings.Contains(body, `"src_ip":"10.0.0.1"`) {
		t.Errorf(`expected "src_ip":"10.0.0.1" in JSON, got %s`, body)
	}
	if !strings.Contains(body, `"bytes":1024`) {
		t.Errorf(`expected "bytes":1024 in JSON, got %s`, body)
	}
	if !strings.Contains(body, `"packets":10`) {
		t.Errorf(`expected "packets":10 in JSON, got %s`, body)
	}
	if strings.Contains(body, `"SrcIP"`) || strings.Contains(body, `"Bytes"`) || strings.Contains(body, `"Packets"`) {
		t.Errorf(`default Go keys must not appear, got %s`, body)
	}
}

// TestProtocolShare_JSONKeysAreSnakeCase covers the same contract for
// ProtocolShare (used by /api/v1/dashboard/protocols).
func TestProtocolShare_JSONKeysAreSnakeCase(t *testing.T) {
	shares := []ProtocolShare{
		{Protocol: "TCP", Percentage: 60.5},
		{Protocol: "UDP", Percentage: 30.0},
	}

	data, err := json.Marshal(shares)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	body := string(data)
	if !strings.Contains(body, `"protocol":"TCP"`) {
		t.Errorf(`expected "protocol":"TCP" in JSON, got %s`, body)
	}
	if !strings.Contains(body, `"percentage":60.5`) {
		t.Errorf(`expected "percentage":60.5 in JSON, got %s`, body)
	}
	if strings.Contains(body, `"Protocol"`) || strings.Contains(body, `"Percentage"`) {
		t.Errorf(`default Go keys must not appear, got %s`, body)
	}
}

// TestDevice_JSONKeysAreSnakeCase covers the same contract for
// Device. Note that the handler layer currently maps to its own
// gin.H shape (deviceJSON) and never calls json.Marshal(Device)
// directly today, but adding the tags here means any future
// internal consumer that marshals the value (e.g. an exports CLI
// or a follow-up handler refactor) gets the right wire format by
// default.
func TestDevice_JSONKeysAreSnakeCase(t *testing.T) {
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	risk := 0.42
	d := Device{
		ID:        1,
		IP:        "10.0.0.1",
		MAC:       "aa:bb:cc:dd:ee:ff",
		Vendor:    "Acme",
		Hostname:  "laptop-01",
		FirstSeen: now.Add(-24 * time.Hour),
		LastSeen:  now,
		RiskScore: &risk,
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	body := string(data)
	wantKeys := []string{
		`"id":1`,
		`"ip":"10.0.0.1"`,
		`"mac":"aa:bb:cc:dd:ee:ff"`,
		`"vendor":"Acme"`,
		`"hostname":"laptop-01"`,
		`"first_seen":`,
		`"last_seen":`,
		`"risk_score":0.42`,
	}
	for _, k := range wantKeys {
		if !strings.Contains(body, k) {
			t.Errorf("expected key %q in JSON, got body=%s", k, body)
		}
	}

	// omitempty on RiskScore would be wrong: a nil pointer must
	// serialize as null, not be elided (the React device table
	// renders "—" for null and "0.0" for 0.0, so the distinction
	// matters). Assert the nil case too.
	var d2 Device
	d2.IP = "10.0.0.2"
	data2, err := json.Marshal(d2)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	if !strings.Contains(string(data2), `"risk_score":null`) {
		t.Errorf(`expected "risk_score":null for nil pointer, got %s`, string(data2))
	}
}
