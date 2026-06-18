-- ViewRed V1 — initial schema
-- Requires: PostgreSQL 15+, TimescaleDB 2.x extension

-- TimescaleDB extension (idempotent)
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Packets: hypertable partitioned by time
CREATE TABLE packets (
    time TIMESTAMPTZ NOT NULL,
    src_ip INET NOT NULL,
    dst_ip INET NOT NULL,
    src_port INTEGER,
    dst_port INTEGER,
    protocol TEXT NOT NULL,
    length INTEGER NOT NULL,
    payload_preview BYTEA
);
SELECT create_hypertable('packets', 'time');

-- Flows: 5-tuple aggregations over time windows
CREATE TABLE flows (
    time TIMESTAMPTZ NOT NULL,
    src_ip INET NOT NULL,
    dst_ip INET NOT NULL,
    src_port INTEGER,
    dst_port INTEGER,
    protocol TEXT NOT NULL,
    packets_count BIGINT NOT NULL,
    bytes_count BIGINT NOT NULL
);
SELECT create_hypertable('flows', 'time');

-- Devices: discovered hosts
CREATE TABLE devices (
    id BIGSERIAL PRIMARY KEY,
    ip INET UNIQUE NOT NULL,
    mac MACADDR UNIQUE,
    vendor TEXT,
    hostname TEXT,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    risk_score REAL
);

-- Topology edges: observed device-to-device connections
CREATE TABLE topology_edges (
    id BIGSERIAL PRIMARY KEY,
    src_device_id BIGINT REFERENCES devices(id),
    dst_device_id BIGINT REFERENCES devices(id),
    bytes_count BIGINT NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    UNIQUE(src_device_id, dst_device_id)
);

-- Alerts: from ML engine or rules
CREATE TABLE alerts (
    id BIGSERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL,
    severity TEXT NOT NULL,
    source TEXT NOT NULL,
    score REAL NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    context JSONB,
    acknowledged_at TIMESTAMPTZ
);

-- Vulnerabilities: from scanner
CREATE TABLE vulnerabilities (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT REFERENCES devices(id),
    cve TEXT,
    cvss REAL,
    description TEXT,
    port INTEGER,
    service TEXT,
    discovered_at TIMESTAMPTZ NOT NULL,
    scan_id UUID
);

-- LLM conversations
CREATE TABLE llm_conversations (
    id UUID PRIMARY KEY,
    title TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE llm_messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id UUID REFERENCES llm_conversations(id),
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
