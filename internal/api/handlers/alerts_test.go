package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viewred/viewred/internal/storage"
)

func TestAlertsList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		now := time.Now()
		mock := &storage.MockQuerier{
			AlertsFn: func(ctx context.Context, filter storage.AlertFilter) ([]storage.Alert, int, error) {
				assert.Equal(t, 50, filter.Limit)
				assert.Equal(t, 10, filter.Offset)
				return []storage.Alert{
					{ID: 1, Title: "A1", Time: now},
					{ID: 2, Title: "A2", Time: now},
				}, 100, nil
			},
		}

		r := gin.New()
		r.GET("/api/v1/alerts", AlertsList(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/alerts?limit=50&offset=10", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Alerts []storage.Alert `json:"alerts"`
			Total  int             `json:"total"`
			Limit  int             `json:"limit"`
			Offset int             `json:"offset"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, 100, resp.Total)
		assert.Len(t, resp.Alerts, 2)
		assert.Equal(t, "A1", resp.Alerts[0].Title)
	})

	t.Run("StorageError", func(t *testing.T) {
		mock := &storage.MockQuerier{
			AlertsFn: func(ctx context.Context, filter storage.AlertFilter) ([]storage.Alert, int, error) {
				return nil, 0, errors.New("db down")
			},
		}

		r := gin.New()
		r.GET("/api/v1/alerts", AlertsList(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/alerts", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "db down")
	})
}

func TestAlertAcknowledge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		mock := &storage.MockQuerier{
			AcknowledgeAlertFn: func(ctx context.Context, id int64) error {
				assert.Equal(t, int64(42), id)
				return nil
			},
		}

		r := gin.New()
		r.POST("/api/v1/alerts/:id/acknowledge", AlertAcknowledge(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/alerts/42/acknowledge", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"status":"ok"`)
	})

	t.Run("InvalidID", func(t *testing.T) {
		mock := &storage.MockQuerier{}

		r := gin.New()
		r.POST("/api/v1/alerts/:id/acknowledge", AlertAcknowledge(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/alerts/abc/acknowledge", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid alert id")
	})

	t.Run("StorageError", func(t *testing.T) {
		mock := &storage.MockQuerier{
			AcknowledgeAlertFn: func(ctx context.Context, id int64) error {
				return errors.New("db timeout")
			},
		}

		r := gin.New()
		r.POST("/api/v1/alerts/:id/acknowledge", AlertAcknowledge(mock))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/alerts/42/acknowledge", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "db timeout")
	})
}
