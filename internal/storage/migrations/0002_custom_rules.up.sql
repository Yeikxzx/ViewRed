-- Create custom_rules table
CREATE TABLE custom_rules (
    id BIGSERIAL PRIMARY KEY,
    rule_text TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Insert some default rules
INSERT INTO custom_rules (rule_text, enabled) VALUES
    ('alert tcp any any -> any 22 (msg:"SSH Traffic Detected"; severity:INFO;)', true),
    ('alert tcp any any -> any 23 (msg:"Insecure Telnet Traffic"; severity:CRITICAL;)', true);
