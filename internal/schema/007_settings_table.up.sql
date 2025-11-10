CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    updated_by BIGINT NOT NULL
);
CREATE INDEX idx_settings_key ON settings (key);