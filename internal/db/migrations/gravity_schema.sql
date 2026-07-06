-- gravity.db: blocklists, allow/deny domains, groups and clients.
-- Modeled on Pi-hole's gravity.db, simplified for a single-Mac install.

CREATE TABLE IF NOT EXISTS "group" (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    enabled         INTEGER NOT NULL DEFAULT 1,
    name            TEXT UNIQUE NOT NULL,
    description     TEXT,
    date_added      INTEGER NOT NULL,
    date_modified   INTEGER NOT NULL
);

-- domainlist.type: 0=allow-exact, 1=deny-exact, 2=allow-regex, 3=deny-regex
CREATE TABLE IF NOT EXISTS domainlist (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    domain          TEXT NOT NULL,
    type            INTEGER NOT NULL CHECK (type IN (0,1,2,3)),
    enabled         INTEGER NOT NULL DEFAULT 1,
    comment         TEXT,
    date_added      INTEGER NOT NULL,
    date_modified   INTEGER NOT NULL,
    UNIQUE(domain, type)
);

-- adlist.type: 0=blocklist source, 1=allowlist source
CREATE TABLE IF NOT EXISTS adlist (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    address         TEXT UNIQUE NOT NULL,
    type            INTEGER NOT NULL DEFAULT 0 CHECK (type IN (0,1)),
    enabled         INTEGER NOT NULL DEFAULT 1,
    comment         TEXT,
    number          INTEGER NOT NULL DEFAULT 0,
    invalid_domains INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'pending',
    etag            TEXT,
    date_added      INTEGER NOT NULL,
    date_modified   INTEGER NOT NULL,
    date_updated    INTEGER
);

-- Compiled flat domain corpus produced by parsing enabled adlists.
CREATE TABLE IF NOT EXISTS gravity (
    domain          TEXT NOT NULL,
    adlist_id       INTEGER NOT NULL REFERENCES adlist(id) ON DELETE CASCADE,
    PRIMARY KEY (domain, adlist_id)
);
CREATE INDEX IF NOT EXISTS idx_gravity_domain ON gravity(domain);

CREATE TABLE IF NOT EXISTS client (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ip              TEXT UNIQUE NOT NULL,
    comment         TEXT,
    date_added      INTEGER NOT NULL,
    date_modified   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS adlist_by_group (
    adlist_id INTEGER NOT NULL REFERENCES adlist(id) ON DELETE CASCADE,
    group_id  INTEGER NOT NULL REFERENCES "group"(id) ON DELETE CASCADE,
    PRIMARY KEY (adlist_id, group_id)
);

CREATE TABLE IF NOT EXISTS domainlist_by_group (
    domainlist_id INTEGER NOT NULL REFERENCES domainlist(id) ON DELETE CASCADE,
    group_id      INTEGER NOT NULL REFERENCES "group"(id) ON DELETE CASCADE,
    PRIMARY KEY (domainlist_id, group_id)
);

CREATE TABLE IF NOT EXISTS client_by_group (
    client_id INTEGER NOT NULL REFERENCES client(id) ON DELETE CASCADE,
    group_id  INTEGER NOT NULL REFERENCES "group"(id) ON DELETE CASCADE,
    PRIMARY KEY (client_id, group_id)
);

CREATE TABLE IF NOT EXISTS info (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Seed data: default group (id 0 by convention), and a catch-all "This Mac" client,
-- since v1 only ever has one physical client.
INSERT OR IGNORE INTO "group" (id, enabled, name, description, date_added, date_modified)
    VALUES (0, 1, 'Default', 'Default group applied to this Mac', strftime('%s','now'), strftime('%s','now'));

INSERT OR IGNORE INTO client (id, ip, comment, date_added, date_modified)
    VALUES (0, '0.0.0.0/0', 'This Mac', strftime('%s','now'), strftime('%s','now'));

INSERT OR IGNORE INTO client_by_group (client_id, group_id) VALUES (0, 0);

INSERT OR IGNORE INTO info (key, value) VALUES ('schema_version', '1');
