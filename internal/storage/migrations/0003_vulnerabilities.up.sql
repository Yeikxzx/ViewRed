CREATE TABLE IF NOT EXISTS vulnerabilities (
    id BIGSERIAL PRIMARY KEY,
    device_ip INET NOT NULL,
    cve_id VARCHAR(50),
    cvss_score NUMERIC(4,1),
    description TEXT,
    port INTEGER,
    mitigated BOOLEAN NOT NULL DEFAULT false,
    discovered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_vulnerabilities_device_ip ON vulnerabilities(device_ip);
