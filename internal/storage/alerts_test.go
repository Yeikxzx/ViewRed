package storage

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorage_QueryAlerts(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Mock COUNT
	mock.ExpectQuery(`SELECT COUNT(*) FROM alerts`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// Mock SELECT
	now := time.Now().UTC()
	rows := sqlmock.NewRows([]string{"id", "time", "severity", "source", "score", "title", "description", "context", "acknowledged_at"}).
		AddRow(1, now, "HIGH", "ML", 0.95, "Anomaly", nil, []byte(`{}`), nil).
		AddRow(2, now.Add(-1*time.Minute), "LOW", "Rules", 0.1, "Match", nil, nil, nil)

	mock.ExpectQuery(`
		SELECT id, time, severity, source, score, title, description, context, acknowledged_at
		FROM alerts
		ORDER BY time DESC
		LIMIT $1 OFFSET $2
	`).
		WithArgs(10, 0).
		WillReturnRows(rows)

	alerts, count, err := s.QueryAlerts(ctx, AlertFilter{Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	require.Len(t, alerts, 2)
	assert.Equal(t, "Anomaly", alerts[0].Title)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_AcknowledgeAlert(t *testing.T) {
	s, mock, cleanup := newMockStorage(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec(`UPDATE alerts SET acknowledged_at = NOW() WHERE id = $1 AND acknowledged_at IS NULL`).
		WithArgs(42).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.AcknowledgeAlert(ctx, 42)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
