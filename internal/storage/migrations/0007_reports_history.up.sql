CREATE TABLE IF NOT EXISTS reports_history (
    id SERIAL PRIMARY KEY,
    generated_at TIMESTAMPTZ DEFAULT NOW(),
    since TIMESTAMPTZ NOT NULL,
    until TIMESTAMPTZ NOT NULL,
    filename TEXT NOT NULL,
    scenario_name TEXT,
    victim_ip TEXT,
    attacker_ip TEXT,
    expected_result TEXT,
    notes TEXT
);
