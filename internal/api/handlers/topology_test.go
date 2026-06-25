package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/storage"
)

func TestTopologyGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns empty topology when no data", func(t *testing.T) {
		mock := &storage.MockQuerier{
			TopologyFn: func(_ context.Context) ([]storage.Device, []storage.TopologyEdge, error) {
				return nil, nil, nil
			},
		}
		r := gin.New()
		r.GET("/api/v1/topology", TopologyGet(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/topology", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		var resp TopologyResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(resp.Nodes) != 0 || len(resp.Edges) != 0 {
			t.Errorf("expected empty, got nodes=%d edges=%d", len(resp.Nodes), len(resp.Edges))
		}
	})

	t.Run("returns topology graph", func(t *testing.T) {
		mock := &storage.MockQuerier{
			TopologyFn: func(_ context.Context) ([]storage.Device, []storage.TopologyEdge, error) {
				return []storage.Device{
						{ID: 1, IP: "192.168.1.1"},
						{ID: 2, IP: "192.168.1.2"},
					}, []storage.TopologyEdge{
						{SrcDeviceID: 1, DstDeviceID: 2},
					}, nil
			},
		}
		r := gin.New()
		r.GET("/api/v1/topology", TopologyGet(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/topology", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		var resp TopologyResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(resp.Nodes) != 2 || len(resp.Edges) != 1 {
			t.Errorf("expected 2 nodes 1 edge, got nodes=%d edges=%d", len(resp.Nodes), len(resp.Edges))
		}
	})

	t.Run("error when store fails", func(t *testing.T) {
		mock := &storage.MockQuerier{
			TopologyFn: func(_ context.Context) ([]storage.Device, []storage.TopologyEdge, error) {
				return nil, nil, errors.New("store error")
			},
		}
		r := gin.New()
		r.GET("/api/v1/topology", TopologyGet(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/topology", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
		}
	})
}
