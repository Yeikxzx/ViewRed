// Package storage provides read access to the captured packet store.
//
// The Querier interface is the read-only surface that HTTP handlers and
// the SSE ticker depend on. Production code uses *Storage (which will
// implement Querier once Task 5+ lands); tests and degraded-DB paths
// use MockQuerier and ErrorQuerier respectively. The split keeps
// handlers free of nil checks: when the database is unavailable, main.go
// injects an ErrorQuerier so every method returns a 503-mappable error
// uniformly.
package storage

import (
	"context"
	"time"

	"github.com/viewred/viewred/internal/capture"
)

// DashboardSummary contains the KPI metrics rendered on the dashboard.
//
// PacketsPerSec and BytesPerSec are windowed rates the SSE ticker
// computes each second; ActiveConnections is the count of distinct
// (src_ip, dst_ip, src_port, dst_port) tuples seen in the window;
// DeviceCount is the count of distinct IPs.
//
// JSON tags use snake_case so the SSE ticker can json.Marshal this
// type directly and produce the wire shape the React dashboard
// expects (metrics.packets_per_sec, metrics.bytes_per_sec, ...). The
// HTTP handler at /api/v1/dashboard/summary also maps to the same
// shape via gin.H, so the two paths agree.
type DashboardSummary struct {
	PacketsPerSec     float64 `json:"packets_per_sec"`
	BytesPerSec       float64 `json:"bytes_per_sec"`
	ActiveConnections int64   `json:"active_connections"`
	DeviceCount       int64   `json:"device_count"`
}

// TopTalker represents a single IP's traffic volume. The HTTP layer maps
// this directly to the top-talkers bar chart.
//
// JSON tags use snake_case to keep the wire format consistent with
// the rest of the dashboard shape.
type TopTalker struct {
	SrcIP   string `json:"src_ip"`
	Bytes   int64  `json:"bytes"`
	Packets int64  `json:"packets"`
}

// ProtocolShare represents one protocol's percentage of the total
// traffic in the window. The sum across all shares in a single response
// is approximately 100 (rounding may yield 99.99 or 100.01).
//
// JSON tags use snake_case to keep the wire format consistent.
type ProtocolShare struct {
	Protocol   string  `json:"protocol"`
	Percentage float64 `json:"percentage"`
}

// PacketFilter contains the parameters for the packet list endpoint.
// Zero values are valid defaults: empty Since/Until mean "no bound",
// empty Protocol/SrcIP/DstIP mean "any", and Limit/Offset of 0 mean
// "use server default" (handler enforces a sane cap).
type PacketFilter struct {
	Since    time.Time
	Until    time.Time
	Protocol string
	SrcIP    string
	DstIP    string
	Limit    int
	Offset   int
}

// DeviceFilter contains the parameters for the device list endpoint.
type DeviceFilter struct {
	Limit  int
	Offset int
}

// Device is a discovered host. RiskScore is nullable because not every
// device has been scored yet (scoring lands in a later plan).
//
// JSON tags use snake_case to match the device-table wire format.
// Note: a nil RiskScore must serialise as JSON `null` (NOT be elided
// with `omitempty`); the React device table renders "—" for null
// and "0.0" for 0.0, so the distinction is part of the contract.
type Device struct {
	ID        int64     `json:"id"`
	IP        string    `json:"ip"`
	MAC       string    `json:"mac"`
	Vendor    string    `json:"vendor"`
	Hostname  string    `json:"hostname"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	RiskScore *float64  `json:"risk_score"`
}

// AlertFilter contains the parameters for the alert list endpoint.
type AlertFilter struct {
	Limit  int
	Offset int
}

// Alert represents an anomaly or rule match.
type Alert struct {
	ID             int64      `json:"id"`
	Time           time.Time  `json:"time"`
	Severity       string     `json:"severity"`
	Source         string     `json:"source"`
	Score          float32    `json:"score"`
	Title          string     `json:"title"`
	Description    *string    `json:"description"`
	Context        []byte     `json:"context"` // raw JSON string/bytes
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
}

// TopologyEdge represents a link between two devices for React Flow.
type TopologyEdge struct {
	ID          int64     `json:"id"`
	SrcDeviceID int64     `json:"src_device_id"`
	DstDeviceID int64     `json:"dst_device_id"`
	BytesCount  uint64    `json:"bytes_count"`
	LastSeen    time.Time `json:"last_seen"`
}

// Querier is the read interface for storage. Implementations must be
// safe for concurrent use by multiple HTTP handlers.
type Querier interface {
	QueryDashboardSummary(ctx context.Context, since time.Time) (DashboardSummary, error)
	QueryTopTalkers(ctx context.Context, since time.Time, limit int) ([]TopTalker, error)
	QueryProtocolDistribution(ctx context.Context, since time.Time) ([]ProtocolShare, error)
	QueryPackets(ctx context.Context, filter PacketFilter) ([]capture.PacketEvent, int, error)
	QueryDevices(ctx context.Context, filter DeviceFilter) ([]Device, int, error)
	QueryTopology(ctx context.Context) ([]Device, []TopologyEdge, error)
	QueryAlerts(ctx context.Context, filter AlertFilter) ([]Alert, int, error)
	AcknowledgeAlert(ctx context.Context, id int64) error
}

// MockQuerier is a hand-rolled test double. Each method delegates to a
// function field when set, otherwise returns the zero value with no
// error. This is the same pattern Plan 2 used for MockCapture.
//
// The function-field approach beats generating a mock per-method
// because it keeps the test code declarative and makes "do nothing"
// trivial — just construct an empty MockQuerier.
type MockQuerier struct {
	DashboardSummaryFn func(ctx context.Context, since time.Time) (DashboardSummary, error)
	TopTalkersFn       func(ctx context.Context, since time.Time, limit int) ([]TopTalker, error)
	ProtocolDistFn     func(ctx context.Context, since time.Time) ([]ProtocolShare, error)
	PacketsFn          func(ctx context.Context, filter PacketFilter) ([]capture.PacketEvent, int, error)
	DevicesFn          func(ctx context.Context, filter DeviceFilter) ([]Device, int, error)
	TopologyFn         func(ctx context.Context) ([]Device, []TopologyEdge, error)
	AlertsFn           func(ctx context.Context, filter AlertFilter) ([]Alert, int, error)
	AcknowledgeAlertFn func(ctx context.Context, id int64) error
}

// Compile-time check that MockQuerier implements Querier. If the
// interface gains a method, the build fails here, not in a downstream
// handler test.
var _ Querier = (*MockQuerier)(nil)

func (m *MockQuerier) QueryDashboardSummary(ctx context.Context, since time.Time) (DashboardSummary, error) {
	if m.DashboardSummaryFn != nil {
		return m.DashboardSummaryFn(ctx, since)
	}
	return DashboardSummary{}, nil
}

func (m *MockQuerier) QueryTopTalkers(ctx context.Context, since time.Time, limit int) ([]TopTalker, error) {
	if m.TopTalkersFn != nil {
		return m.TopTalkersFn(ctx, since, limit)
	}
	return nil, nil
}

func (m *MockQuerier) QueryProtocolDistribution(ctx context.Context, since time.Time) ([]ProtocolShare, error) {
	if m.ProtocolDistFn != nil {
		return m.ProtocolDistFn(ctx, since)
	}
	return nil, nil
}

func (m *MockQuerier) QueryPackets(ctx context.Context, filter PacketFilter) ([]capture.PacketEvent, int, error) {
	if m.PacketsFn != nil {
		return m.PacketsFn(ctx, filter)
	}
	return nil, 0, nil
}

func (m *MockQuerier) QueryDevices(ctx context.Context, filter DeviceFilter) ([]Device, int, error) {
	if m.DevicesFn != nil {
		return m.DevicesFn(ctx, filter)
	}
	return nil, 0, nil
}

func (m *MockQuerier) QueryTopology(ctx context.Context) ([]Device, []TopologyEdge, error) {
	if m.TopologyFn != nil {
		return m.TopologyFn(ctx)
	}
	return nil, nil, nil
}

func (m *MockQuerier) QueryAlerts(ctx context.Context, filter AlertFilter) ([]Alert, int, error) {
	if m.AlertsFn != nil {
		return m.AlertsFn(ctx, filter)
	}
	return nil, 0, nil
}

func (m *MockQuerier) AcknowledgeAlert(ctx context.Context, id int64) error {
	if m.AcknowledgeAlertFn != nil {
		return m.AcknowledgeAlertFn(ctx, id)
	}
	return nil
}

// ErrorQuerier returns its Err from every method. main.go injects this
// when storage initialization fails, so HTTP handlers never need a nil
// check on the querier — every error maps cleanly to a 503.
//
// A nil Err is treated as a generic "storage unavailable" error so
// handlers never accidentally serve an empty success response when the
// dependency is missing.
type ErrorQuerier struct{ Err error }

// Compile-time check that ErrorQuerier implements Querier.
var _ Querier = (*ErrorQuerier)(nil)

func (e *ErrorQuerier) err() error {
	if e.Err != nil {
		return e.Err
	}
	return errStorageUnavailable
}

// errStorageUnavailable is the fallback returned by ErrorQuerier when
// Err is nil. It is package-private because no caller should ever need
// to match on it — handlers only need a non-nil error to map to 503.
var errStorageUnavailable = &storageError{message: "storage: unavailable"}

// storageError is the concrete type behind errStorageUnavailable. It
// carries no fields beyond a message because the only thing handlers
// need is a string for the JSON error body.
type storageError struct{ message string }

func (s *storageError) Error() string { return s.message }

func (e *ErrorQuerier) QueryDashboardSummary(ctx context.Context, since time.Time) (DashboardSummary, error) {
	return DashboardSummary{}, e.err()
}

func (e *ErrorQuerier) QueryTopTalkers(ctx context.Context, since time.Time, limit int) ([]TopTalker, error) {
	return nil, e.err()
}

func (e *ErrorQuerier) QueryProtocolDistribution(ctx context.Context, since time.Time) ([]ProtocolShare, error) {
	return nil, e.err()
}

func (e *ErrorQuerier) QueryPackets(ctx context.Context, filter PacketFilter) ([]capture.PacketEvent, int, error) {
	return nil, 0, e.err()
}

func (e *ErrorQuerier) QueryDevices(ctx context.Context, filter DeviceFilter) ([]Device, int, error) {
	return nil, 0, e.err()
}

func (e *ErrorQuerier) QueryTopology(ctx context.Context) ([]Device, []TopologyEdge, error) {
	return nil, nil, e.err()
}

func (e *ErrorQuerier) QueryAlerts(ctx context.Context, filter AlertFilter) ([]Alert, int, error) {
	return nil, 0, e.err()
}

func (e *ErrorQuerier) AcknowledgeAlert(ctx context.Context, id int64) error {
	return e.err()
}
