-- queries.db: durable DNS query log. Denormalized (domain/client as plain TEXT columns)
-- since a single Mac's query volume never justifies Pi-hole's interned-ID normalization.

-- status: 0=unknown, 1=blocked_gravity, 2=forwarded, 4=blocked_regex,
--         5=blocked_exact (manual blacklist), 6=blocked_upstream_error
-- reply_type: 0=unknown, 2=nxdomain, 3=cname, 4=ip, 7=servfail
CREATE TABLE IF NOT EXISTS query_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp       INTEGER NOT NULL,
    qtype           INTEGER NOT NULL,
    domain          TEXT NOT NULL,
    client          TEXT NOT NULL,
    status          INTEGER NOT NULL,
    reply_type      INTEGER NOT NULL DEFAULT 0,
    reply_time_ms   INTEGER NOT NULL DEFAULT 0,
    forward         TEXT,
    regex_id        INTEGER
);

CREATE INDEX IF NOT EXISTS idx_query_log_timestamp ON query_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_query_log_domain ON query_log(domain);
CREATE INDEX IF NOT EXISTS idx_query_log_status ON query_log(status);

CREATE TABLE IF NOT EXISTS info (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO info (key, value) VALUES ('schema_version', '1');
