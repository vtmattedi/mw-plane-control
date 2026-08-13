CREATE TABLE service_metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO service_metadata (key, value, updated_at)
VALUES (
    'schema.bootstrap',
    'step-01',
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
);
