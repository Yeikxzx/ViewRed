CREATE TABLE IF NOT EXISTS scan_history (
    id BIGSERIAL PRIMARY KEY,
    target TEXT NOT NULL,
    profile VARCHAR(50) NOT NULL DEFAULT 'quick',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    hosts_scanned INTEGER NOT NULL DEFAULT 0,
    vulns_found INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'COMPLETED',
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_scan_history_started_at ON scan_history(started_at DESC);
