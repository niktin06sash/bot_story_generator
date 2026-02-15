CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    updated_by BIGINT NOT NULL
);

INSERT INTO settings (key, value, updated_by) VALUES
    ('sub.basic.price', '1', 0),
    ('limit.day.base', '10', 0),
    ('limit.day.premium', '1000', 0);
